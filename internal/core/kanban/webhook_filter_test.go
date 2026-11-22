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

// TestWebhookExplicitSubscriptions 验证新配置的空选择不会回退成全选。
func TestWebhookExplicitSubscriptions(t *testing.T) {
	t.Parallel()
	moved := &kanban.Event{Type: kanban.EventCardMoved, ActorKind: "agent", Payload: `{"to_id":5}`}
	created := &kanban.Event{Type: kanban.EventCardCreated, ActorKind: "agent"}
	hook := &kanban.Webhook{Enabled: true, StrictFilters: true, ActorKinds: []string{"agent"}, MoveColumnIDs: []int64{5}}
	if kanban.WebhookMatchesEventForTest(hook, moved) {
		t.Fatal("无事件选择不应发送")
	}
	hook.EventTypes = []string{kanban.EventCardMoved, kanban.EventCardCreated}
	hook.ActorKinds = nil
	if kanban.WebhookMatchesEventForTest(hook, moved) {
		t.Fatal("无操作者选择不应发送")
	}
	hook.ActorKinds = []string{"agent"}
	hook.MoveColumnIDs = nil
	if kanban.WebhookMatchesEventForTest(hook, moved) || !kanban.WebhookMatchesEventForTest(hook, created) {
		t.Fatal("无目标列时只排除移动事件")
	}
	hook.MoveColumnIDs = []int64{5}
	if !kanban.WebhookMatchesEventForTest(hook, moved) {
		t.Fatal("三个条件都满足时应发送")
	}
}
