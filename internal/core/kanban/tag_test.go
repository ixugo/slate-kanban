package kanban_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// setTags 以人的身份整体替换标签库，失败就让测试终止，返回写入后的看板。
func (e *cardEnv) setTags(t *testing.T, tags []kanban.TagInput) *kanban.Board {
	t.Helper()
	b, err := e.core.UpdateBoard(context.Background(), testActor, e.board.ID, kanban.UpdateBoardInput{Tags: &tags})
	if err != nil {
		t.Fatalf("修改标签库失败: %v", err)
	}
	return b
}

// tag 按文字给任务设置标签（库里没有的文字会新建），失败就让测试终止。
func (e *cardEnv) tag(t *testing.T, c *kanban.Card, tags ...string) {
	t.Helper()
	if _, err := e.core.UpdateCard(context.Background(), testActor, c.ID, kanban.UpdateCardInput{Tags: &tags}); err != nil {
		t.Fatalf("设置标签失败: %v", err)
	}
}

// tagIDs 按主键给任务设置标签，失败就让测试终止。
func (e *cardEnv) tagIDs(t *testing.T, c *kanban.Card, ids ...int64) {
	t.Helper()
	if _, err := e.core.UpdateCard(context.Background(), testActor, c.ID, kanban.UpdateCardInput{TagIDs: &ids}); err != nil {
		t.Fatalf("设置标签失败: %v", err)
	}
}

// names 返回任务上带文字的标签的文字。
func (e *cardEnv) names(t *testing.T, c *kanban.Card) []string {
	t.Helper()
	names, err := e.core.CardTagNames(context.Background(), e.stored(t, c.ID))
	if err != nil {
		t.Fatalf("读取标签文字失败: %v", err)
	}
	return names
}

// library 读出看板当前的标签库。
func (e *cardEnv) library(t *testing.T) []*kanban.Tag {
	t.Helper()
	b, err := e.core.GetBoard(context.Background(), e.board.ID)
	if err != nil {
		t.Fatalf("读取看板失败: %v", err)
	}
	return b.Tags
}

// TestNewBoardSeedsSixTags 验证新看板自带 6 个标准色标签：dev 和 simple 模板按语言带文字，empty 模板只有颜色。
func TestNewBoardSeedsSixTags(t *testing.T) {
	ctx := context.Background()
	core, _ := newBare(t)
	colors := []string{"green-3", "yellow-3", "orange-3", "red-3", "purple-3", "blue-3"}
	cases := map[string]struct {
		template, locale string
		texts            []string
	}{
		"dev 中文":    {kanban.TemplateDev, "zh-CN", []string{"需求", "文档", "移动端", "Bug", "服务端", "前端"}},
		"simple 英文": {kanban.TemplateSimple, "en", []string{"需求", "文档", "移动端", "Bug", "服务端", "前端"}}, // 起步标签只存中文，网页按语言翻译
		"empty":     {kanban.TemplateEmpty, "zh-CN", []string{"", "", "", "", "", ""}},
	}
	i := 0
	for name, c := range cases {
		i++
		b, err := core.CreateBoard(ctx, testActor, kanban.CreateBoardInput{Name: name, Slug: "SD" + string(rune('A'+i)), Template: c.template, Locale: c.locale})
		if err != nil {
			t.Fatalf("%s 创建失败: %v", name, err)
		}
		got, err := core.GetBoard(ctx, b.ID)
		if err != nil || len(got.Tags) != 6 || len(b.Tags) != 6 {
			t.Fatalf("%s 应该带 6 个标签: %+v %v", name, got.Tags, err)
		}
		for j, tag := range got.Tags {
			if tag.Text != c.texts[j] || tag.Color != colors[j] || tag.ID == 0 {
				t.Errorf("%s 第 %d 个标签不对: %+v", name, j, tag)
			}
		}
	}
}

// TestSetTagsAssignsIDsAndKeepsThemOnReload 验证新增标签由服务端分配主键，再次保存同样的内容不变也不写事件。
func TestSetTagsAssignsIDsAndKeepsThemOnReload(t *testing.T) {
	e := newCardEnv(t)
	b := e.setTags(t, []kanban.TagInput{{Text: "bug", Color: "red-2"}, {Text: "后端"}})
	if len(b.Tags) != 2 || b.Tags[0].ID == 0 || b.Tags[0].ID == b.Tags[1].ID || b.Tags[0].Color != "red-2" {
		t.Fatalf("标签库不对: %+v", b.Tags)
	}
	got := e.library(t)
	if len(got) != 2 || got[0].ID != b.Tags[0].ID || got[1].Text != "后端" {
		t.Fatalf("读回的标签库不一致: %+v", got)
	}
	before := len(e.events(t))
	e.setTags(t, []kanban.TagInput{{ID: b.Tags[0].ID, Text: "bug", Color: "red-2"}, {ID: b.Tags[1].ID, Text: "后端"}})
	if len(e.events(t)) != before {
		t.Fatalf("没有变化不应该写事件")
	}
}

