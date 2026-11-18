package kanban_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
)

// dev 模板里各列的下标：收件箱、待办、进行中（agent 领活）、审查、完成。
const (
	colIdea = iota
	colTodo
	colDoing
	colReview
	colDone
)

// cardEnv 是卡片测试的公共环境：一个用 dev 模板建好的看板（标识 TST，五列），不带起步规则。
type cardEnv struct {
	core  kanban.Core
	db    kanbandb.DB
	board *kanban.BoardDetail
}

// newCardEnv 创建卡片测试环境。起步规则会在创建和换列时改卡片、写 automation.fired 事件，
// 会干扰卡片测试对版本和事件的断言，所以这里把它们删掉；要测自动化的用例自己创建规则。
func newCardEnv(t *testing.T, opts ...kanban.Option) *cardEnv {
	t.Helper()
	core, db := newBare(t, opts...)
	return newCardEnvOn(t, core, db, "TST")
}

// newCardEnvOn 在已有的 Core 上再建一个标识为 slug 的看板，用于需要两个看板的测试。
func newCardEnvOn(t *testing.T, core kanban.Core, db kanbandb.DB, slug string) *cardEnv {
	t.Helper()
	board := mustCreateBoard(t, core, kanban.CreateBoardInput{Name: "卡片测试 " + slug, Slug: slug, Locale: "zh-CN"})
	if err := db.Rule().DeleteByBoard(context.Background(), board.ID); err != nil {
		t.Fatalf("清掉起步规则失败: %v", err)
	}
	board.Rules = []*kanban.Rule{}
	return &cardEnv{core: core, db: db, board: board}
}

// col 返回 dev 模板里第 i 列的主键。
func (e *cardEnv) col(i int) int64 { return e.board.Columns[i].ID }

// card 以人的身份在第 i 列创建一张只有标题的卡片，失败就让测试终止。
func (e *cardEnv) card(t *testing.T, column int, title string) *kanban.Card {
	t.Helper()
	c, err := e.core.CreateCard(context.Background(), testActor, e.board.ID, kanban.CreateCardInput{Title: title, ColumnID: e.col(column)})
	if err != nil {
		t.Fatalf("创建卡片 %s 失败: %v", title, err)
	}
	return c
}

// events 返回看板里的全部事件。
func (e *cardEnv) events(t *testing.T) []*kanban.Event {
	t.Helper()
	return boardEvents(t, e.db, e.board.ID)
}

// lastEvent 返回看板里最新的一条事件。
func (e *cardEnv) lastEvent(t *testing.T) *kanban.Event {
	t.Helper()
	events := e.events(t)
	return events[len(events)-1]
}

// stored 直接从数据库读出卡片的当前状态。
func (e *cardEnv) stored(t *testing.T, id int64) *kanban.Card {
	t.Helper()
	c, err := e.db.Card().GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("读取卡片 %d 失败: %v", id, err)
	}
	return c
}

// TestCreateCardDefaults 验证最简单的创建：编号从 1 递增，默认放进第一列，位置依次是 1024、2048，
// 版本从 1 开始，标签和依赖是空数组，并追加 card.created 事件。
func TestCreateCardDefaults(t *testing.T) {
	ctx := context.Background()
	fixed := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	e := newCardEnv(t, kanban.WithClock(func() time.Time { return fixed }))

	a, err := e.core.CreateCard(ctx, testActor, e.board.ID, kanban.CreateCardInput{Title: "  第一张  "})
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if a.Key != "TST-1" || a.Number != 1 || a.Title != "第一张" || a.ColumnID != e.col(colIdea) ||
		a.Position != 1024 || a.Version != 1 || a.CreatedBy != "admin" || a.UpdatedBy != "admin" ||
		!a.CreatedAt.Equal(fixed) || a.TagIDs == nil || a.WaitsOn == nil || a.ClaimedBy != "" {
		t.Fatalf("第一张卡片的字段不对: %+v", a)
	}
	b := e.card(t, colIdea, "第二张")
	if b.Key != "TST-2" || b.Position != 2048 {
		t.Fatalf("第二张的编号或位置不对: %+v", b)
	}
	if got := e.stored(t, a.ID); got.Key != "TST-1" || len(got.TagIDs) != 0 {
		t.Fatalf("读出来的卡片不对: %+v", got)
	}

	last := e.lastEvent(t)
	want := fmt.Sprintf(`{"column":"收件箱","column_id":%d,"key":"TST-2","title":"第二张"}`, e.col(colIdea))
	if last.Type != kanban.EventCardCreated || last.CardID != b.ID || last.Payload != want {
		t.Fatalf("事件不对: %+v，期望内容 %s", last, want)
	}
	got, err := e.core.GetCard(ctx, a.ID)
	if err != nil || got.Key != "TST-1" {
		t.Fatalf("GetCard 失败: %+v %v", got, err)
	}
	if _, err := e.core.GetCard(ctx, 9999); !errors.Is(err, kanban.ErrCardNotFound) {
		t.Fatalf("不存在应该得到 ErrCardNotFound，实际 %v", err)
	}
}

