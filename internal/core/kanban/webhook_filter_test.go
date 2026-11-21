package kanban_test

import (
	"testing"

	"slate-kanban/internal/core/kanban"
)

// TestWebhookMatchesEvent 验证订阅过滤逻辑。
func TestWebhookMatchesEvent(t *testing.T) {
	t.Parallel()
	ev := &kanban.Event{Type: kanban.EventCardMoved, ActorKind: "human", Payload: `{"to_id":5}`}
	all := &kanban.Webhook{Enabled: true}
	if !kanban.WebhookMatchesEventForTest(all, ev) {
		t.Fatal("默认应匹配")
	}
	off := &kanban.Webhook{Enabled: false}
	if kanban.WebhookMatchesEventForTest(off, ev) {
		t.Fatal("停用不应匹配")
	}
	typ := &kanban.Webhook{Enabled: true, EventTypes: []string{kanban.EventCardCreated}}
	if kanban.WebhookMatchesEventForTest(typ, ev) {
		t.Fatal("事件类型不匹配")
	}
	actor := &kanban.Webhook{Enabled: true, ActorKinds: []string{"agent"}}
	if kanban.WebhookMatchesEventForTest(actor, ev) {
		t.Fatal("操作者不匹配")
	}
	col := &kanban.Webhook{Enabled: true, MoveColumnIDs: []int64{9}}
	if kanban.WebhookMatchesEventForTest(col, ev) {
		t.Fatal("目标列不匹配")
	}
	colOK := &kanban.Webhook{Enabled: true, MoveColumnIDs: []int64{5}}
	if !kanban.WebhookMatchesEventForTest(colOK, ev) {
		t.Fatal("目标列匹配应通过")
	}
}
