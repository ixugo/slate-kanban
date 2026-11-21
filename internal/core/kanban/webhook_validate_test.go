package kanban_test

import (
	"strings"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// TestValidateWebhookURL 验证各类型 webhook 的 URL 协议、主机白名单与长度上限。
func TestValidateWebhookURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		typ, raw string
		ok       bool
	}{
		{kanban.WebhookTypeDingTalk, "https://oapi.dingtalk.com/robot/send?access_token=abc", true},
		{kanban.WebhookTypeDingTalk, "http://oapi.dingtalk.com/robot/send", false},
		{kanban.WebhookTypeDingTalk, "https://evil.com/robot/send", false},
		{kanban.WebhookTypeWeCom, "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx", true},
		{kanban.WebhookTypeWeCom, "https://open.feishu.cn/open-apis/bot/v2/hook/xxx", false},
		{kanban.WebhookTypeFeishu, "https://open.feishu.cn/open-apis/bot/v2/hook/xxx", true},
		{kanban.WebhookTypeFeishu, "https://open.larksuite.com/open-apis/bot/v2/hook/xxx", true},
		{kanban.WebhookTypeFeishu, "https://oapi.dingtalk.com/x", false},
		{kanban.WebhookTypeDingTalk, "ftp://oapi.dingtalk.com/x", false},
		{kanban.WebhookTypeDingTalk, "not-a-url", false},
	}
	for _, c := range cases {
		err := kanban.ValidateWebhookURLForTest(c.typ, c.raw)
		if c.ok && err != nil {
			t.Fatalf("%s %q 应该通过，实际 %v", c.typ, c.raw, err)
		}
		if !c.ok && err == nil {
			t.Fatalf("%s %q 应该拒绝", c.typ, c.raw)
		}
	}
	long := "https://oapi.dingtalk.com/" + strings.Repeat("a", kanban.MaxWebhookURLLen)
	if err := kanban.ValidateWebhookURLForTest(kanban.WebhookTypeDingTalk, long); err == nil {
		t.Fatal("超长 URL 应该拒绝")
	}
}

// TestNormalizeWebhookSubscriptions 验证事件类型、操作者类型与目标列订阅的白名单。
func TestNormalizeWebhookSubscriptions(t *testing.T) {
	t.Parallel()
	cols := []*kanban.Column{{ID: 10}, {ID: 20}}
	w := &kanban.Webhook{
		EventTypes:    []string{kanban.EventCardMoved, kanban.EventCardCreated},
		ActorKinds:    []string{"human", "agent"},
		MoveColumnIDs: []int64{20, 10},
	}
	if err := kanban.NormalizeWebhookSubscriptionsForTest(w, cols); err != nil {
		t.Fatalf("合法订阅应该通过: %v", err)
	}
	if len(w.EventTypes) != 2 || w.EventTypes[0] != kanban.EventCardMoved {
		t.Fatalf("事件类型顺序应保留去重: %v", w.EventTypes)
	}
	bad := &kanban.Webhook{EventTypes: []string{"card.hacked"}}
	if err := kanban.NormalizeWebhookSubscriptionsForTest(bad, cols); err == nil {
		t.Fatal("未知事件类型应拒绝")
	}
	bad = &kanban.Webhook{ActorKinds: []string{"bot"}}
	if err := kanban.NormalizeWebhookSubscriptionsForTest(bad, cols); err == nil {
		t.Fatal("未知操作者类型应拒绝")
	}
	bad = &kanban.Webhook{MoveColumnIDs: []int64{99}}
	if err := kanban.NormalizeWebhookSubscriptionsForTest(bad, cols); err == nil {
		t.Fatal("不属于看板的列应拒绝")
	}
}
