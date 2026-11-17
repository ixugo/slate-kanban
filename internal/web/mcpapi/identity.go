package mcpapi

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/auth"

	"slate-kanban/internal/core/kanban"
)

// actorKey 是 TokenInfo.Extra 里存放看板操作者的键，工具从 req.Extra 取出，测试里也会用到。
const actorKey = "actor"

// actorFromHeaders 从 HTTP 请求头取出 middleware 写入的操作者。
func actorFromHeaders(h http.Header) (kanban.Actor, error) {
	kind := kanban.ActorKind(h.Get(headerActorKind))
	name := h.Get(headerActorName)
	if name == "" || kind == "" {
		return kanban.Actor{}, kanban.ErrInvalidActor
	}
	actor := kanban.Actor{Kind: kind, Name: name}
	if err := actor.Validate(); err != nil {
		return kanban.Actor{}, err
	}
	return actor, nil
}

// actorOf 从 TokenInfo 取出看板操作者，会话创建路径和工具路径都可能用到。
func actorOf(info *auth.TokenInfo) (kanban.Actor, error) {
	if info == nil {
		return kanban.Actor{}, kanban.ErrInvalidActor
	}
	actor, ok := info.Extra[actorKey].(kanban.Actor)
	if !ok {
		return kanban.Actor{}, kanban.ErrInvalidActor
	}
	return actor, nil
}
