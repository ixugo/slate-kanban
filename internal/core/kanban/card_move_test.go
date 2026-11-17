package kanban_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// titles 按位置顺序返回第 i 列的卡片标题，用逗号连接，空列返回空字符串。
func (e *cardEnv) titles(t *testing.T, column int) string {
	t.Helper()
	cards, err := e.db.Card().ListByColumn(context.Background(), e.col(column))
	if err != nil {
		t.Fatalf("读取列 %d 的卡片失败: %v", column, err)
	}
	names := make([]string, 0, len(cards))
	for _, c := range cards {
		names = append(names, c.Title)
	}
	return strings.Join(names, ",")
}

// TestMoveCardToAnotherColumn 验证换列：落到目标列末尾，版本加一，记录修改者，
// 并追加 card.moved 事件，事件里有原列和目标列的主键与名称。
func TestMoveCardToAnotherColumn(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	a := e.card(t, colTodo, "A")
	e.card(t, colDoing, "已有")

	got, err := e.core.MoveCard(ctx, testActor, a.ID, kanban.MoveCardInput{ColumnID: e.col(colDoing)})
	if err != nil {
		t.Fatalf("移动失败: %v", err)
	}
	if got.ColumnID != e.col(colDoing) || got.Position != 2048 || got.Version != 2 || got.UpdatedBy != "admin" {
		t.Fatalf("返回的卡片不对: %+v", got)
	}
	if e.titles(t, colDoing) != "已有,A" || e.titles(t, colTodo) != "" {
		t.Fatalf("移动后各列的卡片不对: 进行中=%s 待办=%s", e.titles(t, colDoing), e.titles(t, colTodo))
	}
	want := fmt.Sprintf(`{"from":"待办","from_id":%d,"key":"TST-1","title":"A","to":"进行中","to_id":%d}`, e.col(colTodo), e.col(colDoing))
	if ev := e.lastEvent(t); ev.Type != kanban.EventCardMoved || ev.CardID != a.ID || ev.Payload != want {
		t.Fatalf("事件不对: %+v，期望内容 %s", ev, want)
	}
}

// TestMoveCardPlacement 验证换列时按参照卡片放置：不指定放末尾，指定前面或后面放到对应位置。
func TestMoveCardPlacement(t *testing.T) {
	cases := []struct {
		name          string
		before, after string // 参照卡片的标题，空表示不指定
		want          string
	}{
		{"不指定放到末尾", "", "", "X,Y,Z,M"},
		{"放到第一张前面", "X", "", "M,X,Y,Z"},
		{"放到中间一张前面", "Z", "", "X,Y,M,Z"},
		{"放到第一张后面", "", "X", "X,M,Y,Z"},
		{"放到最后一张后面", "", "Z", "X,Y,Z,M"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newCardEnv(t)
			ids := map[string]int64{}
			for _, title := range []string{"X", "Y", "Z"} {
				ids[title] = e.card(t, colDoing, title).ID
			}
			m := e.card(t, colTodo, "M")

			in := kanban.MoveCardInput{ColumnID: e.col(colDoing), BeforeID: ids[c.before], AfterID: ids[c.after]}
			if _, err := e.core.MoveCard(context.Background(), testActor, m.ID, in); err != nil {
				t.Fatalf("移动失败: %v", err)
			}
			if got := e.titles(t, colDoing); got != c.want {
				t.Fatalf("期望顺序 %s，实际 %s", c.want, got)
			}
		})
	}
}

