package kanbanapi

import (
	"context"

	"github.com/gin-gonic/gin"

	"slate-kanban/internal/core/kanban"
)

// webhookRoutes 注册活动通知 webhook 路由，仅供人类管理员使用。
func (a API) webhookRoutes(r gin.IRouter) {
	r.GET("/boards/:board_id/webhooks", route(a.listWebhooks))
	r.POST("/boards/:board_id/webhooks", route(a.createWebhook))
	r.PATCH("/webhooks/:webhook_id", route(a.updateWebhook))
	r.DELETE("/webhooks/:webhook_id", route(a.deleteWebhook))
	r.POST("/webhooks/:webhook_id/test", route(a.testWebhook))
}

// createWebhookInput 是新增 webhook 的请求。
type createWebhookInput struct {
	boardPath
	kanban.CreateWebhookInput
}

// updateWebhookInput 是修改 webhook 的请求。
type updateWebhookInput struct {
	webhookPath
	kanban.UpdateWebhookInput
}

// listWebhooks 返回看板的全部 webhook，密钥已脱敏。
func (a API) listWebhooks(ctx context.Context, actor kanban.Actor, in *boardPath) ([]kanban.WebhookView, error) {
	return a.core.ListWebhooks(ctx, actor, in.BoardID)
}

// createWebhook 新增 webhook。
func (a API) createWebhook(ctx context.Context, actor kanban.Actor, in *createWebhookInput) (*kanban.WebhookView, error) {
	return a.core.CreateWebhook(ctx, actor, in.BoardID, in.CreateWebhookInput)
}

// updateWebhook 修改 webhook。
func (a API) updateWebhook(ctx context.Context, actor kanban.Actor, in *updateWebhookInput) (*kanban.WebhookView, error) {
	return a.core.UpdateWebhook(ctx, actor, in.WebhookID, in.UpdateWebhookInput)
}

// deleteWebhook 删除 webhook。
func (a API) deleteWebhook(ctx context.Context, actor kanban.Actor, in *webhookPath) (empty, error) {
	return empty{}, a.core.DeleteWebhook(ctx, actor, in.WebhookID)
}

// testWebhook 发送测试消息。
func (a API) testWebhook(ctx context.Context, actor kanban.Actor, in *webhookPath) (*kanban.TestWebhookResult, error) {
	return a.core.TestWebhook(ctx, actor, in.WebhookID)
}
