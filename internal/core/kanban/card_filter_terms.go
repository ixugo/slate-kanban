package kanban

import (
	"cmp"
	"strconv"
	"strings"
	"time"
)

// dateLayout 筛选里日期的写法，例如 2026-11-17。
const dateLayout = "2006-01-02"

// namedTest 是一个有名字的判断，用在 is:、has: 这类"从固定几个值里选一个"的筛选词里。
type namedTest struct {
	name string
	test testFunc
}

// isTests 是 is:xxx 支持的全部取值，顺序就是错误提示里列出的顺序。
var isTests = []namedTest{
	{"done", func(c *Card, ix *boardIndex) bool { return ix.doneCols[c.ColumnID] }},
	{"open", func(c *Card, ix *boardIndex) bool { return !ix.doneCols[c.ColumnID] }},
	{"human", func(c *Card, _ *boardIndex) bool { return c.HumanOnly }},
	{"claimed", func(c *Card, ix *boardIndex) bool { return claimActive(c, ix.now) }},
	{"free", func(c *Card, ix *boardIndex) bool { return !claimActive(c, ix.now) }},
	{"waiting", func(c *Card, ix *boardIndex) bool { return ix.isWaiting(c) }},
	{"overdue", func(c *Card, ix *boardIndex) bool {
		return c.DueAt != nil && c.DueAt.Before(ix.now) && !ix.doneCols[c.ColumnID]
	}},
	{"started", func(c *Card, _ *boardIndex) bool { return c.StartedAt != nil }},
	{"finished", func(c *Card, _ *boardIndex) bool { return c.FinishedAt != nil }},
}

// hasTests 是 has:xxx 和 no:xxx 支持的全部取值，表示任务"有没有"某项内容。
var hasTests = []namedTest{
	{"due", func(c *Card, _ *boardIndex) bool { return c.DueAt != nil }},
	{"branch", func(c *Card, _ *boardIndex) bool { return c.Branch != "" }},
	{"url", func(c *Card, _ *boardIndex) bool { return c.URL != "" }},
	{"tags", func(c *Card, _ *boardIndex) bool { return len(c.TagIDs) > 0 }},
	{"desc", func(c *Card, _ *boardIndex) bool { return c.Description != "" }},
	{"deps", func(c *Card, _ *boardIndex) bool { return len(c.WaitsOn) > 0 }},
}

// priorityLevels 是 priority: 后面可以写的名字，对应任务优先级的数字。
var priorityLevels = map[string]int{"high": 3, "medium": 2, "low": 1, "none": 0}

// buildIs 解析 is:xxx。
func buildIs(text, value string) (testFunc, error) {
	return pickNamed(text, value, isTests)
}

// buildHas 解析 has:xxx，任务有这项内容。
func buildHas(text, value string) (testFunc, error) {
	return pickNamed(text, value, hasTests)
}

// buildNo 解析 no:xxx，任务没有这项内容，是 has:xxx 的反面。
func buildNo(text, value string) (testFunc, error) {
	has, err := pickNamed(text, value, hasTests)
	if err != nil {
		return nil, err
	}
	return func(c *Card, ix *boardIndex) bool { return !has(c, ix) }, nil
}

// cutOp 把值开头的比较符切出来。两个字符的比较符要先判断，否则 >= 会被当成 >。
// 没写比较符时按等于处理。
func cutOp(value string) (op, rest string) {
	for _, candidate := range []string{"<=", ">=", "<", ">", "="} {
		if strings.HasPrefix(value, candidate) {
			return candidate, value[len(candidate):]
		}
	}
	return "=", value
}

// applyOp 按比较符解释比较结果：c 是 cmp.Compare 的结果，小于零表示左边更小。
func applyOp(op string, c int) bool {
	switch op {
	case "<":
		return c < 0
	case "<=":
		return c <= 0
	case ">":
		return c > 0
	case ">=":
		return c >= 0
	default:
		return c == 0
	}
}