// TestMoveCardReorderSameColumn 验证同列调整顺序：写 card.reordered 而不是 card.moved，
// 事件里只有编号和列；ColumnID 留空和显式写当前列效果一样。
func TestMoveCardReorderSameColumn(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	a, b, c := e.card(t, colTodo, "A"), e.card(t, colTodo, "B"), e.card(t, colTodo, "C")

	got, err := e.core.MoveCard(ctx, testActor, c.ID, kanban.MoveCardInput{BeforeID: a.ID})
	if err != nil || got.Version != 2 || got.ColumnID != e.col(colTodo) {
		t.Fatalf("同列调整失败: %+v %v", got, err)
	}
	if e.titles(t, colTodo) != "C,A,B" {
		t.Fatalf("顺序不对: %s", e.titles(t, colTodo))
	}
	want := fmt.Sprintf(`{"column_id":%d,"key":"TST-3"}`, e.col(colTodo))
	if ev := e.lastEvent(t); ev.Type != kanban.EventCardReordered || ev.CardID != c.ID || ev.Payload != want {
		t.Fatalf("事件不对: %+v，期望内容 %s", ev, want)
	}

	if _, err := e.core.MoveCard(ctx, testActor, a.ID, kanban.MoveCardInput{ColumnID: e.col(colTodo), AfterID: b.ID}); err != nil {
		t.Fatalf("显式写当前列失败: %v", err)
	}
	if e.titles(t, colTodo) != "C,B,A" {
		t.Fatalf("顺序不对: %s", e.titles(t, colTodo))
	}
}

// TestMoveCardNoChange 验证位置没有变化时什么都不做：放回原来的位置、放到本来就在的列末尾、
// 以及卡片位置不是整数倍时放回原处，都不涨版本、不写事件、不改位置。
func TestMoveCardNoChange(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	a, b, c := e.card(t, colTodo, "A"), e.card(t, colTodo, "B"), e.card(t, colTodo, "C")
	if err := e.db.Card().SetPosition(ctx, b.ID, 1500); err != nil { // 不是 A 和 C 的中点，放回原处时要靠顺序判断
		t.Fatalf("准备数据失败: %v", err)
	}
	events := len(e.events(t))

	for name, in := range map[string]struct {
		id int64
		in kanban.MoveCardInput
	}{
		"第一张放到自己下一张的前面":      {a.ID, kanban.MoveCardInput{BeforeID: b.ID}},
		"中间的卡片放到下一张前面":       {b.ID, kanban.MoveCardInput{BeforeID: c.ID}},
		"中间的卡片放到上一张后面":       {b.ID, kanban.MoveCardInput{AfterID: a.ID}},
		"最后一张不指定参照放到末尾":      {c.ID, kanban.MoveCardInput{}},
		"最后一张显式写当前列并放在上一张后面": {c.ID, kanban.MoveCardInput{ColumnID: e.col(colTodo), AfterID: b.ID}},
	} {
		if _, err := e.core.MoveCard(ctx, testActor, in.id, in.in); err != nil {
			t.Fatalf("%s：应该成功，实际 %v", name, err)
		}
	}
	if after := len(e.events(t)); after != events {
		t.Fatalf("位置没变不应该写事件，事件数 %d → %d", events, after)
	}
	if got := e.stored(t, b.ID); got.Position != 1500 || got.Version != 1 {
		t.Fatalf("位置没变不应该改卡片: %+v", got)
	}
}