// TestCreateCardAllFields 验证人创建时可以填全部字段：标签去重（保留首次写法）、
// 依赖、带时区的时间转成 UTC、分支和链接。
func TestCreateCardAllFields(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	dep := e.card(t, colTodo, "被依赖的卡")

	c, err := e.core.CreateCard(ctx, testActor, e.board.ID, kanban.CreateCardInput{
		Title: "全字段", Description: "描述", ColumnID: e.col(colTodo), Priority: 3,
		Tags: []string{" Bug ", "bug", "后端"}, HumanOnly: true, WaitsOn: []int64{dep.ID, dep.ID},
		DueAt: "2026-12-01T09:30:00+08:00", StartedAt: "2026-11-01T00:00:00Z", Branch: "feat/x", URL: "https://example.com/pr/1",
	})
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	got := e.stored(t, c.ID)
	if got.Description != "描述" || got.Priority != 3 || !got.HumanOnly || got.Branch != "feat/x" || got.URL != "https://example.com/pr/1" ||
		strings.Join(e.names(t, got), ",") != "Bug,后端" || len(got.WaitsOn) != 1 || got.WaitsOn[0] != dep.ID {
		t.Fatalf("字段不对: %+v", got)
	}
	if got.DueAt == nil || !got.DueAt.Equal(time.Date(2026, 12, 1, 1, 30, 0, 0, time.UTC)) || got.DueAt.Location() != time.UTC {
		t.Fatalf("到期时间应该转成 UTC，实际 %v", got.DueAt)
	}
	if got.StartedAt == nil || got.FinishedAt != nil {
		t.Fatalf("开始时间应该有、完成时间应该没有: %+v", got)
	}
}

// TestCreateCardColumnRules 验证目标列：指定的列必须属于这个看板；没有列的看板创建失败。
func TestCreateCardColumnRules(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	other := mustCreateBoard(t, e.core, kanban.CreateBoardInput{Name: "别的", Slug: "OTHER", Template: kanban.TemplateSimple})
	empty := mustCreateBoard(t, e.core, kanban.CreateBoardInput{Name: "空", Slug: "EMPTY", Template: kanban.TemplateEmpty})

	for name, in := range map[string]kanban.CreateCardInput{
		"列不存在":    {Title: "x", ColumnID: 9999},
		"列属于别的看板": {Title: "x", ColumnID: other.Columns[0].ID},
	} {
		if _, err := e.core.CreateCard(ctx, testActor, e.board.ID, in); !errors.Is(err, kanban.ErrInvalidInput) {
			t.Errorf("%s：应该得到 ErrInvalidInput，实际 %v", name, err)
		}
	}
	if _, err := e.core.CreateCard(ctx, testActor, empty.ID, kanban.CreateCardInput{Title: "x"}); !errors.Is(err, kanban.ErrInvalidInput) {
		t.Fatalf("没有列的看板应该得到 ErrInvalidInput，实际 %v", err)
	}
	if _, err := e.core.CreateCard(ctx, testActor, 9999, kanban.CreateCardInput{Title: "x"}); !errors.Is(err, kanban.ErrBoardNotFound) {
		t.Fatalf("看板不存在应该得到 ErrBoardNotFound，实际 %v", err)
	}
	if cards, _ := e.db.Card().ListByBoard(ctx, e.board.ID); len(cards) != 0 {
		t.Fatalf("失败不应该留下卡片，实际 %d", len(cards))
	}
}

