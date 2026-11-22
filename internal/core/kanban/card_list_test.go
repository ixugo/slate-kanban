package kanban_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// listTitles 返回列表里任务标题，用逗号连接。
func listTitles(l *kanban.CardList) string {
	names := make([]string, 0, len(l.Cards))
	for _, c := range l.Cards {
		names = append(names, c.Title)
	}
	return strings.Join(names, ",")
}

// listEnv 建一个有四个任务的看板：待办 A、D（D 在等 A），进行中 B（被 claude 领取），完成 C。
func listEnv(t *testing.T) *cardEnv {
	t.Helper()
	ctx := context.Background()
	e := newCardEnv(t)
	a := e.card(t, colTodo, "A")
	d := e.card(t, colTodo, "D")
	b := e.card(t, colDoing, "B")
	e.card(t, colDone, "C")
	deps := []int64{a.ID}
	if _, err := e.core.UpdateCard(ctx, testActor, d.ID, kanban.UpdateCardInput{WaitsOn: &deps}); err != nil {
		t.Fatalf("设置依赖失败: %v", err)
	}
	e.hold(t, b.ID, testAgent.Name)
	return e
}

// TestListCards 验证列表：按看板显示顺序（列从左到右、列内按位置）排列，
// 每张卡带着列名、是否完成、是否在等待、领取是否有效；筛选条件生效。
func TestListCards(t *testing.T) {
	ctx := context.Background()
	e := listEnv(t)

	all, err := e.core.ListCards(ctx, e.board.ID, kanban.ListCardsInput{})
	if err != nil || listTitles(all) != "A,D,B,C" || all.Total != 4 || all.Truncated {
		t.Fatalf("全部任务不对: %s total=%d truncated=%v err=%v", listTitles(all), all.Total, all.Truncated, err)
	}
	byTitle := map[string]*kanban.CardView{}
	for _, v := range all.Cards {
		byTitle[v.Title] = v
	}
	if v := byTitle["A"]; v.Column != "待办" || v.Done || v.Waiting || v.ClaimActive {
		t.Errorf("A 的视图不对: %+v", v)
	}
	if v := byTitle["D"]; !v.Waiting || v.Key != "TST-2" {
		t.Errorf("D 应该在等待: %+v", v)
	}
	if v := byTitle["B"]; v.Column != "进行中" || !v.ClaimActive || v.ClaimedBy != "claude" {
		t.Errorf("B 应该被 claude 领取: %+v", v)
	}
	if v := byTitle["C"]; !v.Done || v.Column != "完成" {
		t.Errorf("C 应该在完成列: %+v", v)
	}

	for q, want := range map[string]string{
		"is:waiting":     "D",
		"col:进行中":        "B",
		"is:open -A":     "D,B",
		"claimed:claude": "B",
		"zzz":            "",
	} {
		got, err := e.core.ListCards(ctx, e.board.ID, kanban.ListCardsInput{Query: q})
		if err != nil || listTitles(got) != want {
			t.Errorf("筛选 %q：期望 %q，实际 %q，错误 %v", q, want, listTitles(got), err)
		}
	}
}

// TestListCardsLimit 验证数量限制：只返回前几张，但 Total 是命中的总数，Truncated 提示还有更多；
// 刚好等于命中数时不算截断。
func TestListCardsLimit(t *testing.T) {
	ctx := context.Background()
	e := listEnv(t)

	got, err := e.core.ListCards(ctx, e.board.ID, kanban.ListCardsInput{Limit: 2})
	if err != nil || listTitles(got) != "A,D" || got.Total != 4 || !got.Truncated {
		t.Fatalf("限制 2 张: %s total=%d truncated=%v err=%v", listTitles(got), got.Total, got.Truncated, err)
	}
	got, err = e.core.ListCards(ctx, e.board.ID, kanban.ListCardsInput{Limit: 4})
	if err != nil || len(got.Cards) != 4 || got.Truncated {
		t.Fatalf("限制 4 张刚好够: %d truncated=%v err=%v", len(got.Cards), got.Truncated, err)
	}
	got, err = e.core.ListCards(ctx, e.board.ID, kanban.ListCardsInput{Query: "is:open", Limit: 1})
	if err != nil || got.Total != 3 || len(got.Cards) != 1 || !got.Truncated {
		t.Fatalf("总数应该按命中的算: %+v %v", got, err)
	}
}

// TestListCardsEmptyBoard 验证空看板：返回空数组而不是 null，JSON 里前端不用判空。
func TestListCardsEmptyBoard(t *testing.T) {
	e := newCardEnv(t)
	got, err := e.core.ListCards(context.Background(), e.board.ID, kanban.ListCardsInput{})
	if err != nil || got.Cards == nil || len(got.Cards) != 0 || got.Total != 0 || got.Truncated {
		t.Fatalf("空看板应该返回空列表: %+v %v", got, err)
	}
}

// TestListCardsRejects 验证拒绝情况：看板不存在、数量为负、筛选条件不合法，都不写事件。
func TestListCardsRejects(t *testing.T) {
	ctx := context.Background()
	e := listEnv(t)
	events := len(e.events(t))

	if _, err := e.core.ListCards(ctx, 9999, kanban.ListCardsInput{}); !errors.Is(err, kanban.ErrBoardNotFound) {
		t.Errorf("看板不存在应该得到 ErrBoardNotFound，实际 %v", err)
	}
	if _, err := e.core.ListCards(ctx, e.board.ID, kanban.ListCardsInput{Limit: -1}); !errors.Is(err, kanban.ErrInvalidInput) {
		t.Errorf("负数应该得到 ErrInvalidInput，实际 %v", err)
	}
	if _, err := e.core.ListCards(ctx, e.board.ID, kanban.ListCardsInput{Query: "is:foo"}); !errors.Is(err, kanban.ErrInvalidInput) {
		t.Errorf("筛选不合法应该得到 ErrInvalidInput，实际 %v", err)
	}
	if after := len(e.events(t)); after != events {
		t.Errorf("读取不应该写事件，事件数 %d → %d", events, after)
	}
}
