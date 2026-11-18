package kanban

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/ixugo/goddd/pkg/orm"
)

// TagStorer 是标签的数据持久化接口。
type TagStorer interface {
	// WithTx 返回绑定到事务的副本，事务内的所有读写都必须用它。
	WithTx(orm.Tx) TagStorer
	// Create 新增一个标签。
	Create(context.Context, *Tag) error
	// Save 把整个标签写回数据库。
	Save(context.Context, *Tag) error
	// Delete 按主键删除，幂等。
	Delete(context.Context, *Tag) error
	// ListByBoard 返回看板的全部标签，按 Sort、主键升序。
	ListByBoard(context.Context, int64) ([]*Tag, error)
	// DeleteByBoard 删除看板的全部标签，用于删除看板。
	DeleteByBoard(context.Context, int64) error
}

const (
	// maxBoardTags 一个看板的标签库最多几项。
	maxBoardTags = 60
	// seedTagCount 新看板起步时自带几个标签。
	seedTagCount = 6
)

// seedTagColors 是新看板起步标签的颜色，依次是绿、黄、橙、红、紫、蓝，都取最浅一档（标准标签色）。
var seedTagColors = [seedTagCount]string{"green-3", "yellow-3", "orange-3", "red-3", "purple-3", "blue-3"}

// seedTagTexts 是新看板起步标签的文字，和 seedTagColors 一一对应。只存中文：网页发现文字和这六个完全相等时，
// 按界面语言显示译文，所以不同语言的看板共用同一份数据。网页里的译文表（label.seed.*）要和这里保持一致。
var seedTagTexts = [seedTagCount]string{"需求", "文档", "移动端", "Bug", "服务端", "前端"}

// tagHues 是标签颜色的色相，每个色相有三档深浅（1 最深，3 最浅），共 30 种，和 Trello 的配色一一对应。
var tagHues = []string{"green", "yellow", "orange", "red", "purple", "blue", "sky", "lime", "pink", "gray"}

// validTagColor 判断颜色编号是否合法：空表示没有颜色，否则是"色相-深浅"，例如 green-2。
func validTagColor(color string) bool {
	if color == "" {
		return true
	}
	hue, shade, ok := strings.Cut(color, "-")
	return ok && slices.Contains(tagHues, hue) && (shade == "1" || shade == "2" || shade == "3")
}

// normalizeTagInputs 整理并校验标签库入参：文字可以为空，非空时按单行文本校验长度，
// 颜色必须合法，主键不能为负也不能重复，非空文字不区分大小写不重复（空文字的标签可以有多个），总数不超过上限。
func normalizeTagInputs(in []TagInput) ([]TagInput, error) {
	if len(in) > maxBoardTags {
		return nil, invalidf("一个看板最多 %d 个标签", maxBoardTags)
	}
	out := make([]TagInput, 0, len(in))
	ids := make(map[int64]bool, len(in))
	texts := make(map[string]bool, len(in))
	for _, t := range in {
		t.Text = strings.TrimSpace(t.Text)
		if t.Text != "" {
			if err := checkLine("标签", t.Text, 1, maxCardTagLen); err != nil {
				return nil, err
			}
		}
		if !validTagColor(t.Color) {
			return nil, invalidf("标签颜色不合法：%s", t.Color)
		}
		if t.ID < 0 {
			return nil, invalidf("标签编号不合法")
		}
		if t.ID != 0 && ids[t.ID] {
			return nil, invalidf("标签编号重复")
		}
		ids[t.ID] = true
		if key := strings.ToLower(t.Text); t.Text != "" {
			if texts[key] {
				return nil, invalidf("标签文字重复：%s", t.Text)
			}
			texts[key] = true
		}
		out = append(out, t)
	}
	return out, nil
}

