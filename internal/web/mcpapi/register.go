package mcpapi

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"slate-kanban/internal/core/kanban"
)

// kind 描述一个工具对看板数据的影响，决定给 host 的注解：host 据此判断哪些调用可以放心自动执行。
type kind int

const (
	// kindRead 只读取，不改任何数据。
	kindRead kind = iota
	// kindWrite 新增或修改数据，但不会删除东西。
	kindWrite
	// kindDestroy 删除数据，删了就找不回来。
	kindDestroy
)

// spec 是一个工具对外的说明：名称、给模型看的描述和影响类型。描述是模型选择和使用工具的主要依据。
type spec struct {
	name string
	desc string
	kind kind
}

// annotations 按影响类型生成工具注解。协议里 destructiveHint 的默认值是 true，
// 所以写入类要明确写 false；所有工具都只和这个看板打交道，openWorldHint 一律为 false。
func (k kind) annotations() *mcp.ToolAnnotations {
	closed, destructive := false, k == kindDestroy
	a := &mcp.ToolAnnotations{OpenWorldHint: &closed}
	switch k {
	case kindRead:
		a.ReadOnlyHint = true
	case kindWrite, kindDestroy:
		a.DestructiveHint = &destructive
	}
	return a
}

// register 把一个业务函数包装成 MCP 工具并加到 server 上。所有工具共用这层包装：
// 先从请求里取出身份，再调用业务函数，最后把错误转成给 agent 看的文本。
// 业务函数只管调用 Core，返回要给 agent 看的对象（必须是 JSON 对象，列表要在外面包一层）。
func register[In any](s *mcp.Server, sp spec, fn func(ctx context.Context, actor kanban.Actor, in In) (any, error)) {
	tool := &mcp.Tool{Name: sp.name, Description: sp.desc, Annotations: sp.kind.annotations()}
	mcp.AddTool(s, tool, func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
		actor, err := requestActor(req)
		if err != nil {
			return nil, nil, toolError(ctx, err)
		}
		out, err := fn(ctx, actor, in)
		if err != nil {
			return nil, nil, toolError(ctx, err)
		}
		return nil, out, nil
	})
}

// requestActor 取出这次工具调用的发起者。身份由 identityMiddleware 写进 HTTP 头，SDK 原样放进 Extra.Header。
func requestActor(req *mcp.CallToolRequest) (kanban.Actor, error) {
	if req.Extra == nil || req.Extra.Header == nil {
		return kanban.Actor{}, kanban.ErrInvalidActor
	}
	if actor, err := actorFromHeaders(req.Extra.Header); err == nil {
		return actor, nil
	}
	return actorOf(req.Extra.TokenInfo)
}