// TestEditTagKeepsCardBinding 验证改文字、改颜色都只动标签自己，任务靠主键引用，所以任务不变、版本不涨；
// 事件是 board.labels_updated。
func TestEditTagKeepsCardBinding(t *testing.T) {
	e := newCardEnv(t)
	b := e.setTags(t, []kanban.TagInput{{Text: "bug", Color: "red-2"}})
	id := b.Tags[0].ID
	c := e.card(t, 0, "有标签")
	e.tagIDs(t, c, id)
	version := e.stored(t, c.ID).Version

	e.setTags(t, []kanban.TagInput{{ID: id, Text: "缺陷", Color: "green-1"}})
	if got := e.names(t, c); !slices.Equal(got, []string{"缺陷"}) || e.stored(t, c.ID).Version != version {
		t.Fatalf("改文字和颜色后任务应该跟着显示新文字且版本不变: %v", got)
	}
	if last := e.lastEvent(t); last.Type != kanban.EventBoardLabelsUpdated || !strings.Contains(last.Payload, "tags") {
		t.Fatalf("应该写 board.labels_updated 事件并带上 tags: %+v", last)
	}

	e.setTags(t, []kanban.TagInput{{ID: id, Text: "", Color: "green-1"}})
	if got := e.names(t, c); len(got) != 0 || !slices.Equal(e.stored(t, c.ID).TagIDs, []int64{id}) {
		t.Fatalf("文字清空后任务仍然绑定这个标签，只是没有文字: %v", got)
	}
}

// TestBlankTagsBindByID 验证没有文字的标签可以有多个，任务靠主键绑定它们，tags 文字视图里不出现它们。
func TestBlankTagsBindByID(t *testing.T) {
	e := newCardEnv(t)
	b := e.setTags(t, []kanban.TagInput{{Color: "green-3"}, {Color: "red-3"}, {Text: "bug", Color: "red-2"}})
	c := e.card(t, 0, "卡")
	e.tagIDs(t, c, b.Tags[1].ID, b.Tags[0].ID, b.Tags[2].ID, b.Tags[1].ID)
	if got := e.stored(t, c.ID).TagIDs; !slices.Equal(got, []int64{b.Tags[1].ID, b.Tags[0].ID, b.Tags[2].ID}) {
		t.Fatalf("应该按给定顺序去重保存: %v", got)
	}
	if got := e.names(t, c); !slices.Equal(got, []string{"bug"}) {
		t.Fatalf("没有文字的标签不应该出现在文字视图里: %v", got)
	}
}

// TestDeleteTagRemovesFromCards 验证删除标签会从任务上摘掉并让版本加一，没用到它的任务不动。
func TestDeleteTagRemovesFromCards(t *testing.T) {
	e := newCardEnv(t)
	b := e.setTags(t, []kanban.TagInput{{Text: "bug"}, {Text: "后端"}})
	c, other := e.card(t, 0, "卡"), e.card(t, 0, "无关")
	e.tagIDs(t, c, b.Tags[0].ID, b.Tags[1].ID)
	e.tagIDs(t, other, b.Tags[1].ID)
	version, otherVersion := e.stored(t, c.ID).Version, e.stored(t, other.ID).Version

	e.setTags(t, []kanban.TagInput{{ID: b.Tags[1].ID, Text: "后端"}})
	if got := e.stored(t, c.ID); !slices.Equal(got.TagIDs, []int64{b.Tags[1].ID}) || got.Version != version+1 {
		t.Fatalf("删除后应该只剩后端且版本加一: %+v", got)
	}
	if got := e.stored(t, other.ID); got.Version != otherVersion {
		t.Fatalf("没用到被删标签的任务不应该被改: %+v", got)
	}
}

// TestSetTagsReorders 验证入参顺序就是标签库的显示顺序。
func TestSetTagsReorders(t *testing.T) {
	e := newCardEnv(t)
	b := e.setTags(t, []kanban.TagInput{{Text: "a"}, {Text: "b"}, {Text: "c"}})
	e.setTags(t, []kanban.TagInput{{ID: b.Tags[2].ID, Text: "c"}, {ID: b.Tags[0].ID, Text: "a"}, {ID: b.Tags[1].ID, Text: "b"}})
	var got []string
	for _, tag := range e.library(t) {
		got = append(got, tag.Text)
	}
	if !slices.Equal(got, []string{"c", "a", "b"}) {
		t.Fatalf("顺序应该是 c a b: %v", got)
	}
}

