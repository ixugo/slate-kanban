package mcpapi

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/auth"

	"slate-kanban/internal/core/kanban"
)

// headerVerifier 从 identityMiddleware 写入的请求头构造 TokenInfo，供 SDK 做会话绑定。
// 不校验 Bearer 字符串本身，只认头里的身份；AllowMissingExpiration 因为不再有真实令牌过期时间。
func headerVerifier(ctx context.Context, _ string, r *http.Request) (*auth.TokenInfo, error) {
	kind := kanban.ActorKind(r.Header.Get(headerActorKind))
	name := r.Header.Get(headerActorName)
	if name == "" || kind == "" {
		return nil, auth.ErrInvalidToken
	}
	actor := kanban.Actor{Kind: kind, Name: name}
	if err := actor.Validate(); err != nil {
		return nil, auth.ErrInvalidToken
	}
	return &auth.TokenInfo{
		UserID: string(actor.Kind) + ":" + actor.Name,
		Extra:  map[string]any{actorKey: actor},
	}, nil
}

// bearerGate 在无 Authorization 时补上占位 Bearer，再走 RequireBearerToken，这样 MCP 客户端不必配置令牌。
func bearerGate(next http.Handler) http.Handler {
	opts := &auth.RequireBearerTokenOptions{AllowMissingExpiration: true}
	inner := auth.RequireBearerToken(headerVerifier, opts)(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			r.Header.Set("Authorization", "Bearer open")
		}
		inner.ServeHTTP(w, r)
	})
}
