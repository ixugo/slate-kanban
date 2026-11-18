package restactor

import (
	"context"

	"github.com/gin-gonic/gin"

	"slate-kanban/internal/core/kanban"
)

type ctxKeyActor struct{}

// humanActor 是 REST 固定的人操作者。
var humanActor = kanban.Actor{Kind: kanban.ActorHuman, Name: "human"}

// Middleware 给 /api/v1 路由组挂上固定的人操作者。
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(withHumanActor(c.Request.Context()))
		c.Next()
	}
}

// withHumanActor 把人操作者写入 context。
func withHumanActor(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKeyActor{}, humanActor)
}

// From 从 REST 上下文取出人操作者。
func From(ctx context.Context) (kanban.Actor, error) {
	a, ok := ctx.Value(ctxKeyActor{}).(kanban.Actor)
	if !ok {
		return kanban.Actor{}, kanban.ErrInvalidActor
	}
	return a, nil
}