// TestTagNamesCreateMissingTags 验证按文字设置标签时，库里没有的文字会新建成没有颜色的标签并写标签库事件，
// 已有的文字不区分大小写复用原来的标签。
func TestTagNamesCreateMissingTags(t *testing.T) {
	e := newCardEnv(t)
	seeded := len(e.library(t))
	c := e.card(t, 0, "卡")
	e.tag(t, c, "bug", "全新标签", "BUG")
	lib := e.library(t)
	if len(lib) != seeded+1 || lib[seeded].Text != "全新标签" || lib[seeded].Color != "" {
		t.Fatalf("应该只新增一个没有颜色的标签: %+v", lib)
	}
	if got := e.names(t, c); !slices.Equal(got, []string{"Bug", "全新标签"}) {
		t.Fatalf("Bug 应该复用起步标签，BUG 是它的重复: %v", got)
	}
	found := slices.ContainsFunc(e.events(t), func(ev *kanban.Event) bool { return ev.Type == kanban.EventBoardLabelsUpdated })
	if !found {
		t.Fatalf("新增标签应该写 board.labels_updated 事件")
	}
}

// TestTagIDsMustBelongToBoard 验证主键不属于这个看板、tags 和 tag_ids 同时填、主键非法都会被拒绝。
func TestTagIDsMustBelongToBoard(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	c := e.card(t, 0, "卡")
	foreign, none := []int64{987654}, []int64{-1}
	names := []string{"x"}
	ids := []int64{e.library(t)[0].ID}
	cases := map[string]kanban.UpdateCardInput{
		"别的看板的标签": {TagIDs: &foreign},
		"非法主键":    {TagIDs: &none},
		"两种都填":    {Tags: &names, TagIDs: &ids},
	}
	for name, in := range cases {
		if _, err := e.core.UpdateCard(ctx, testActor, c.ID, in); !errors.Is(err, kanban.ErrInvalidInput) {
			t.Errorf("%s 应该被拒绝，实际 %v", name, err)
		}
	}
}

// TestSetTagsRejects 验证非法输入：超长、重复文字（忽略大小写）、非法颜色、负数或重复主键、超过上限、换行；
// 空文字和重复的空文字是允许的。
func TestSetTagsRejects(t *testing.T) {
	e := newCardEnv(t)
	tooMany := make([]kanban.TagInput, 61)
	cases := map[string][]kanban.TagInput{
		"超长":    {{Text: strings.Repeat("长", 31)}},
		"重复文字":  {{Text: "Bug"}, {Text: "bug"}},
		"非法颜色":  {{Text: "a", Color: "teal-2"}},
		"颜色深浅":  {{Text: "a", Color: "red-4"}},
		"负数主键":  {{ID: -1, Text: "a"}},
		"重复主键":  {{ID: 5, Text: "a"}, {ID: 5, Text: "b"}},
		"超过上限":  tooMany,
		"换行的文字": {{Text: "a\nb"}},
	}
	for name, in := range cases {
		if _, err := e.core.UpdateBoard(context.Background(), testActor, e.board.ID, kanban.UpdateBoardInput{Tags: &in}); !errors.Is(err, kanban.ErrInvalidInput) {
			t.Errorf("%s 应该被拒绝，实际 %v", name, err)
		}
	}
	blanks := []kanban.TagInput{{}, {}, {Text: "  ", Color: "red-1"}}
	if b := e.setTags(t, blanks); len(b.Tags) != 3 {
		t.Fatalf("空文字的标签可以有多个: %+v", b.Tags)
	}
}

// TestTagNamesRejectWhenLibraryFull 验证标签库满了以后，按文字设置标签遇到新文字会被拒绝，已有文字不受影响。
func TestTagNamesRejectWhenLibraryFull(t *testing.T) {
	e := newCardEnv(t)
	full := make([]kanban.TagInput, 60)
	for i := range full {
		full[i] = kanban.TagInput{Text: strings.Repeat("x", 1+i/26) + string(rune('a'+i%26))}
	}
	e.setTags(t, full)
	c := e.card(t, 0, "卡")
	extra := []string{"新来的"}
	if _, err := e.core.UpdateCard(context.Background(), testActor, c.ID, kanban.UpdateCardInput{Tags: &extra}); !errors.Is(err, kanban.ErrInvalidInput) {
		t.Fatalf("标签库已满应该拒绝新文字，实际 %v", err)
	}
	e.tag(t, c, full[0].Text)
}

