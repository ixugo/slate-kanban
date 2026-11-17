package accessapi

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/ixugo/goddd/pkg/web"

	"slate-kanban/internal/core/access"
	"slate-kanban/internal/core/kanban"
)

// ctxKey 是放进请求上下文的键的类型，用自己的类型避免和别的包的键冲突。
type ctxKey int

const (
	principalKey ctxKey = iota // 认证通过的令牌身份
	actorKey                   // 对应的看板操作者
)

// Middleware 返回认证中间件：读取 Authorization 请求头，认证通过就把身份放进请求上下文，
// 失败直接返回 401（查库出错时返回数据库错误）并中止后面的处理。
// 后面的处理函数要用 c.Request.Context() 取身份，也要用它调用 Core，客户端断开时才能一起取消。
func (a API) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		p, err := a.core.Authenticate(c.Request.Context(), c.GetHeader("Authorization"))
		if err != nil {
			web.AbortWithStatusJSON(c, err)
			return
		}
		c.Request = c.Request.WithContext(WithPrincipal(c.Request.Context(), p))
		c.Next()
	}
}

// ActorOf 由令牌身份算出看板操作者：管理令牌对应人（名称 admin），其余令牌对应 agent（名称是令牌绑定的名称）。
// REST 的认证中间件和 MCP 适配器都用它，保证同一个令牌在两个入口里是同一个操作者。
func ActorOf(p access.Principal) kanban.Actor {
	kind := kanban.ActorAgent
	if p.Admin {
		kind = kanban.ActorHuman
	}
	return kanban.Actor{Kind: kind, Name: p.Name}
}

// WithPrincipal 把令牌身份和它对应的看板操作者放进上下文。
// 认证中间件和需要模拟已登录请求的测试都用它。
func WithPrincipal(ctx context.Context, p access.Principal) context.Context {
	ctx = context.WithValue(ctx, principalKey, p)
	return context.WithValue(ctx, actorKey, ActorOf(p))
}

// PrincipalFrom 取出认证通过的令牌身份，用于令牌管理，权限由 access.Core 判断。
// 上下文里没有身份说明路由没有挂认证中间件，按未认证处理而不是放行。
func PrincipalFrom(ctx context.Context) (access.Principal, error) {
	p, ok := ctx.Value(principalKey).(access.Principal)
	if !ok {
		return access.Principal{}, access.ErrUnauthorized
	}
	return p, nil
}

// ActorFrom 取出看板操作者，传给 kanban.Core。没有身份时同样返回 401。
func ActorFrom(ctx context.Context) (kanban.Actor, error) {
	a, ok := ctx.Value(actorKey).(kanban.Actor)
	if !ok {
		return kanban.Actor{}, access.ErrUnauthorized
	}
	return a, nil
}