// TestMoveCardReleasesClaim 验证换列会清除领取：先写 card.released（带上原持有者和原因），
// 再写 card.moved；没有被领取的卡片换列不写 card.released；同列调整顺序不清除领取。
func TestMoveCardReleasesClaim(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	a := e.card(t, colDoing, "A")
	other := e.card(t, colDoing, "B")
	e.hold(t, a.ID, "claude")

	if _, err := e.core.MoveCard(ctx, testActor, a.ID, kanban.MoveCardInput{BeforeID: other.ID}); err != nil {
		t.Fatalf("同列调整失败: %v", err)
	}
	if got := e.stored(t, a.ID); got.ClaimedBy != "claude" || got.ClaimExpiresAt == nil {
		t.Fatalf("同列调整不应该清除领取: %+v", got)
	}

	if _, err := e.core.MoveCard(ctx, testActor, a.ID, kanban.MoveCardInput{ColumnID: e.col(colReview)}); err != nil {
		t.Fatalf("换列失败: %v", err)
	}
	if got := e.stored(t, a.ID); got.ClaimedBy != "" || got.ClaimExpiresAt != nil {
		t.Fatalf("换列应该清除领取: %+v", got)
	}
	events := e.events(t)
	released, moved := events[len(events)-2], events[len(events)-1]
	if released.Type != kanban.EventCardReleased || released.CardID != a.ID ||
		released.Payload != `{"agent":"claude","key":"TST-1","title":"A","why":"moved"}` {
		t.Fatalf("释放事件不对: %+v", released)
	}
	if moved.Type != kanban.EventCardMoved || moved.CardID != a.ID {
		t.Fatalf("移动事件应该紧跟在释放事件后: %+v", moved)
	}

	before := len(events)
	if _, err := e.core.MoveCard(ctx, testActor, other.ID, kanban.MoveCardInput{ColumnID: e.col(colReview)}); err != nil {
		t.Fatalf("移动没被领取的卡片失败: %v", err)
	}
	if after := e.events(t); len(after) != before+1 || after[before].Type != kanban.EventCardMoved {
		t.Fatalf("没被领取的卡片换列只应该写一条移动事件: %+v", after[before:])
	}
}

// TestMoveCardByAgent 验证 agent 的权限：只能移动自己持有的卡片；可以移到非领活列，换列后领取被清除；
// 不能换进领活列；同列调整顺序不受领活列限制。
func TestMoveCardByAgent(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	a := e.card(t, colDoing, "A")
	b := e.card(t, colDoing, "B")
	review := kanban.MoveCardInput{ColumnID: e.col(colReview)}

	if _, err := e.core.MoveCard(ctx, testAgent, a.ID, review); !errors.Is(err, kanban.ErrNotHolder) {
		t.Fatalf("没持有应该得到 ErrNotHolder，实际 %v", err)
	}
	e.hold(t, a.ID, "someone-else")
	if _, err := e.core.MoveCard(ctx, testAgent, a.ID, review); !errors.Is(err, kanban.ErrNotHolder) {
		t.Fatalf("被别的 agent 持有应该得到 ErrNotHolder，实际 %v", err)
	}

	e.hold(t, a.ID, testAgent.Name)
	if _, err := e.core.MoveCard(ctx, testAgent, a.ID, kanban.MoveCardInput{AfterID: b.ID}); err != nil {
		t.Fatalf("持有者在领活列内调整顺序应该成功: %v", err)
	}
	if e.titles(t, colDoing) != "B,A" {
		t.Fatalf("顺序不对: %s", e.titles(t, colDoing))
	}
	if _, err := e.core.MoveCard(ctx, testAgent, a.ID, review); err != nil {
		t.Fatalf("持有者移到审查列应该成功: %v", err)
	}
	if ev := e.lastEvent(t); ev.Type != kanban.EventCardMoved || ev.ActorKind != "agent" || ev.Actor != "claude" {
		t.Fatalf("事件应该记录 agent: %+v", ev)
	}
	if got := e.stored(t, a.ID); got.ClaimedBy != "" || got.ColumnID != e.col(colReview) {
		t.Fatalf("移动后的状态不对: %+v", got)
	}

	c := e.card(t, colTodo, "C")
	e.hold(t, c.ID, testAgent.Name)
	if _, err := e.core.MoveCard(ctx, testAgent, c.ID, kanban.MoveCardInput{ColumnID: e.col(colDoing)}); !errors.Is(err, kanban.ErrAgentPickupColumn) {
		t.Fatalf("agent 移进领活列应该得到 ErrAgentPickupColumn，实际 %v", err)
	}
	if got := e.stored(t, c.ID); got.ColumnID != e.col(colTodo) || got.ClaimedBy != testAgent.Name {
		t.Fatalf("被拒绝的移动不应该生效: %+v", got)
	}
}

