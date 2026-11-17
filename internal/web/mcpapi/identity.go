package mcpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/auth"

	"slate-kanban/internal/core/access"
	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/web/api/accessapi"
)

// actorKey 是 TokenInfo.Extra 里存放看板操作者（kanban.Actor）的键。
const actorKey = "actor"

// errAuthUnavailable 是认证过程本身出错（比如查库失败）时返回给调用方的错误文本。
// 调用方还没有通过认证，所以不告诉它内部发生了什么，完整原因只记在日志里。
var errAuthUnavailable = errors.New("authentication temporarily unavailable")

// newVerifier 返回 SDK 认证中间件用的令牌校验函数。每个 HTTP 请求都会调用它，
// 所以令牌一旦被删除，下一个请求就会失败，不用等会话结束。
// 成功后的 TokenInfo 带两样东西：UserID 让 SDK 防止会话被别的身份拿去用；
// Extra 里是看板操作者，工具处理函数直接取用，不必再查一次库。
func newVerifier(core access.Core) auth.TokenVerifier {
	return func(ctx context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
		principal, err := core.Authenticate(ctx, "Bearer "+token)
		switch {
		case errors.Is(err, access.ErrUnauthorized):
			return nil, auth.ErrInvalidToken
		case err != nil:
			slog.ErrorContext(ctx, "MCP 认证时出错", "err", err)
			return nil, errAuthUnavailable
		}
		actor := accessapi.ActorOf(principal)
		return &auth.TokenInfo{
			UserID: string(actor.Kind) + ":" + actor.Name,
			Extra:  map[string]any{actorKey: actor},
		}, nil
	}
}

// actorOf 从令牌信息里取出看板操作者。没有令牌信息或里面没有操作者，
// 说明请求没有经过认证中间件，按未认证处理，而不是放行。
func actorOf(info *auth.TokenInfo) (kanban.Actor, error) {
	if info == nil {
		return kanban.Actor{}, access.ErrUnauthorized
	}
	actor, ok := info.Extra[actorKey].(kanban.Actor)
	if !ok {
		return kanban.Actor{}, access.ErrUnauthorized
	}
	return actor, nil
}
