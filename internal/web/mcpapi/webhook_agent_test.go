package mcpapi_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// TestWebhookPushesAgentToolActivity 验证只订阅 agent 的 webhook：
// agent 通过 MCP 领取、评论、交接（finish_card）产生的活动都会推送，人的操作不推送。
func TestWebhookPushesAgentToolActivity(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "HOOK")
	var mu sync.Mutex
	var sent []string
	restoreDo := kanban.SetWebhookHTTPDoForTest(func(_ *http.Client, req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		mu.Lock()
		sent = append(sent, string(body))
		mu.Unlock()
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"errcode":0}`))}, nil
	})
	defer restoreDo()
	restoreHook := kanban.SetWebhookNotifyHookForTest(func(c kanban.Core, ctx context.Context, events []*kanban.Event) {
		c.DeliverWebhooksForTest(ctx, events)
	})
	defer restoreHook()

	if _, err := s.core.CreateWebhook(t.Context(), humanActor, b.ID, kanban.CreateWebhookInput{
		Name: "只看 agent", Type: kanban.WebhookTypeWeCom,
		URL:        "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=test",
		ActorKinds: []string{"agent"}, MoveColumnIDs: []int64{b.doing, b.review, b.done},
	}); err != nil {
		t.Fatalf("创建 webhook 失败: %v", err)
	}
	card := s.card(t, b, b.doing, "写文档")
	sess := s.agent(t, "claude")

	ok(t, sess, "claim_card", map[string]any{"card_id": card.ID})
	ok(t, sess, "comment_card", map[string]any{"card_id": card.ID, "body": "开始干活"})
	ok(t, sess, "finish_card", map[string]any{"card_id": card.ID, "summary": "做完了"})

	mu.Lock()
	defer mu.Unlock()
	all := strings.Join(sent, "\n")
	for _, want := range []string{"领取了任务", "添加了评论：开始干活", "添加了评论：做完了", "移到"} {
		if !strings.Contains(all, want) {
			t.Errorf("应该推送「%s」，实际收到 %d 条: %s", want, len(sent), all)
		}
	}
}