// TestMoveCardRejects 验证拒绝情况：卡片不存在、目标列不存在或属于别的看板、
// 参照卡片不合法、身份无效；失败的移动都不改动卡片。
func TestMoveCardRejects(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	a := e.card(t, colTodo, "A")
	inDoing := e.card(t, colDoing, "进行中的卡")
	foreign := mustCreateBoard(t, e.core, kanban.CreateBoardInput{Name: "别的看板", Slug: "OTH", Locale: "zh-CN"})

	if _, err := e.core.MoveCard(ctx, testActor, 9999, kanban.MoveCardInput{}); !errors.Is(err, kanban.ErrCardNotFound) {
		t.Fatalf("不存在应该得到 ErrCardNotFound，实际 %v", err)
	}
	if _, err := e.core.MoveCard(ctx, kanban.Actor{}, a.ID, kanban.MoveCardInput{}); !errors.Is(err, kanban.ErrInvalidActor) {
		t.Fatalf("身份无效应该得到 ErrInvalidActor，实际 %v", err)
	}
	for name, in := range map[string]kanban.MoveCardInput{
		"别的看板的列":    {ColumnID: foreign.Columns[0].ID},
		"不存在的列":     {ColumnID: 9999},
		"前后参照同时给出":  {BeforeID: inDoing.ID, AfterID: inDoing.ID},
		"参照卡片在别的列":  {ColumnID: e.col(colTodo), BeforeID: inDoing.ID},
		"参照卡片是自己":   {BeforeID: a.ID},
		"参照卡片不存在":   {BeforeID: 9999},
		"换列后参照仍在原列": {ColumnID: e.col(colReview), AfterID: a.ID},
	} {
		if _, err := e.core.MoveCard(ctx, testActor, a.ID, in); !errors.Is(err, kanban.ErrInvalidInput) {
			t.Errorf("%s：应该得到 ErrInvalidInput，实际 %v", name, err)
		}
	}
	if got := e.stored(t, a.ID); got.ColumnID != e.col(colTodo) || got.Version != 1 || got.Position != 1024 {
		t.Fatalf("失败的移动不应该生效: %+v", got)
	}
}

// TestMoveCardRenumbersTightColumn 验证位置间隙太小时先整列重排：A 和 B 的位置几乎重合，
// 把 C 插到它们中间时，A 和 B 被重写成整数倍间距但版本不变，C 落在两者之间。
func TestMoveCardRenumbersTightColumn(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	a, b := e.card(t, colTodo, "A"), e.card(t, colTodo, "B")
	c := e.card(t, colIdea, "C")
	if err := e.db.Card().SetPosition(ctx, a.ID, 1); err != nil {
		t.Fatalf("准备数据失败: %v", err)
	}
	if err := e.db.Card().SetPosition(ctx, b.ID, 1.0000001); err != nil {
		t.Fatalf("准备数据失败: %v", err)
	}

	if _, err := e.core.MoveCard(ctx, testActor, c.ID, kanban.MoveCardInput{ColumnID: e.col(colTodo), AfterID: a.ID}); err != nil {
		t.Fatalf("移动失败: %v", err)
	}
	if e.titles(t, colTodo) != "A,C,B" {
		t.Fatalf("顺序不对: %s", e.titles(t, colTodo))
	}
	ga, gb, gc := e.stored(t, a.ID), e.stored(t, b.ID), e.stored(t, c.ID)
	if ga.Position != 1024 || gb.Position != 2048 || gc.Position != 1536 {
		t.Fatalf("位置应该是 1024、2048、1536，实际 %v、%v、%v", ga.Position, gb.Position, gc.Position)
	}
	if ga.Version != 1 || gb.Version != 1 {
		t.Fatalf("整列重排不应该涨版本: A=%d B=%d", ga.Version, gb.Version)
	}
}
