package kanban_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"slate-kanban/internal/core/kanban"
)

// TestWebhookNotifyDoesNotBlockCommit 验证异步推送不阻塞主事务，失败也不影响写库。
func TestWebhookNotifyDoesNotBlockCommit(t *testing.T) {
	e := newCardEnv(t)
	var calls atomic.Int32
	block := make(chan struct{})
	restoreDo := kanban.SetWebhookHTTPDoForTest(func(_ *http.Client, req *http.Request) (*http.Response, error) {
		calls.Add(1)
		<-block
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})
	defer restoreDo()

	restoreHook := kanban.SetWebhookNotifyHookForTest(func(c kanban.Core, ctx context.Context, events []*kanban.Event) {
		go c.DeliverWebhooksForTest(ctx, events)
	})
	defer restoreHook()

	_, err := e.core.CreateWebhook(context.Background(), testActor, e.board.ID, kanban.CreateWebhookInput{
		Name: "通知", Type: kanban.WebhookTypeWeCom,
		URL: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=test",
	})
	if err != nil {
		t.Fatalf("创建 webhook: %v", err)
	}

	start := time.Now()
	if _, err := e.core.CreateCard(context.Background(), testActor, e.board.ID, kanban.CreateCardInput{
		Title: "推", ColumnID: e.col(colTodo),
	}); err != nil {
		t.Fatalf("创建卡片: %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("主事务不应被 webhook 阻塞")
	}
	deadline := time.Now().Add(3 * time.Second)
	for calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if calls.Load() == 0 {
		close(block)
		t.Fatal("异步推送应被触发")
	}
	close(block)
}

// TestWebhookAgentActivityDelivered 验证只订阅 agent 操作的 webhook：agent 领取、评论、移列都会推送，
// 人的操作和自动化（系统）事件不会推送。移列只推目标列在订阅范围内的。
func TestWebhookAgentActivityDelivered(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	var mu sync.Mutex
	var texts []string
	restoreDo := kanban.SetWebhookHTTPDoForTest(func(_ *http.Client, req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		mu.Lock()
		texts = append(texts, string(body))
		mu.Unlock()
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"errcode":0}`))}, nil
	})
	defer restoreDo()
	restoreHook := kanban.SetWebhookNotifyHookForTest(func(c kanban.Core, ctx context.Context, events []*kanban.Event) {
		c.DeliverWebhooksForTest(ctx, events)
	})
	defer restoreHook()

	if _, err := e.core.CreateWebhook(ctx, testActor, e.board.ID, kanban.CreateWebhookInput{
		Name: "只看 agent", Type: kanban.WebhookTypeWeCom,
		URL:        "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=test",
		ActorKinds: []string{"agent"}, MoveColumnIDs: []int64{e.col(colDoing), e.col(colReview), e.col(colDone)},
	}); err != nil {
		t.Fatalf("创建 webhook: %v", err)
	}
	c := e.card(t, colTodo, "任务")
	mu.Lock()
	texts = nil // 人创建卡片不应推送，这里清空只为下面断言更直观
	mu.Unlock()

	if _, err := e.core.ClaimCard(ctx, testAgent, c.ID, kanban.ClaimCardInput{}); err != nil {
		t.Fatalf("领取失败: %v", err)
	}
	if _, err := e.core.AddComment(ctx, testAgent, c.ID, "开始干活"); err != nil {
		t.Fatalf("评论失败: %v", err)
	}
	if _, err := e.core.MoveCard(ctx, testAgent, c.ID, kanban.MoveCardInput{ColumnID: e.col(colReview)}); err != nil {
		t.Fatalf("移列失败: %v", err)
	}
	if _, err := e.core.AddComment(ctx, testActor, c.ID, "人的评论"); err != nil {
		t.Fatalf("人评论失败: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	all := strings.Join(texts, "\n")
	for _, want := range []string{"领取了任务", "添加了评论：开始干活", "移到"} {
		if !strings.Contains(all, want) {
			t.Fatalf("应该推送「%s」，实际收到 %d 条: %s", want, len(texts), all)
		}
	}
	if strings.Contains(all, "人的评论") {
		t.Fatalf("人的评论不应推送: %s", all)
	}
}
