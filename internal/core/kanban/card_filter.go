package kanban

import (
	"slices"
	"strconv"
	"strings"
	"unicode"
)

const (
	// maxFilterLen 筛选字符串的最大字符数。
	maxFilterLen = 500
	// maxFilterTerms 一次筛选最多包含多少个词，防止恶意构造超长条件拖慢列表接口。
	maxFilterTerms = 30
)

// testFunc 判断一张卡片是否满足某个筛选词。ix 提供列、依赖和当前时间这些判断所需的上下文。
type testFunc func(card *Card, ix *boardIndex) bool

// filterTerm 是一个解析好的筛选词。negate 为真表示取反，也就是词前面加了减号。
type filterTerm struct {
	negate bool
	test   testFunc
}

// filter 是解析好的筛选条件：所有词都满足才算命中。没有任何词时命中全部卡片。
type filter struct {
	terms []filterTerm
}

// match 判断一张卡片是否命中筛选条件。
func (f *filter) match(card *Card, ix *boardIndex) bool {
	for _, t := range f.terms {
		if t.test(card, ix) == t.negate {
			return false
		}
	}
	return true
}

// rawTerm 是从筛选字符串里切出来的一个词，还没有解析含义。
type rawTerm struct {
	text    string
	negate  bool // 词以没加引号的减号开头
	literal bool // 词以引号开头：当作普通文字，不解析 tag: 这样的前缀
}

// termBuilders 是各种前缀对应的解析函数。词里冒号前面的部分不区分大小写地查这张表。
var termBuilders = map[string]func(text, value string) (testFunc, error){
	"tag": buildTag, "col": buildColumn, "column": buildColumn, "key": buildKey,
	"title": buildTitle, "desc": buildDesc, "priority": buildPriority, "claimed": buildClaimed,
	"is": buildIs, "has": buildHas, "no": buildNo,
	"due": buildDue, "created": buildCreated, "updated": buildUpdated,
}

// parseFilter 把筛选字符串解析成筛选条件，语法见 kanban.card.go.md 的"筛选语法"。
// 空字符串得到命中全部卡片的条件；字符串太长、词太多、前缀或值不认识、引号没有闭合都返回 ErrInvalidInput，
// 错误里写出出错的那个词，让人和 agent 都知道该改哪里。
func parseFilter(q string) (*filter, error) {
	if err := checkText("筛选条件", q, 0, maxFilterLen); err != nil {
		return nil, err
	}
	raws, err := splitTerms(q)
	if err != nil {
		return nil, err
	}
	if len(raws) > maxFilterTerms {
		return nil, invalidf("筛选条件最多包含 %d 个词", maxFilterTerms)
	}
	f := &filter{terms: make([]filterTerm, 0, len(raws))}
	for _, raw := range raws {
		test, err := buildTerm(raw)
		if err != nil {
			return nil, err
		}
		f.terms = append(f.terms, filterTerm{negate: raw.negate, test: test})
	}
	return f, nil
}

// termScanner 是切词时的状态：逐个字符读入，空白处断开，双引号包起来的空白不断开。
type termScanner struct {
	terms   []rawTerm
	cur     []rune
	inQuote bool
	started bool // 当前词已经读到了第一个字符
	negate  bool
	literal bool
}

// flush 结束当前词。没有内容的词（比如单独一个减号或者一对空引号）直接丢弃。
func (s *termScanner) flush() {
	if len(s.cur) > 0 {
		s.terms = append(s.terms, rawTerm{text: string(s.cur), negate: s.negate, literal: s.literal})
	}
	s.cur, s.started, s.negate, s.literal = s.cur[:0], false, false, false
}

// feed 读入一个字符。
func (s *termScanner) feed(r rune) {
	switch {
	case r == '"':
		if !s.started {
			s.started, s.literal = true, true
		}
		s.inQuote = !s.inQuote
	case unicode.IsSpace(r) && !s.inQuote:
		s.flush()
	case !s.started && r == '-':
		s.started, s.negate = true, true
	default:
		s.started = true
		s.cur = append(s.cur, r)
	}
}

