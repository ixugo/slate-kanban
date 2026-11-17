package kanban_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ixugo/goddd/pkg/orm"

	"slate-kanban/internal/core/kanban"
)

// fillBoard 往看板里放齐每一类数据：卡片、评论、一条规则（再加事件）和一条 agent 的读取进度，
// 返回卡片，用来验证删除时一类都不落下。
func (e *cardEnv) fillBoard(t *testing.T) *kanban.Card {
	t.Helper()
	ctx := context.Background()
	c := e.card(t, colTodo, "A")
	if _, err := e.core.AddComment(ctx, testActor, c.ID, "评论"); err != nil {
		t.Fatalf("写评论失败: %v", err)
	}
	e.enterRule(t, "规则", 0, kanban.RuleAction{Type: kanban.ActionSetPriority, Priority: 1})
	if _, err := e.db.Cursor().Advance(ctx, e.board.ID, "claude", 3); err != nil {
		t.Fatalf("写读取进度失败: %v", err)
	}
	return c
}

// TestDeleteBoardRemovesEverything 验证删除看板把它的全部数据都清掉，别的看板一点不受影响。
func TestDeleteBoardRemovesEverything(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	card := e.fillBoard(t)
	other := newCardEnvOn(t, e.core, e.db, "OTH")
	otherCard := other.fillBoard(t)

	if err := e.core.DeleteBoard(ctx, testActor, e.board.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if _, err := e.core.GetBoard(ctx, e.board.ID); !errors.Is(err, kanban.ErrBoardNotFound) {
		t.Fatalf("看板应该不存在: %v", err)
	}
	if _, err := e.db.Cursor().Get(ctx, e.board.ID, "claude"); !orm.IsErrRecordNotFound(err) {
		t.Fatalf("读取进度应该被删除: %v", err)
	}
	checks := map[string]func() (int, error){
		"列":  func() (int, error) { return countOf(e.db.Column().ListByBoard(ctx, e.board.ID)) },
		"卡片": func() (int, error) { return countOf(e.db.Card().ListByBoard(ctx, e.board.ID)) },
		"规则": func() (int, error) { return countOf(e.db.Rule().ListByBoard(ctx, e.board.ID)) },
		"评论": func() (int, error) { return countOf(e.db.Comment().ListByCard(ctx, card.ID)) },
		"事件": func() (int, error) { return countOf(e.db.Event().ListAfter(ctx, e.board.ID, 0, 1000)) },
	}
	for what, count := range checks {
		if n, err := count(); err != nil || n != 0 {
			t.Errorf("%s 应该被删除干净，实际 %d 条，错误 %v", what, n, err)
		}
	}

	if _, err := other.core.GetBoard(ctx, other.board.ID); err != nil {
		t.Fatalf("别的看板应该还在: %v", err)
	}
	if n, _ := countOf(other.db.Card().ListByBoard(ctx, other.board.ID)); n != 1 {
		t.Fatalf("别的看板的卡片不应该被删: %d", n)
	}
	if n, _ := countOf(other.db.Comment().ListByCard(ctx, otherCard.ID)); n != 1 {
		t.Fatalf("别的看板的评论不应该被删: %d", n)
	}
	if n, _ := countOf(other.db.Rule().ListByBoard(ctx, other.board.ID)); n != 1 {
		t.Fatalf("别的看板的规则不应该被删: %d", n)
	}
	if _, err := other.db.Cursor().Get(ctx, other.board.ID, "claude"); err != nil {
		t.Fatalf("别的看板的读取进度不应该被删: %v", err)
	}
	if n, _ := countOf(other.db.Event().ListAfter(ctx, other.board.ID, 0, 1000)); n == 0 {
		t.Fatalf("别的看板的事件不应该被删")
	}
}

// countOf 把"列表加错误"的返回值换成"条数加错误"，让上面的检查可以放进同一张表。
func countOf[T any](items []T, err error) (int, error) { return len(items), err }

// TestDeleteBoardWakesWaiters 验证删除后会广播一次，正在等待这个看板更新的人被唤醒。
func TestDeleteBoardWakesWaiters(t *testing.T) {
	e := newCardEnv(t)
	signal, cancel := e.core.Subscribe(e.board.ID)
	defer cancel()

	if err := e.core.DeleteBoard(context.Background(), testActor, e.board.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatal("删除看板后等待者应该被唤醒")
	}
}

// TestDeleteBoardIdempotentAndDenied 验证看板不存在时返回成功；agent、自动化、身份无效都不能删，看板保留。
func TestDeleteBoardIdempotentAndDenied(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	for name, tc := range map[string]struct {
		actor kanban.Actor
		want  error
	}{
		"agent": {testAgent, kanban.ErrHumanOnly},
		"自动化":   {kanban.AutomationActor("x"), kanban.ErrHumanOnly},
		"身份无效":  {kanban.Actor{}, kanban.ErrInvalidActor},
	} {
		if err := e.core.DeleteBoard(ctx, tc.actor, e.board.ID); !errors.Is(err, tc.want) {
			t.Errorf("%s：期望 %v，实际 %v", name, tc.want, err)
		}
	}
	if _, err := e.core.GetBoard(ctx, e.board.ID); err != nil {
		t.Fatalf("被拒绝的删除不应该删看板: %v", err)
	}

	if err := e.core.DeleteBoard(ctx, testActor, e.board.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if err := e.core.DeleteBoard(ctx, testActor, e.board.ID); err != nil {
		t.Fatalf("重复删除应该成功: %v", err)
	}
	if err := e.core.DeleteBoard(ctx, testActor, 9999); err != nil {
		t.Fatalf("删除不存在的看板应该成功: %v", err)
	}
}
