package kanban_test

import (
	"context"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// TestPlaceCardRenumbersWhenGapTooSmall 验证相邻位置太近时，整列重排成 1024 的整数倍并写进数据库，
// 再返回两张卡中间的位置；重排不涨版本号。
func TestPlaceCardRenumbersWhenGapTooSmall(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	store := e.db.Card()
	var cards []*kanban.Card
	for i, pos := range []float64{1, 1 + 1e-7, 5} {
		c := &kanban.Card{BoardID: e.board.ID, ColumnID: 1, Key: string(rune('A'+i)) + "-1", Title: "t", Position: pos, Version: 1}
		if err := store.Create(ctx, c); err != nil {
			t.Fatalf("创建任务失败: %v", err)
		}
		cards = append(cards, c)
	}

	var got float64
	err := e.core.RunForTest(ctx, e.board.ID, func(u *kanban.Unit) error {
		var err error
		got, err = u.PlaceCardForTest(ctx, cards, 0, cards[0].ID) // 放到第一张后面，也就是前两张中间
		return err
	})
	if err != nil {
		t.Fatalf("计算位置失败: %v", err)
	}
	if got != 1536 {
		t.Fatalf("重排后应该落在 1024 和 2048 的中点 1536，实际 %v", got)
	}
	saved, _ := store.ListByColumn(ctx, 1)
	for i, want := range []float64{1024, 2048, 3072} {
		if saved[i].Position != want || saved[i].Version != 1 {
			t.Errorf("第 %d 张应该是位置 %v、版本 1，实际 %+v", i, want, saved[i])
		}
	}
}

// TestPlaceCardWithoutRenumber 验证间隙够用时直接取中点，不改动任何任务的位置。
func TestPlaceCardWithoutRenumber(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	cards := []*kanban.Card{{ID: 1, Position: 1024}, {ID: 2, Position: 2048}}
	var got float64
	err := e.core.RunForTest(ctx, e.board.ID, func(u *kanban.Unit) error {
		var err error
		got, err = u.PlaceCardForTest(ctx, cards, 2, 0)
		return err
	})
	if err != nil || got != 1536 {
		t.Fatalf("应该直接得到 1536，实际 %v, %v", got, err)
	}
	if cards[0].Position != 1024 || cards[1].Position != 2048 {
		t.Fatalf("不应该改动任务位置: %+v %+v", cards[0], cards[1])
	}
}