// splitTerms 按空白把筛选字符串切成词，双引号里的内容算同一个词，引号本身不保留。
// 引号没有闭合时返回 ErrInvalidInput。
func splitTerms(q string) ([]rawTerm, error) {
	s := &termScanner{}
	for _, r := range q {
		s.feed(r)
	}
	if s.inQuote {
		return nil, badTerm(string(s.cur), "的双引号没有闭合")
	}
	s.flush()
	return s.terms, nil
}

// buildTerm 解析一个词：整个词加了引号、或者冒号前面不是一个英文单词时当作普通文字，
// 否则冒号前面必须是认识的前缀。
func buildTerm(raw rawTerm) (testFunc, error) {
	key, value, found := strings.Cut(raw.text, ":")
	if raw.literal || !found || !isWord(key) {
		return textTerm(raw.text), nil
	}
	build, ok := termBuilders[strings.ToLower(key)]
	if !ok {
		return nil, badTerm(raw.text, "的前缀 "+key+" 无法识别；想搜索包含冒号的文字，请用双引号把它括起来")
	}
	if value == "" {
		return nil, badTerm(raw.text, "的值不能为空")
	}
	return build(raw.text, value)
}

// badTerm 生成"某个筛选词有问题"的错误，开头固定写出出错的词。
func badTerm(text, why string) error {
	return invalidf("筛选词 %s %s", text, why)
}

// isWord 判断 s 是不是非空的纯英文字母，用来区分 tag:xxx 这样的前缀和 12:30、:) 这样含冒号的普通文字。
func isWord(s string) bool {
	return s != "" && strings.IndexFunc(s, func(r rune) bool { return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') }) < 0
}

// containsFold 判断 s 是否包含 lowerNeedle，不区分大小写。lowerNeedle 必须已经是小写。
func containsFold(s, lowerNeedle string) bool {
	return strings.Contains(strings.ToLower(s), lowerNeedle)
}

// textTerm 普通文字：标题、描述、编号、标签里任何一处包含它就算命中，不区分大小写。
func textTerm(needle string) testFunc {
	needle = strings.ToLower(needle)
	return func(c *Card, _ *boardIndex) bool {
		return containsFold(c.Title, needle) || containsFold(c.Description, needle) || containsFold(c.Key, needle) ||
			slices.ContainsFunc(c.Tags, func(tag string) bool { return containsFold(tag, needle) })
	}
}

// buildTag 解析 tag:xxx，卡片带有这个标签（不区分大小写）。
func buildTag(_, value string) (testFunc, error) {
	return func(c *Card, _ *boardIndex) bool {
		return slices.ContainsFunc(c.Tags, func(tag string) bool { return strings.EqualFold(tag, value) })
	}, nil
}

// buildColumn 解析 col:xxx，所在列的名称等于 xxx（不区分大小写），或者列的主键等于 xxx。
func buildColumn(_, value string) (testFunc, error) {
	id, _ := strconv.ParseInt(value, 10, 64)
	return func(c *Card, ix *boardIndex) bool {
		col, ok := ix.columns[c.ColumnID]
		return ok && (strings.EqualFold(col.Name, value) || (id > 0 && col.ID == id))
	}, nil
}

// buildKey 解析 key:xxx，卡片编号等于 xxx（不区分大小写）。
func buildKey(_, value string) (testFunc, error) {
	return func(c *Card, _ *boardIndex) bool { return strings.EqualFold(c.Key, value) }, nil
}

// buildTitle 解析 title:xxx，标题里包含 xxx。
func buildTitle(_, value string) (testFunc, error) {
	needle := strings.ToLower(value)
	return func(c *Card, _ *boardIndex) bool { return containsFold(c.Title, needle) }, nil
}

// buildDesc 解析 desc:xxx，描述里包含 xxx。
func buildDesc(_, value string) (testFunc, error) {
	needle := strings.ToLower(value)
	return func(c *Card, _ *boardIndex) bool { return containsFold(c.Description, needle) }, nil
}

// buildClaimed 解析 claimed:xxx，卡片正被名为 xxx 的 agent 领取并且租约有效。
func buildClaimed(_, value string) (testFunc, error) {
	return func(c *Card, ix *boardIndex) bool {
		return claimActive(c, ix.now) && strings.EqualFold(c.ClaimedBy, value)
	}, nil
}
