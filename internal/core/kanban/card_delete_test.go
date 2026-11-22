package kanban_test

import (
	"context"
	"errors"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// TestDeleteCard 验证删除：任务和它的评论都没了，写 card.deleted 事件；
// 别的任务不受影响；编号不会回收，下一个新任务继续往后编。
func TestDeleteCard(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	a, b := e.card(t, colTodo, "A"), e.card(t, colTodo, "B")
	if _, err := e.core.AddComment(ctx, testActor, b.ID, "一条评论"); err != nil {
		t.Fatalf("准备评论失败: %v", err)
	}

	if err := e.core.DeleteCard(ctx, testActor, b.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if _, err := e.db.Card().GetByID(ctx, b.ID); err == nil {
		t.Fatal("任务应该已经被删除")
	}
	if comments, err := e.db.Comment().ListByCard(ctx, b.ID); err != nil || len(comments) != 0 {
		t.Fatalf("评论应该一起删除: %v %v", comments, err)
	}
	if got := e.stored(t, a.ID); got.Version != 1 {
		t.Fatalf("别的任务不应该受影响: %+v", got)
	}
	want := `{"key":"TST-2","title":"B"}`
	if ev := e.lastEvent(t); ev.Type != kanban.EventCardDeleted || ev.CardID != b.ID || ev.Payload != want {
		t.Fatalf("事件不对: %+v，期望内容 %s", ev, want)
	}
	if next := e.card(t, colTodo, "C"); next.Key != "TST-3" {
		t.Fatalf("编号不应该回收，期望 TST-3，实际 %s", next.Key)
	}
}

// TestDeleteCardRemovesDependency 验证删除被依赖的任务：其他任务依赖里的它被移除且版本加一，
// 没有依赖它的任务不受影响，也不会为这些连带清理另写事件。
func TestDeleteCardRemovesDependency(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	a, b, c, d := e.card(t, colTodo, "A"), e.card(t, colTodo, "B"), e.card(t, colTodo, "C"), e.card(t, colTodo, "D")
	setDeps := func(id int64, deps ...int64) {
		t.Helper()
		if _, err := e.core.UpdateCard(ctx, testActor, id, kanban.UpdateCardInput{WaitsOn: &deps}); err != nil {
			t.Fatalf("设置依赖失败: %v", err)
		}
	}
	setDeps(a.ID, b.ID)       // A 等 B
	setDeps(c.ID, b.ID, a.ID) // C 等 B 和 A
	events := len(e.events(t))

	if err := e.core.DeleteCard(ctx, testActor, b.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if got := e.stored(t, a.ID); len(got.WaitsOn) != 0 || got.Version != 3 {
		t.Fatalf("A 的依赖应该清空且版本加一: %+v", got)
	}
	if got := e.stored(t, c.ID); len(got.WaitsOn) != 1 || got.WaitsOn[0] != a.ID || got.Version != 3 {
		t.Fatalf("C 应该只剩依赖 A: %+v", got)
	}
	if got := e.stored(t, d.ID); got.Version != 1 {
		t.Fatalf("D 不应该受影响: %+v", got)
	}
	if after := len(e.events(t)); after != events+1 {
		t.Fatalf("只应该多一条 card.deleted 事件，事件数 %d → %d", events, after)
	}
}

// TestDeleteCardIsIdempotent 验证幂等：重复删除、删除不存在的任务都返回成功，并且不写事件。
func TestDeleteCardIsIdempotent(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	a := e.card(t, colTodo, "A")
	if err := e.core.DeleteCard(ctx, testActor, a.ID); err != nil {
		t.Fatalf("第一次删除失败: %v", err)
	}
	events := len(e.events(t))

	for _, id := range []int64{a.ID, 9999} {
		if err := e.core.DeleteCard(ctx, testActor, id); err != nil {
			t.Fatalf("删除 %d 应该成功: %v", id, err)
		}
	}
	if after := len(e.events(t)); after != events {
		t.Fatalf("幂等的删除不应该写事件，事件数 %d → %d", events, after)
	}
}

// TestDeleteCardHumanOnly 验证权限：agent、自动化和身份无效的调用都被拒绝，任务原样保留。
func TestDeleteCardHumanOnly(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	a := e.card(t, colDoing, "A")
	e.hold(t, a.ID, testAgent.Name)

	for name, tc := range map[string]struct {
		actor kanban.Actor
		want  error
	}{
		"agent":  {testAgent, kanban.ErrHumanOnly},
		"自动化":    {kanban.Actor{Kind: kanban.ActorSystem, Name: "automation"}, kanban.ErrHumanOnly},
		"身份无效":   {kanban.Actor{}, kanban.ErrInvalidActor},
		"没有名字的人": {kanban.Actor{Kind: kanban.ActorHuman}, kanban.ErrInvalidActor},
	} {
		if err := e.core.DeleteCard(ctx, tc.actor, a.ID); !errors.Is(err, tc.want) {
			t.Errorf("%s：期望 %v，实际 %v", name, tc.want, err)
		}
	}
	if got := e.stored(t, a.ID); got.Version != 1 {
		t.Fatalf("被拒绝的删除不应该改动任务: %+v", got)
	}
}