// buildPriority 解析 priority:xxx，xxx 可以是 high、medium、low、none 或者 0 到 3，前面可以加比较符。
func buildPriority(text, value string) (testFunc, error) {
	op, rest := cutOp(value)
	level, ok := priorityLevels[strings.ToLower(rest)]
	if !ok {
		n, err := strconv.Atoi(rest)
		if err != nil || n < 0 || n > maxCardPriority {
			return nil, badTerm(text, "的优先级不合法，应该是 high、medium、low、none 或 0 到 3，前面可以加比较符")
		}
		level = n
	}
	return func(c *Card, _ *boardIndex) bool { return applyOp(op, cmp.Compare(c.Priority, level)) }, nil
}

// dayRef 是筛选里写的一个日期：要么是"今天"，要么是具体的年月日。
// 它要等到匹配时才能换算成具体的时刻，因为"今天"和时区都取决于匹配时的环境。
type dayRef struct {
	today bool
	year  int
	month time.Month
	day   int
}

// parseDay 解析日期，写法是 today 或者 2026-11-17。
func parseDay(text, value string) (dayRef, error) {
	if strings.EqualFold(value, "today") {
		return dayRef{today: true}, nil
	}
	t, err := time.Parse(dateLayout, value)
	if err != nil {
		return dayRef{}, badTerm(text, "的日期不合法，应该写成 2026-11-17 或 today")
	}
	return dayRef{year: t.Year(), month: t.Month(), day: t.Day()}, nil
}

// start 返回这个日期在 ix 时区里的 0 点。
func (d dayRef) start(ix *boardIndex) time.Time {
	if d.today {
		y, m, day := ix.now.In(ix.loc).Date()
		return time.Date(y, m, day, 0, 0, 0, 0, ix.loc)
	}
	return time.Date(d.year, d.month, d.day, 0, 0, 0, 0, ix.loc)
}

// buildDate 构造一个按自然日比较时间字段的判断。pick 取出任务上要比较的时间，没有时间的任务不命中。
// 比较的是日期而不是时刻：due:=today 命中今天任意时刻到期的任务。
func buildDate(text, value string, pick func(*Card) *time.Time) (testFunc, error) {
	op, rest := cutOp(value)
	day, err := parseDay(text, rest)
	if err != nil {
		return nil, err
	}
	return func(c *Card, ix *boardIndex) bool {
		t := pick(c)
		if t == nil {
			return false
		}
		y, m, d := t.In(ix.loc).Date()
		return applyOp(op, time.Date(y, m, d, 0, 0, 0, 0, ix.loc).Compare(day.start(ix)))
	}, nil
}

// buildDue 解析 due:xxx，比较到期时间的日期。
func buildDue(text, value string) (testFunc, error) {
	return buildDate(text, value, func(c *Card) *time.Time { return c.DueAt })
}

// buildCreated 解析 created:xxx，比较创建时间的日期。
func buildCreated(text, value string) (testFunc, error) {
	return buildDate(text, value, func(c *Card) *time.Time { return &c.CreatedAt })
}

// buildUpdated 解析 updated:xxx，比较最后修改时间的日期。
func buildUpdated(text, value string) (testFunc, error) {
	return buildDate(text, value, func(c *Card) *time.Time { return &c.UpdatedAt })
}

// pickNamed 从一组有名字的判断里按名字选一个，不区分大小写。
// 名字不在其中时返回错误，错误里按定义的顺序列出全部可选值。
func pickNamed(text, value string, options []namedTest) (testFunc, error) {
	for _, o := range options {
		if strings.EqualFold(o.name, value) {
			return o.test, nil
		}
	}
	names := make([]string, len(options))
	for i, o := range options {
		names[i] = o.name
	}
	return nil, badTerm(text, "无法识别，可选值为 "+strings.Join(names, "、"))
}