// TestCreateCardByAgent 验证 agent 创建卡片：只能填标题、描述、标签、分支、链接；
// 不指定列时进第一列；不能放进开启 agent 领取的列；填规划类字段被拒绝。
func TestCreateCardByAgent(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)

	c, err := e.core.CreateCard(ctx, testAgent, e.board.ID, kanban.CreateCardInput{Title: "agent 发现的问题", Tags: []string{"bug"}, Branch: "fix/a"})
	if err != nil || c.ColumnID != e.col(colIdea) || c.CreatedBy != "claude" {
		t.Fatalf("agent 应该能创建到第一列: %+v %v", c, err)
	}
	if ev := e.lastEvent(t); ev.ActorKind != "agent" || ev.Actor != "claude" {
		t.Fatalf("事件应该记录 agent 为操作者: %+v", ev)
	}
	if _, err := e.core.CreateCard(ctx, testAgent, e.board.ID, kanban.CreateCardInput{Title: "x", ColumnID: e.col(colDoing)}); !errors.Is(err, kanban.ErrAgentPickupColumn) {
		t.Fatalf("放进领活列应该得到 ErrAgentPickupColumn，实际 %v", err)
	}
	for _, in := range []kanban.CreateCardInput{
		{Title: "x", Priority: 2},
		{Title: "x", HumanOnly: true},
		{Title: "x", WaitsOn: []int64{c.ID}},
		{Title: "x", DueAt: "2026-01-01T00:00:00Z"},
		{Title: "x", FinishedAt: "2026-01-01T00:00:00Z"},
	} {
		if _, err := e.core.CreateCard(ctx, testAgent, e.board.ID, in); !errors.Is(err, kanban.ErrHumanOnly) {
			t.Errorf("agent 填规划类字段应该得到 ErrHumanOnly，实际 %v", err)
		}
	}
	system := kanban.AutomationActor("规则")
	if _, err := e.core.CreateCard(ctx, system, e.board.ID, kanban.CreateCardInput{Title: "x"}); !errors.Is(err, kanban.ErrHumanOnly) {
		t.Fatalf("系统操作者不能走创建入口，实际 %v", err)
	}
	if _, err := e.core.CreateCard(ctx, kanban.Actor{}, e.board.ID, kanban.CreateCardInput{Title: "x"}); !errors.Is(err, kanban.ErrInvalidActor) {
		t.Fatalf("身份无效应该得到 ErrInvalidActor，实际 %v", err)
	}
}

// TestCreateCardRejectsInvalidInput 验证各字段的校验：每种不合法的输入都返回 ErrInvalidInput，且不留下卡片。
func TestCreateCardRejectsInvalidInput(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	e.card(t, colIdea, "已有的卡")
	other := mustCreateBoard(t, e.core, kanban.CreateBoardInput{Name: "别的", Slug: "OTHER", Template: kanban.TemplateSimple})
	foreign, err := e.core.CreateCard(ctx, testActor, other.ID, kanban.CreateCardInput{Title: "别的看板的卡"})
	if err != nil {
		t.Fatalf("创建别的看板的卡片失败: %v", err)
	}
	tooManyTags := make([]string, 11)
	for i := range tooManyTags {
		tooManyTags[i] = fmt.Sprintf("t%d", i)
	}
	cases := map[string]kanban.CreateCardInput{
		"标题为空":       {Title: "  "},
		"标题太长":       {Title: strings.Repeat("长", 201)},
		"标题有换行":      {Title: "a\nb"},
		"描述太长":       {Title: "x", Description: strings.Repeat("长", 20001)},
		"优先级过大":      {Title: "x", Priority: 4},
		"优先级为负":      {Title: "x", Priority: -1},
		"标签太多":       {Title: "x", Tags: tooManyTags},
		"标签为空":       {Title: "x", Tags: []string{" "}},
		"标签太长":       {Title: "x", Tags: []string{strings.Repeat("a", 31)}},
		"依赖不存在":      {Title: "x", WaitsOn: []int64{9999}},
		"依赖是负数":      {Title: "x", WaitsOn: []int64{-1}},
		"依赖太多":       {Title: "x", WaitsOn: seq(21)},
		"到期时间格式不对":   {Title: "x", DueAt: "2026-12-01"},
		"开始时间格式不对":   {Title: "x", StartedAt: "昨天"},
		"分支太长":       {Title: "x", Branch: strings.Repeat("a", 201)},
		"链接不是 http":  {Title: "x", URL: "ftp://example.com"},
		"链接是脚本":      {Title: "x", URL: "javascript:alert(1)"},
		"链接太长":       {Title: "x", URL: "https://" + strings.Repeat("a", 500)},
		"依赖是别的看板的卡片": {Title: "x", WaitsOn: []int64{foreign.ID}},
	}
	for name, in := range cases {
		if _, err := e.core.CreateCard(ctx, testActor, e.board.ID, in); !errors.Is(err, kanban.ErrInvalidInput) {
			t.Errorf("%s：应该得到 ErrInvalidInput，实际 %v", name, err)
		}
	}
	if cards, _ := e.db.Card().ListByBoard(ctx, e.board.ID); len(cards) != 1 {
		t.Fatalf("失败不应该留下卡片，实际 %d 张", len(cards))
	}
}

