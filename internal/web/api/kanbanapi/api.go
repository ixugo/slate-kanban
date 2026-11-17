// Package kanbanapi 是看板领域的 REST 适配器。处理函数只做四件事：解析请求、取出调用者身份、
// 调用 kanban.Core、返回结果；权限判断和字段校验都留在 Core 里，这样 REST 和 MCP 两个入口行为一致。
package kanbanapi

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ixugo/goddd/pkg/web"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/web/restactor"
)

// API 持有看板领域的业务入口，以及实时流的时间参数。
type API struct {
	core      kanban.Core
	heartbeat time.Duration // 实时流发保活注释行的间隔
	maxAge    time.Duration // 实时流单次连接的最长时间
}

// New 创建看板领域的 REST 适配器。
func New(core kanban.Core) API {
	return API{core: core, heartbeat: streamHeartbeat, maxAge: streamMaxAge}
}

// Register 注册全部看板路由。r 应该是已经挂了认证中间件的路由组，处理函数从请求上下文取身份。
func Register(r gin.IRouter, a API) {
	a.boardRoutes(r)
	a.columnRoutes(r)
	a.cardRoutes(r)
	a.claimRoutes(r)
	a.ruleRoutes(r)
	a.eventRoutes(r)
}

// empty 是没有内容的成功响应，序列化成 {}。
type empty struct{}

// route 把"带身份的处理函数"接到 goddd 的 WrapH 上：先从请求上下文取出看板操作者，再调用处理函数。
// 上下文用 c.Request.Context()，客户端断开连接时阻塞等待和实时流会跟着取消。
func route[I, O any](fn func(context.Context, kanban.Actor, *I) (O, error)) gin.HandlerFunc {
	return web.WrapH(func(c *gin.Context, in *I) (O, error) {
		ctx := c.Request.Context()
		actor, err := restactor.From(ctx)
		if err != nil {
			var zero O
			return zero, err
		}
		return fn(ctx, actor, in)
	})
}

// boardPath 是路径里的看板编号。路径字段统一标 json:"-" form:"-"，
// 只能从 URL 取值，请求体和查询参数里的同名字段覆盖不了。
type boardPath struct {
	BoardID int64 `uri:"board_id" json:"-" form:"-"`
}

// columnPath 是路径里的看板和列编号。
type columnPath struct {
	boardPath
	ColumnID int64 `uri:"column_id" json:"-" form:"-"`
}

// cardPath 是路径里的卡片编号。
type cardPath struct {
	CardID int64 `uri:"card_id" json:"-" form:"-"`
}

// commentPath 是路径里的评论编号。
type commentPath struct {
	CommentID int64 `uri:"comment_id" json:"-" form:"-"`
}

// rulePath 是路径里的规则编号。
type rulePath struct {
	RuleID int64 `uri:"rule_id" json:"-" form:"-"`
}
