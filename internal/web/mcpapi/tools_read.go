package mcpapi

import (
	"context"

	"slate-kanban/internal/core/kanban"
)

// whoamiOut 是 whoami 的输出。
type whoamiOut struct {
	Name     string           `json:"name"`
	Kind     kanban.ActorKind `json:"kind"`
	View     view             `json:"view"`
	Version  string           `json:"server_version"`
	BoardURI string           `json:"board_resource_template"`
}

// whoami 返回调用者的身份和当前视图。身份来自令牌，不来自入参，所以没有入参。
func (a API) whoami(v view) func(context.Context, kanban.Actor, struct{}) (any, error) {
	return func(_ context.Context, actor kanban.Actor, _ struct{}) (any, error) {
		return whoamiOut{
			Name: actor.Name, Kind: actor.Kind, View: v,
			Version: a.version, BoardURI: boardURITemplate,
		}, nil
	}
}
