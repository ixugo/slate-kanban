package kanban_test

import (
	"context"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// TestMoveCardTop 验证 MoveCardInput.Top：换列且没指定位置时放到目标列最顶部，
// 指定了 BeforeID 或 AfterID 时以指定为准，不指定也不要求顶部时仍然放末尾。
func TestMoveCardTop(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		in   func(e *cardEnv, ids map[string]int64) kanban.MoveCardInput
		want string
	}{
		{"顶部", func(e *cardEnv, _ map[string]int64) kanban.MoveCardInput {
			return kanban.MoveCardInput{ColumnID: e.col(colDoing), Top: true}
		}, "M,X,Y"},
		{"顶部但指定了后面", func(e *cardEnv, ids map[string]int64) kanban.MoveCardInput {
			return kanban.MoveCardInput{ColumnID: e.col(colDoing), Top: true, AfterID: ids["X"]}
		}, "X,M,Y"},
		{"顶部但指定了前面", func(e *cardEnv, ids map[string]int64) kanban.MoveCardInput {
			return kanban.MoveCardInput{ColumnID: e.col(colDoing), Top: true, BeforeID: ids["Y"]}
		}, "X,M,Y"},
		{"不要求顶部放末尾", func(e *cardEnv, _ map[string]int64) kanban.MoveCardInput {
			return kanban.MoveCardInput{ColumnID: e.col(colDoing)}
		}, "X,Y,M"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newCardEnv(t)
			ids := map[string]int64{"X": e.card(t, colDoing, "X").ID, "Y": e.card(t, colDoing, "Y").ID}
			m := e.card(t, colTodo, "M")
			if _, err := e.core.MoveCard(ctx, testActor, m.ID, tc.in(e, ids)); err != nil {
				t.Fatalf("移动失败: %v", err)
			}
			if got := e.titles(t, colDoing); got != tc.want {
				t.Fatalf("顺序不对: 期望 %s，实际 %s", tc.want, got)
			}
		})
	}
}

// TestMoveCardTopEmptyAndSameColumn 验证目标列是空列时放到顶部不出错；
// 同一列里要求顶部，会把任务调到第一位，已经在第一位时什么都不做（不写事件）。
func TestMoveCardTopEmptyAndSameColumn(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	a := e.card(t, colTodo, "A")
	if _, err := e.core.MoveCard(ctx, testActor, a.ID, kanban.MoveCardInput{ColumnID: e.col(colDoing), Top: true}); err != nil {
		t.Fatalf("移进空列失败: %v", err)
	}
	e.card(t, colDoing, "B")
	c := e.card(t, colDoing, "C")
	if e.titles(t, colDoing) != "A,B,C" {
		t.Fatalf("初始顺序不对: %s", e.titles(t, colDoing))
	}
	if _, err := e.core.MoveCard(ctx, testActor, c.ID, kanban.MoveCardInput{Top: true}); err != nil {
		t.Fatalf("同列调到顶部失败: %v", err)
	}
	if e.titles(t, colDoing) != "C,A,B" {
		t.Fatalf("同列顶部后的顺序不对: %s", e.titles(t, colDoing))
	}
	events := len(e.events(t))
	if _, err := e.core.MoveCard(ctx, testActor, c.ID, kanban.MoveCardInput{Top: true}); err != nil {
		t.Fatalf("已在顶部再移动失败: %v", err)
	}
	if len(e.events(t)) != events {
		t.Fatalf("已经在顶部不该写事件: %v", e.eventTypes(t, events))
	}
}

// TestFinishCardPlacement 验证交接的落点：默认放目标列末尾，Top 放最顶部，指定参照任务以指定为准。
func TestFinishCardPlacement(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		in   func(ids map[string]int64) kanban.FinishCardInput
		want string
	}{
		{"默认末尾", func(map[string]int64) kanban.FinishCardInput { return kanban.FinishCardInput{Summary: "做完了"} }, "X,Y,M"},
		{"顶部", func(map[string]int64) kanban.FinishCardInput {
			return kanban.FinishCardInput{Summary: "做完了", Top: true}
		}, "M,X,Y"},
		{"指定在 X 后面", func(ids map[string]int64) kanban.FinishCardInput {
			return kanban.FinishCardInput{Summary: "做完了", Top: true, AfterID: ids["X"]}
		}, "X,M,Y"},
		{"指定在 Y 前面", func(ids map[string]int64) kanban.FinishCardInput {
			return kanban.FinishCardInput{Summary: "做完了", BeforeID: ids["Y"]}
		}, "X,M,Y"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newClaimEnv(t)
			ids := map[string]int64{"X": e.card(t, colReview, "X").ID, "Y": e.card(t, colReview, "Y").ID}
			m := e.card(t, colDoing, "M")
			if _, err := e.core.ClaimCard(ctx, testAgent, m.ID, kanban.ClaimCardInput{}); err != nil {
				t.Fatalf("领取失败: %v", err)
			}
			if _, err := e.core.FinishCard(ctx, testAgent, m.ID, tc.in(ids)); err != nil {
				t.Fatalf("交接失败: %v", err)
			}
			if got := e.titles(t, colReview); got != tc.want {
				t.Fatalf("顺序不对: 期望 %s，实际 %s", tc.want, got)
			}
		})
	}
}

// TestCreateCardTop 验证创建任务：默认放列尾，Top 放到所在列最顶部，空列也能用。
func TestCreateCardTop(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	create := func(title string, top bool) {
		t.Helper()
		if _, err := e.core.CreateCard(ctx, testActor, e.board.ID, kanban.CreateCardInput{Title: title, ColumnID: e.col(colTodo), Top: top}); err != nil {
			t.Fatalf("创建 %s 失败: %v", title, err)
		}
	}
	create("A", true)
	create("B", false)
	create("C", true)
	if got := e.titles(t, colTodo); got != "C,A,B" {
		t.Fatalf("顺序不对: 期望 C,A,B，实际 %s", got)
	}
}
