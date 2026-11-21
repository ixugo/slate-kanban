package kanban_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// TestUnreadCountEmitIncrements 验证带 card_id 的事件会递增未读数，创建和删除不计数。
func TestUnreadCountEmitIncrements(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	c := e.card(t, colTodo, "未读测试")
	if got := e.stored(t, c.ID).UnreadCount; got != 0 {
		t.Fatalf("新建后未读应为 0: %d", got)
	}

	if _, err := e.core.AddComment(ctx, testActor, c.ID, "人的评论"); err != nil {
		t.Fatalf("人写评论失败: %v", err)
	}
	if _, err := e.core.AddComment(ctx, testAgent, c.ID, "agent 评论"); err != nil {
		t.Fatalf("agent 写评论失败: %v", err)
	}
	err := e.core.RunForTest(ctx, e.board.ID, func(u *kanban.Unit) error {
		return u.EmitForTest(ctx, kanban.AutomationActor("计时"), kanban.NewEvent{
			BoardID: e.board.ID, CardID: c.ID, Type: kanban.EventCardUpdated,
		})
	})
	if err != nil {
		t.Fatalf("系统事件失败: %v", err)
	}
	if got := e.stored(t, c.ID).UnreadCount; got != 3 {
		t.Fatalf("三次事件应未读 +3: %d", got)
	}

	deleted := e.card(t, colTodo, "待删")
	before := e.stored(t, deleted.ID).UnreadCount
	if err := e.core.DeleteCard(ctx, testActor, deleted.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if before != 0 {
		t.Fatalf("删除前未读应为 0: %d", before)
	}
}

// TestUnreadCountEmitSkipsNoCard 验证与任务无关的事件不递增未读。
func TestUnreadCountEmitSkipsNoCard(t *testing.T) {
	e := newCardEnv(t)
	c := e.card(t, colTodo, "板级事件")
	before := e.stored(t, c.ID).UnreadCount
	e.emitRaw(t, testActor, kanban.EventBoardUpdated)
	if got := e.stored(t, c.ID).UnreadCount; got != before {
		t.Fatalf("看板事件不应改未读: %d -> %d", before, got)
	}
}

// TestMarkCardReadIdempotent 验证清零幂等，且不改 version 与 updated_at。
func TestMarkCardReadIdempotent(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	c := e.card(t, colTodo, "已读测试")
	if _, err := e.core.AddComment(ctx, testActor, c.ID, "hi"); err != nil {
		t.Fatal(err)
	}
	stored := e.stored(t, c.ID)
	if stored.UnreadCount != 1 {
		t.Fatalf("应有 1 条未读: %d", stored.UnreadCount)
	}
	ver, updated := stored.Version, stored.UpdatedAt

	for range 2 {
		if err := e.core.MarkCardRead(ctx, c.ID); err != nil {
			t.Fatalf("MarkCardRead: %v", err)
		}
	}
	after := e.stored(t, c.ID)
	if after.UnreadCount != 0 || after.Version != ver || !after.UpdatedAt.Equal(updated) {
		t.Fatalf("清零后: unread=%d version=%d updated 变了=%v", after.UnreadCount, after.Version, !after.UpdatedAt.Equal(updated))
	}
}

// TestExportOmitsUnreadCount 验证导出 JSON 不含 unread_count。
func TestExportOmitsUnreadCount(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	c := e.card(t, colTodo, "导出未读")
	if _, err := e.core.AddComment(ctx, testActor, c.ID, "x"); err != nil {
		t.Fatal(err)
	}
	exp, err := e.core.ExportBoard(ctx, testActor, e.board.ID)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(exp.Cards[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "unread_count") {
		t.Fatalf("导出不应带 unread_count: %s", raw)
	}
}
