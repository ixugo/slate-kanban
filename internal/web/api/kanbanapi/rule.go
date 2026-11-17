package kanbanapi

import (
	"context"

	"github.com/gin-gonic/gin"

	"slate-kanban/internal/core/kanban"
)

// ruleRoutes 注册自动化规则的路由。读取规则用看板详情或快照，不另设列表接口。
func (a API) ruleRoutes(r gin.IRouter) {
	r.POST("/boards/:board_id/rules", route(a.createRule))
	r.PATCH("/rules/:rule_id", route(a.updateRule))
	r.DELETE("/rules/:rule_id", route(a.deleteRule))
}

// createRuleInput 是新增规则的请求。
type createRuleInput struct {
	boardPath
	kanban.CreateRuleInput
}

// updateRuleInput 是修改规则的请求。
type updateRuleInput struct {
	rulePath
	kanban.UpdateRuleInput
}

// createRule 新增一条自动化规则，只有人能做。
func (a API) createRule(ctx context.Context, actor kanban.Actor, in *createRuleInput) (*kanban.Rule, error) {
	return a.core.CreateRule(ctx, actor, in.BoardID, in.CreateRuleInput)
}

// updateRule 修改一条规则，只更新请求里出现的字段，只有人能做。
func (a API) updateRule(ctx context.Context, actor kanban.Actor, in *updateRuleInput) (*kanban.Rule, error) {
	return a.core.UpdateRule(ctx, actor, in.RuleID, in.UpdateRuleInput)
}

// deleteRule 删除一条规则，只有人能做；规则已经不存在也返回成功。
func (a API) deleteRule(ctx context.Context, actor kanban.Actor, in *rulePath) (empty, error) {
	return empty{}, a.core.DeleteRule(ctx, actor, in.RuleID)
}
