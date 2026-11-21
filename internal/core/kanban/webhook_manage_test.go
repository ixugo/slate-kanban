package kanban_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// TestWebhookCRUD 验证 webhook 增删改查、脱敏与数量上限。
func TestWebhookCRUD(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	created, err := e.core.CreateWebhook(ctx, testActor, e.board.ID, kanban.CreateWebhookInput{
		Name: "钉钉群", Type: kanban.WebhookTypeDingTalk,
		URL: "https://oapi.dingtalk.com/robot/send?access_token=abc", Secret: "sec",
		EventTypes: []string{kanban.EventCardMoved},
	})
	if err != nil {
		t.Fatalf("创建: %v", err)
	}
	if !created.SecretSet || created.SecretMask != "******" || created.SecretSet && created.URL == "" {
		t.Fatalf("应脱敏密钥: %+v", created)
	}
	list, err := e.core.ListWebhooks(ctx, testActor, e.board.ID)
	if err != nil || len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("列表: %v %+v", err, list)
	}
	patched, err := e.core.UpdateWebhook(ctx, testActor, created.ID, kanban.UpdateWebhookInput{
		Enabled: ptrBool(false),
	})
	if err != nil || patched.Enabled {
		t.Fatalf("更新: %v %+v", err, patched)
	}
	if err := e.core.DeleteWebhook(ctx, testActor, created.ID); err != nil {
		t.Fatalf("删除: %v", err)
	}
	if err := e.core.DeleteWebhook(ctx, testActor, created.ID); err != nil {
		t.Fatalf("幂等删除: %v", err)
	}
	for i := range kanban.MaxWebhooksPerBoard {
		_, err := e.core.CreateWebhook(ctx, testActor, e.board.ID, kanban.CreateWebhookInput{
			Name: fmt.Sprintf("w%d", i), Type: kanban.WebhookTypeWeCom,
			URL: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=x",
		})
		if err != nil {
			t.Fatalf("第 %d 条: %v", i, err)
		}
	}
	if _, err := e.core.CreateWebhook(ctx, testActor, e.board.ID, kanban.CreateWebhookInput{
		Name: "超限", Type: kanban.WebhookTypeWeCom,
		URL: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=y",
	}); !errors.Is(err, kanban.ErrWebhookLimit) {
		t.Fatalf("应返回上限错误: %v", err)
	}
}

// TestDeleteBoardRemovesWebhooks 验证删除看板级联删除 webhook。
func TestDeleteBoardRemovesWebhooks(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	wh, err := e.core.CreateWebhook(ctx, testActor, e.board.ID, kanban.CreateWebhookInput{
		Name: "级联", Type: kanban.WebhookTypeWeCom,
		URL: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=z",
	})
	if err != nil {
		t.Fatalf("创建: %v", err)
	}
	if err := e.core.DeleteBoard(ctx, testActor, e.board.ID); err != nil {
		t.Fatalf("删看板: %v", err)
	}
	if _, err := e.db.Webhook().GetByID(ctx, wh.ID); err == nil {
		t.Fatal("webhook 应被删掉")
	}
}

func ptrBool(v bool) *bool { return &v }