// seq 返回 1 到 n 的整数列表。
func seq(n int) []int64 {
	out := make([]int64, n)
	for i := range out {
		out[i] = int64(i + 1)
	}
	return out
}

// TestCreateCardNumbersAreUniqueUnderConcurrency 验证并发创建时编号不重复也没有空洞；
// 删除卡片后编号不回收。
func TestCreateCardNumbersAreUniqueUnderConcurrency(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	const n = 10
	keys := make(chan string, n)
	within(t, 30*time.Second, func() {
		var wg sync.WaitGroup
		for i := range n {
			wg.Go(func() {
				c, err := e.core.CreateCard(ctx, testActor, e.board.ID, kanban.CreateCardInput{Title: fmt.Sprintf("并发 %d", i)})
				if err != nil {
					t.Errorf("并发创建失败: %v", err)
					return
				}
				keys <- c.Key
			})
		}
		wg.Wait()
	})
	close(keys)
	seen := map[string]bool{}
	for k := range keys {
		seen[k] = true
	}
	for i := 1; i <= n; i++ {
		if !seen[fmt.Sprintf("TST-%d", i)] {
			t.Fatalf("缺少编号 TST-%d，实际 %v", i, seen)
		}
	}

	last, _ := e.db.Card().GetByKey(ctx, fmt.Sprintf("TST-%d", n))
	if err := e.db.Card().Delete(ctx, last); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if next := e.card(t, colIdea, "删除后再建"); next.Key != fmt.Sprintf("TST-%d", n+1) {
		t.Fatalf("编号不应该回收，实际 %s", next.Key)
	}
}

// hold 直接在数据库里把卡片标成由某个 agent 持有。领取功能有自己的测试，
// 这里只需要"被持有"这个状态，用它测试权限判断。
func (e *cardEnv) hold(t *testing.T, id int64, agent string) {
	t.Helper()
	c := e.stored(t, id)
	expires := time.Now().Add(time.Hour).UTC()
	c.ClaimedBy, c.ClaimExpiresAt = agent, &expires
	if err := e.db.Card().Save(context.Background(), c); err != nil {
		t.Fatalf("标记持有失败: %v", err)
	}
}

// intPtr 返回整数的指针，用来构造"出现的字段"。
func intPtr(n int) *int { return &n }