// applyTags 在事务里把新的标签库写到看板上：主键在库里的算修改，其余算新增（不信任调用方自带的主键），
// 库里原有、入参里没有的算删除并从所有卡片上摘掉；入参顺序就是新的显示顺序。
// 每次保存后标签的 Sort 都是连续的 1 到 n，所以内容和顺序都没变时不会写库。标签库没有任何变化时返回 false。
func (u *unit) applyTags(ctx context.Context, b *Board, in []TagInput) (bool, error) {
	prev, err := u.tag.ListByBoard(ctx, b.ID)
	if err != nil {
		return false, wrapDB("读取标签库", err)
	}
	known := make(map[int64]*Tag, len(prev))
	for _, t := range prev {
		known[t.ID] = t
	}
	now := u.now()
	next := make([]*Tag, 0, len(in))
	kept := make(map[int64]bool, len(in))
	changed := false
	for i, item := range in {
		sortNo := int64(i + 1)
		tag, exists := known[item.ID]
		switch {
		case !exists:
			tag = &Tag{BoardID: b.ID, Text: item.Text, Color: item.Color, Sort: sortNo, CreatedAt: now, UpdatedAt: now}
			if err := u.tag.Create(ctx, tag); err != nil {
				return false, wrapDB("创建标签", err)
			}
			changed = true
		case tag.Text != item.Text || tag.Color != item.Color || tag.Sort != sortNo:
			tag.Text, tag.Color, tag.Sort, tag.UpdatedAt = item.Text, item.Color, sortNo, now
			if err := u.tag.Save(ctx, tag); err != nil {
				return false, wrapDB("保存标签", err)
			}
			changed = true
			kept[tag.ID] = true
		default:
			kept[tag.ID] = true
		}
		next = append(next, tag)
	}
	removed, err := u.removeTags(ctx, b.ID, prev, kept)
	if err != nil {
		return false, err
	}
	b.Tags = next
	return changed || removed, nil
}

// removeTags 删除 prev 里不在 kept 中的标签，并把它们从看板的卡片上摘掉，返回有没有删。
func (u *unit) removeTags(ctx context.Context, boardID int64, prev []*Tag, kept map[int64]bool) (bool, error) {
	gone := make(map[int64]bool)
	for _, t := range prev {
		if kept[t.ID] {
			continue
		}
		if err := u.tag.Delete(ctx, t); err != nil {
			return false, wrapDB("删除标签", err)
		}
		gone[t.ID] = true
	}
	if len(gone) == 0 {
		return false, nil
	}
	return true, u.untagCards(ctx, boardID, gone)
}

// untagCards 把被删掉的标签从看板的全部卡片上摘掉，只保存真的变了的卡片（版本加一，旧页面上的修改会被乐观锁挡住）。
// 复杂度 O(卡片数 × 每张卡片的标签数)，一个看板通常是几百张卡片。
func (u *unit) untagCards(ctx context.Context, boardID int64, gone map[int64]bool) error {
	cards, err := u.card.ListByBoard(ctx, boardID)
	if err != nil {
		return wrapDB("读取卡片", err)
	}
	now := u.now()
	for _, card := range cards {
		if !slices.ContainsFunc(card.TagIDs, func(id int64) bool { return gone[id] }) {
			continue
		}
		card.TagIDs = slices.DeleteFunc(slices.Clone(card.TagIDs), func(id int64) bool { return gone[id] })
		card.Version++
		card.UpdatedAt = now
		if err := u.card.Save(ctx, card); err != nil {
			return wrapDB("保存卡片", err)
		}
	}
	return nil
}

// seedTags 为新看板创建起步的 6 个标签：模板是 empty 时只有颜色没有文字，
// 其他模板带上中文文字（seedTagTexts），之后都是看板自己的数据，可以随意改。
func (u *unit) seedTags(ctx context.Context, boardID int64, template string) ([]*Tag, error) {
	var texts [seedTagCount]string
	if template != TemplateEmpty {
		texts = seedTagTexts
	}
	now := u.now()
	out := make([]*Tag, seedTagCount)
	for i, color := range seedTagColors {
		out[i] = &Tag{BoardID: boardID, Text: texts[i], Color: color, Sort: int64(i + 1), CreatedAt: now, UpdatedAt: now}
		if err := u.tag.Create(ctx, out[i]); err != nil {
			return nil, wrapDB("创建起步的标签", err)
		}
	}
	return out, nil
}

