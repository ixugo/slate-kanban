// Package accessapi 是身份和令牌的 REST 适配器：认证中间件、当前身份、agent 令牌管理。
package accessapi

import (
	"github.com/gin-gonic/gin"
	"github.com/ixugo/goddd/pkg/web"

	"slate-kanban/internal/core/access"
	"slate-kanban/internal/core/kanban"
)

// API 持有令牌领域的业务入口。
type API struct {
	core access.Core
}

// New 创建令牌领域的 REST 适配器。
func New(core access.Core) API {
	return API{core: core}
}

// Register 注册 /me 和 /agent-tokens 路由。r 应该是已经挂了认证中间件的路由组。
func Register(r gin.IRouter, a API) {
	r.GET("/me", web.WrapH(a.me))
	tokens := r.Group("/agent-tokens")
	tokens.GET("", web.WrapH(a.listTokens))
	tokens.POST("", web.WrapH(a.createToken))
	tokens.DELETE("/:token_id", web.WrapH(a.deleteToken))
}

// meOutput 是当前身份：名称和类型，网页登录时用它验证令牌并决定显示哪些管理功能。
type meOutput struct {
	Name string           `json:"name"`
	Kind kanban.ActorKind `json:"kind"`
}

// me 返回当前请求的身份。
func (a API) me(c *gin.Context, _ *struct{}) (*meOutput, error) {
	actor, err := ActorFrom(c.Request.Context())
	if err != nil {
		return nil, err
	}
	return &meOutput{Name: actor.Name, Kind: actor.Kind}, nil
}