// TestUpdateCardByHuman 验证人修改卡片：只更新出现的字段，版本加一，记录修改者，
// 事件里记录变化的字段名；值没变的字段不算变化。
func TestUpdateCardByHuman(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	c := e.card(t, colTodo, "旧标题")
	tags := []string{"a", "A", "b"}
	due := "2026-12-01T00:00:00Z"

	got, err := e.core.UpdateCard(ctx, testActor, c.ID, kanban.UpdateCardInput{
		Title: strPtr(" 新标题 "), Description: strPtr("说明"), Priority: intPtr(2), Tags: &tags, DueAt: &due,
		Branch: strPtr("feat/a"), URL: strPtr("https://example.com/1"),
	})
	if err != nil {
		t.Fatalf("修改失败: %v", err)
	}
	if got.Title != "新标题" || got.Priority != 2 || got.Version != 2 || got.UpdatedBy != "admin" || got.Position != c.Position {
		t.Fatalf("返回的卡片不对: %+v", got)
	}
	stored := e.stored(t, c.ID)
	if stored.Title != "新标题" || stored.Description != "说明" || strings.Join(e.names(t, stored), ",") != "a,b" ||
		stored.DueAt == nil || stored.Branch != "feat/a" || stored.URL != "https://example.com/1" || stored.Version != 2 {
		t.Fatalf("读出来的卡片不对: %+v", stored)
	}
	// 同时改了标签和别的字段：别的字段一条 card.updated（不含 tags），标签另写一条 card.tags_updated
	want := `{"fields":["title","description","branch","url","priority","due_at"],"key":"TST-1","title":"新标题"}`
	events := e.events(t)
	updated, tagged := events[len(events)-2], events[len(events)-1]
	if updated.Type != kanban.EventCardUpdated || updated.CardID != c.ID || updated.Payload != want {
		t.Fatalf("事件不对: %+v，期望内容 %s", updated, want)
	}
	wantTags := `{"fields":["tag_ids"],"key":"TST-1","title":"新标题"}`
	if tagged.Type != kanban.EventCardTagsUpdated || tagged.CardID != c.ID || tagged.Payload != wantTags {
		t.Fatalf("标签事件不对: %+v，期望内容 %s", tagged, wantTags)
	}
}

// TestUpdateCardTagsOnlyEvent 验证只改标签时只写 card.tags_updated，不写 card.updated；
// 只改别的字段时不写标签事件。
func TestUpdateCardTagsOnlyEvent(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	c := e.card(t, colTodo, "标签事件")
	from := len(e.events(t))

	tags := []string{"Bug"}
	if _, err := e.core.UpdateCard(ctx, testActor, c.ID, kanban.UpdateCardInput{Tags: &tags}); err != nil {
		t.Fatalf("改标签失败: %v", err)
	}
	if got := strings.Join(e.eventTypes(t, from), ","); got != kanban.EventCardTagsUpdated {
		t.Fatalf("只改标签应该只写标签事件，实际: %s", got)
	}

	from = len(e.events(t))
	if _, err := e.core.UpdateCard(ctx, testActor, c.ID, kanban.UpdateCardInput{Priority: intPtr(3)}); err != nil {
		t.Fatalf("改优先级失败: %v", err)
	}
	if got := strings.Join(e.eventTypes(t, from), ","); got != kanban.EventCardUpdated {
		t.Fatalf("只改优先级不应该写标签事件，实际: %s", got)
	}
}

// TestUpdateCardClearsFields 验证清空：标签和依赖传空数组、时间传空字符串、布尔值传 false 都能真正清掉。
func TestUpdateCardClearsFields(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	dep := e.card(t, colTodo, "依赖")
	c, err := e.core.CreateCard(ctx, testActor, e.board.ID, kanban.CreateCardInput{
		Title: "x", Tags: []string{"a"}, WaitsOn: []int64{dep.ID}, HumanOnly: true, DueAt: "2026-01-01T00:00:00Z", Priority: 3,
	})
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	none, noDeps, empty, no := []string{}, []int64{}, "", false
	if _, err := e.core.UpdateCard(ctx, testActor, c.ID, kanban.UpdateCardInput{
		Tags: &none, WaitsOn: &noDeps, DueAt: &empty, HumanOnly: &no, Priority: intPtr(0),
	}); err != nil {
		t.Fatalf("清空失败: %v", err)
	}
	got := e.stored(t, c.ID)
	if len(got.TagIDs) != 0 || len(got.WaitsOn) != 0 || got.DueAt != nil || got.HumanOnly || got.Priority != 0 {
		t.Fatalf("字段应该都被清掉: %+v", got)
	}
}

