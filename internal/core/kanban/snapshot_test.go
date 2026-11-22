package kanban_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// TestBoardSnapshot 验证整板快照：任务按列在看板上的顺序、同列按位置排列，而不是按列主键或创建顺序。
// 这里把"完成"列挪到最左边，让列的显示顺序和列主键的顺序不一致，才能验证到排序。
// 每个任务带着列名和是否完成，序号等于读取时看板的最新事件序号。
func TestBoardSnapshot(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	a := e.card(t, colTodo, "A")
	b := e.card(t, colIdea, "B")
	c := e.card(t, colTodo, "C")
	d := e.card(t, colDone, "D")
	if _, err := e.core.MoveCard(ctx, testActor, a.ID, kanban.MoveCardInput{AfterID: c.ID}); err != nil {
		t.Fatalf("调整任务顺序失败: %v", err)
	}
	order := []int64{e.col(colDone), e.col(colIdea), e.col(colTodo), e.col(colDoing), e.col(colReview)}
	if _, err := e.core.ReorderColumns(ctx, testActor, e.board.ID, order); err != nil {
		t.Fatalf("调整列顺序失败: %v", err)
	}

	snap, err := e.core.BoardSnapshot(ctx, e.board.ID)
	if err != nil {
		t.Fatalf("读取快照失败: %v", err)
	}
	var got []int64
	for _, v := range snap.Cards {
		got = append(got, v.ID)
	}
	if want := []int64{d.ID, b.ID, c.ID, a.ID}; !slices.Equal(got, want) {
		t.Fatalf("任务顺序不对: %v，期望 %v", got, want)
	}
	if snap.Seq != e.lastEvent(t).ID {
		t.Fatalf("序号应该等于最新事件序号: %d，期望 %d", snap.Seq, e.lastEvent(t).ID)
	}
	if snap.ID != e.board.ID || len(snap.Columns) != 5 || snap.Columns[0].ID != e.col(colDone) || snap.Rules == nil {
		t.Fatalf("看板、列、规则应该一起带上，列是调整后的顺序: %+v", snap.BoardDetail)
	}
	first, last := snap.Cards[0], snap.Cards[3]
	if first.Column != e.board.Columns[colDone].Name || !first.Done || last.Column != e.board.Columns[colTodo].Name || last.Done {
		t.Fatalf("列名和完成标记不对: %+v %+v", first, last)
	}
}

// TestBoardSnapshotEmptyBoard 验证空看板：任务是空数组而不是 null，序号是建看板那条事件。
func TestBoardSnapshotEmptyBoard(t *testing.T) {
	e := newCardEnv(t)
	snap, err := e.core.BoardSnapshot(context.Background(), e.board.ID)
	if err != nil || snap.Cards == nil || len(snap.Cards) != 0 || snap.Seq != e.lastEvent(t).ID {
		t.Fatalf("空看板应该返回空数组: %+v %v", snap, err)
	}
}

// TestBoardSnapshotIncludesRules 验证起步模板带的自动化规则也在快照里。
func TestBoardSnapshotIncludesRules(t *testing.T) {
	core, _ := newBare(t)
	board := mustCreateBoard(t, core, kanban.CreateBoardInput{Name: "有规则", Slug: "RUL", Locale: "zh-CN"})
	snap, err := core.BoardSnapshot(context.Background(), board.ID)
	if err != nil || len(snap.Rules) == 0 || len(snap.Rules) != len(board.Rules) {
		t.Fatalf("快照应该带上起步规则: %+v %v", snap, err)
	}
}

// TestBoardSnapshotMissingBoard 验证看板不存在返回 404。
func TestBoardSnapshotMissingBoard(t *testing.T) {
	e := newCardEnv(t)
	if _, err := e.core.BoardSnapshot(context.Background(), 9999); !errors.Is(err, kanban.ErrBoardNotFound) {
		t.Fatalf("应该返回看板不存在: %v", err)
	}
}
