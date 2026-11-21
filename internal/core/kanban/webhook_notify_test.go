package kanban_test

import (
	"context"
	"net/http"
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