// TestUpdateCardNoChange 验证没有真实变化时不涨版本、不写事件：空请求、值相同、标题只多了空白、
// 标签传空数组而卡片本来就没有标签、优先级和到期时间传的就是现有值。
func TestUpdateCardNoChange(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	c := e.card(t, colTodo, "标题")
	before := len(e.events(t))
	same := []string{}

	for _, in := range []kanban.UpdateCardInput{
		{}, {Title: strPtr("标题")}, {Title: strPtr("  标题 ")}, {Tags: &same}, {Priority: intPtr(0)}, {DueAt: strPtr("")},
	} {
		got, err := e.core.UpdateCard(ctx, testActor, c.ID, in)
		if err != nil || got.Version != 1 {
			t.Fatalf("没有变化也应该成功且版本不变: %+v %v", got, err)
		}
	}
	if after := len(e.events(t)); after != before {
		t.Fatalf("没有变化不应该写事件，事件数 %d → %d", before, after)
	}
	if e.stored(t, c.ID).Version != 1 {
		t.Fatal("数据库里的版本不应该变化")
	}
}

// TestUpdateCardVersionConflict 验证乐观锁：带着过期的版本号提交返回 409 且不生效，
// 带当前版本号可以成功，不带版本号不检查。
func TestUpdateCardVersionConflict(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	c := e.card(t, colTodo, "标题")
	one, two := int64(1), int64(2)

	if _, err := e.core.UpdateCard(ctx, testActor, c.ID, kanban.UpdateCardInput{Title: strPtr("第二版"), Version: &one}); err != nil {
		t.Fatalf("版本一致应该成功: %v", err)
	}
	_, err := e.core.UpdateCard(ctx, testActor, c.ID, kanban.UpdateCardInput{Title: strPtr("过期的修改"), Version: &one})
	if !errors.Is(err, kanban.ErrCardVersionConflict) {
		t.Fatalf("版本过期应该得到 ErrCardVersionConflict，实际 %v", err)
	}
	if e.stored(t, c.ID).Title != "第二版" {
		t.Fatal("冲突的修改不应该生效")
	}
	if _, err := e.core.UpdateCard(ctx, testActor, c.ID, kanban.UpdateCardInput{Title: strPtr("第三版"), Version: &two}); err != nil {
		t.Fatalf("带当前版本应该成功: %v", err)
	}
	if _, err := e.core.UpdateCard(ctx, testActor, c.ID, kanban.UpdateCardInput{Title: strPtr("不带版本")}); err != nil {
		t.Fatalf("不带版本不检查: %v", err)
	}
}

// TestUpdateCardByAgent 验证 agent 的权限：只能改自己持有的卡片，并且只能改五个允许的字段；
// 没持有、被别的 agent 持有都得到 ErrNotHolder；碰规划类字段得到 ErrHumanOnly。
func TestUpdateCardByAgent(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	c := e.card(t, colDoing, "标题")
	patch := kanban.UpdateCardInput{Title: strPtr("agent 改的")}

	if _, err := e.core.UpdateCard(ctx, testAgent, c.ID, patch); !errors.Is(err, kanban.ErrNotHolder) {
		t.Fatalf("没持有应该得到 ErrNotHolder，实际 %v", err)
	}
	e.hold(t, c.ID, "someone-else")
	if _, err := e.core.UpdateCard(ctx, testAgent, c.ID, patch); !errors.Is(err, kanban.ErrNotHolder) {
		t.Fatalf("被别的 agent 持有应该得到 ErrNotHolder，实际 %v", err)
	}

	e.hold(t, c.ID, testAgent.Name)
	tags := []string{"done"}
	got, err := e.core.UpdateCard(ctx, testAgent, c.ID, kanban.UpdateCardInput{
		Title: strPtr("agent 改的"), Description: strPtr("进展"), Tags: &tags, Branch: strPtr("feat/x"), URL: strPtr("https://example.com/pr/2"),
	})
	if err != nil || got.UpdatedBy != "claude" || got.Version != 2 {
		t.Fatalf("持有者应该能改允许的字段: %+v %v", got, err)
	}
	if ev := e.lastEvent(t); ev.ActorKind != "agent" || ev.Actor != "claude" {
		t.Fatalf("事件应该记录 agent: %+v", ev)
	}

	yes, due := true, "2026-01-01T00:00:00Z"
	deps := []int64{}
	for name, in := range map[string]kanban.UpdateCardInput{
		"优先级": {Priority: intPtr(3)}, "仅限人处理": {HumanOnly: &yes}, "依赖": {WaitsOn: &deps},
		"到期时间": {DueAt: &due}, "开始时间": {StartedAt: &due}, "完成时间": {FinishedAt: &due},
		"混合": {Title: strPtr("x"), Priority: intPtr(1)},
	} {
		if _, err := e.core.UpdateCard(ctx, testAgent, c.ID, in); !errors.Is(err, kanban.ErrHumanOnly) {
			t.Errorf("%s：应该得到 ErrHumanOnly，实际 %v", name, err)
		}
	}
	if e.stored(t, c.ID).Priority != 0 {
		t.Fatal("被拒绝的修改不应该生效")
	}
}