// resolveTagNames 把标签文字换成标签库里的主键：不区分大小写匹配非空文字，保持入参顺序并去重。
// create 为真时库里没有的文字会新建一个没有颜色的标签（库满了返回 400），为假时直接忽略。
// 返回主键和标签库有没有新增。agent 和规则只会写文字，靠它落到主键上。
func (u *unit) resolveTagNames(ctx context.Context, boardID int64, names []string, create bool) ([]int64, bool, error) {
	tags, err := u.tag.ListByBoard(ctx, boardID)
	if err != nil {
		return nil, false, wrapDB("读取标签库", err)
	}
	byText := make(map[string]int64, len(tags))
	for _, t := range tags {
		if _, dup := byText[strings.ToLower(t.Text)]; t.Text != "" && !dup {
			byText[strings.ToLower(t.Text)] = t.ID
		}
	}
	ids := make([]int64, 0, len(names))
	added := false
	for _, name := range names {
		name = strings.TrimSpace(name)
		if err := checkLine("标签", name, 1, maxCardTagLen); err != nil {
			return nil, false, err
		}
		id, ok := byText[strings.ToLower(name)]
		if !ok && !create {
			continue
		}
		if !ok {
			if len(tags) >= maxBoardTags {
				return nil, false, invalidf("标签库已经有 %d 个标签，放不下新标签 %s", maxBoardTags, name)
			}
			now := u.now()
			t := &Tag{BoardID: boardID, Text: name, Sort: int64(len(tags) + 1), CreatedAt: now, UpdatedAt: now}
			if err := u.tag.Create(ctx, t); err != nil {
				return nil, false, wrapDB("创建标签", err)
			}
			tags, id, added = append(tags, t), t.ID, true
			byText[strings.ToLower(name)] = id
		}
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return ids, added, nil
}

// resolveCardTags 把卡片入参里的标签统一成主键列表：只填文字时按文字解析（缺的会新建），
// 只填主键时确认每个都属于这个看板，两种都填返回 400。返回新的入参和标签库有没有新增，
// 新增时调用方要补一条标签库变化事件。
func (u *unit) resolveCardTags(ctx context.Context, boardID int64, in UpdateCardInput) (UpdateCardInput, bool, error) {
	switch {
	case in.Tags != nil && in.TagIDs != nil:
		return in, false, invalidf("tags 和 tag_ids 不能同时填写")
	case in.Tags != nil:
		ids, added, err := u.resolveTagNames(ctx, boardID, *in.Tags, true)
		if err != nil {
			return in, false, err
		}
		in.Tags, in.TagIDs = nil, &ids
		return in, added, nil
	case in.TagIDs != nil:
		return in, false, u.checkTagIDs(ctx, boardID, *in.TagIDs)
	}
	return in, false, nil
}

// checkTagIDs 确认每个主键都是这个看板标签库里的标签。
func (u *unit) checkTagIDs(ctx context.Context, boardID int64, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	tags, err := u.tag.ListByBoard(ctx, boardID)
	if err != nil {
		return wrapDB("读取标签库", err)
	}
	for _, id := range ids {
		if !slices.ContainsFunc(tags, func(t *Tag) bool { return t.ID == id }) {
			return invalidf("标签 %d 不存在，或者不属于这个看板", id)
		}
	}
	return nil
}

// CardTagNames 返回卡片上带文字的标签的文字，按卡片上的顺序，给只拿到卡片、没有拿到视图的调用方用（例如 MCP 的写入结果）。
func (c Core) CardTagNames(ctx context.Context, card *Card) ([]string, error) {
	tags, err := c.store.Tag().ListByBoard(ctx, card.BoardID)
	if err != nil {
		return nil, wrapDB("读取标签库", err)
	}
	return newBoardIndex(c.now(), time.Local, nil, tags, nil).tagNames(card), nil
}

// emitTagsUpdated 追加一条标签库变化事件。
func (u *unit) emitTagsUpdated(ctx context.Context, actor Actor, boardID int64) error {
	return u.emit(ctx, actor, newEvent{BoardID: boardID, Type: EventBoardLabelsUpdated, Payload: map[string]any{"fields": []string{"tags"}}})
}