// TestSetTagsOnlyHuman 验证 agent 不能改标签库；人可以。
func TestSetTagsOnlyHuman(t *testing.T) {
	e := newCardEnv(t)
	tags := []kanban.TagInput{{Text: "bug"}}
	if _, err := e.core.UpdateBoard(context.Background(), testAgent, e.board.ID, kanban.UpdateBoardInput{Tags: &tags}); !errors.Is(err, kanban.ErrHumanOnly) {
		t.Fatalf("agent 改标签库应该被拒绝，实际 %v", err)
	}
	path := "/tmp"
	if _, err := e.core.UpdateBoard(context.Background(), testAgent, e.board.ID, kanban.UpdateBoardInput{ProjectPath: &path, Tags: &tags}); !errors.Is(err, kanban.ErrHumanOnly) {
		t.Fatalf("agent 同时带标签库和项目路径也应该被拒绝，实际 %v", err)
	}
}

// TestSetTagsIgnoresForeignIDs 验证入参里带着库里没有的主键时，当作新增并分配新主键。
func TestSetTagsIgnoresForeignIDs(t *testing.T) {
	e := newCardEnv(t)
	b := e.setTags(t, []kanban.TagInput{{ID: 424242, Text: "bug"}})
	if len(b.Tags) != 1 || b.Tags[0].ID == 424242 {
		t.Fatalf("陌生主键应该被换掉: %+v", b.Tags)
	}
}

// TestDeleteBoardRemovesTags 验证删除看板会删掉它的标签库，不影响别的看板。
func TestDeleteBoardRemovesTags(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	other := newCardEnvOn(t, e.core, e.db, "OTH")
	if err := e.core.DeleteBoard(ctx, testActor, e.board.ID); err != nil {
		t.Fatalf("删除看板失败: %v", err)
	}
	if got, _ := e.db.Tag().ListByBoard(ctx, e.board.ID); len(got) != 0 {
		t.Fatalf("被删看板的标签应该一起删掉: %+v", got)
	}
	if got, _ := e.db.Tag().ListByBoard(ctx, other.board.ID); len(got) != 6 {
		t.Fatalf("别的看板的标签不应该受影响: %+v", got)
	}
}

// TestRuleAddAndRemoveTag 验证规则按文字加减标签：加的文字库里已有就复用（不写标签库事件），
// 去掉的文字库里没有就什么都不做，没有文字的标签不受影响。
func TestRuleAddAndRemoveTag(t *testing.T) {
	e := newClaimEnv(t)
	blank := e.setTags(t, []kanban.TagInput{{Text: "bug", Color: "red-2"}, {Color: "green-3"}, {Text: "急"}}).Tags
	e.enterRule(t, "进进行中", e.col(colDoing),
		kanban.RuleAction{Type: kanban.ActionAddTag, Tag: "BUG"},
		kanban.RuleAction{Type: kanban.ActionRemoveTag, Tag: "急"},
		kanban.RuleAction{Type: kanban.ActionRemoveTag, Tag: "不存在"},
	)
	c := e.card(t, colTodo, "卡")
	e.tagIDs(t, c, blank[2].ID, blank[1].ID)
	events := len(e.events(t))

	moved := e.moveTo(t, testActor, c.ID, colDoing)
	if !slices.Equal(moved.TagIDs, []int64{blank[1].ID, blank[0].ID}) {
		t.Fatalf("应该去掉急、保留无文字标签、再加上 bug: %v", moved.TagIDs)
	}
	for _, typ := range e.eventTypes(t, events) {
		if typ == kanban.EventBoardLabelsUpdated {
			t.Fatalf("文字在库里已有，不应该写标签库事件")
		}
	}
}

// TestRuleAddTagSkipsWhenLibraryFull 验证标签库满了时，规则要加的新文字被跳过并写进 automation.fired 的 skipped，任务不变。
func TestRuleAddTagSkipsWhenLibraryFull(t *testing.T) {
	e := newClaimEnv(t)
	full := make([]kanban.TagInput, 60)
	for i := range full {
		full[i] = kanban.TagInput{Text: strings.Repeat("x", 1+i/26) + string(rune('a'+i%26))}
	}
	e.setTags(t, full)
	e.enterRule(t, "加新标签", e.col(colDoing), kanban.RuleAction{Type: kanban.ActionAddTag, Tag: "全新"})
	c := e.card(t, colTodo, "卡")
	moved := e.moveTo(t, testActor, c.ID, colDoing)
	if len(moved.TagIDs) != 0 {
		t.Fatalf("库满时规则不应该加上标签: %v", moved.TagIDs)
	}
	if got := e.fired(t); len(got) != 1 || !strings.Contains(got[0].Payload, "skipped") || !strings.Contains(got[0].Payload, "全新") {
		t.Fatalf("应该记一条带 skipped 的 automation.fired: %+v", got)
	}
}