// TestUpdateCardDependencies 验证修改依赖时的校验：能设置、能清除；指向自己、成环、不存在的卡都返回 ErrInvalidInput，
// 并且失败的修改不生效。
func TestUpdateCardDependencies(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	a, b, c := e.card(t, colTodo, "A"), e.card(t, colTodo, "B"), e.card(t, colTodo, "C")
	set := func(id int64, deps ...int64) error {
		_, err := e.core.UpdateCard(ctx, testActor, id, kanban.UpdateCardInput{WaitsOn: &deps})
		return err
	}

	if err := set(a.ID, b.ID); err != nil { // A 等 B
		t.Fatalf("设置依赖失败: %v", err)
	}
	if err := set(b.ID, c.ID); err != nil { // B 等 C
		t.Fatalf("设置依赖失败: %v", err)
	}
	for name, tc := range map[string]struct {
		card int64
		deps []int64
	}{
		"依赖自己":   {a.ID, []int64{a.ID}},
		"直接成环":   {b.ID, []int64{a.ID}},
		"间接成环":   {c.ID, []int64{a.ID}},
		"依赖不存在的": {c.ID, []int64{9999}},
	} {
		if err := set(tc.card, tc.deps...); !errors.Is(err, kanban.ErrInvalidInput) {
			t.Errorf("%s：应该得到 ErrInvalidInput，实际 %v", name, err)
		}
	}
	if got := e.stored(t, c.ID); len(got.WaitsOn) != 0 || got.Version != 1 {
		t.Fatalf("失败的修改不应该生效: %+v", got)
	}
	if err := set(a.ID); err != nil {
		t.Fatalf("清除依赖失败: %v", err)
	}
	if got := e.stored(t, a.ID); len(got.WaitsOn) != 0 {
		t.Fatalf("依赖应该已清除: %+v", got)
	}
}

// TestUpdateCardRejects 验证其他拒绝情况：卡片不存在、输入不合法、身份无效，失败都不改动卡片。
func TestUpdateCardRejects(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	c := e.card(t, colTodo, "标题")

	if _, err := e.core.UpdateCard(ctx, testActor, 9999, kanban.UpdateCardInput{Title: strPtr("x")}); !errors.Is(err, kanban.ErrCardNotFound) {
		t.Fatalf("不存在应该得到 ErrCardNotFound，实际 %v", err)
	}
	for name, in := range map[string]kanban.UpdateCardInput{
		"标题为空": {Title: strPtr(" ")}, "优先级越界": {Priority: intPtr(9)},
		"时间格式不对": {DueAt: strPtr("明天")}, "链接不合法": {URL: strPtr("file:///etc/passwd")},
		"前面合法后面不合法": {Title: strPtr("新"), Branch: strPtr(strings.Repeat("a", 201))},
	} {
		if _, err := e.core.UpdateCard(ctx, testActor, c.ID, in); !errors.Is(err, kanban.ErrInvalidInput) {
			t.Errorf("%s：应该得到 ErrInvalidInput，实际 %v", name, err)
		}
	}
	if _, err := e.core.UpdateCard(ctx, kanban.Actor{}, c.ID, kanban.UpdateCardInput{Title: strPtr("x")}); !errors.Is(err, kanban.ErrInvalidActor) {
		t.Fatalf("身份无效应该得到 ErrInvalidActor，实际 %v", err)
	}
	if got := e.stored(t, c.ID); got.Title != "标题" || got.Version != 1 {
		t.Fatalf("失败的修改不应该生效: %+v", got)
	}
}
