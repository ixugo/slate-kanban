package mcpapi

import (
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"slate-kanban/internal/core/kanban"
)

const managementGateway = "enable_management"

type managementIn struct {
	IncludeTools bool           `json:"include_tools,omitempty" jsonschema:"返回管理工具及其参数 schema；供不能刷新工具列表的客户端使用"`
	Tool         string         `json:"tool,omitempty" jsonschema:"兼容调用：已披露的管理工具名；首次启用时留空"`
	Arguments    map[string]any `json:"arguments,omitempty" jsonschema:"兼容调用时，传入目标管理工具的参数对象"`
}

type managementOut struct {
	Enabled bool        `json:"enabled"`
	Next    string      `json:"next"`
	Tools   []*mcp.Tool `json:"tools,omitempty"`
}

// managementTools 同时记录披露的工具名，供兼容入口限定转发范围。
type managementTools struct {
	server *mcp.Server
	names  map[string]bool
}

func registerManagement[In any](group managementTools, sp spec, fn func(context.Context, kanban.Actor, In) (any, error)) {
	group.names[sp.name] = true
	register(group.server, sp, func(ctx context.Context, actor kanban.Actor, in In) (any, error) {
		// 只有显式的管理操作使用 Core 已有的人工入口，普通工具保留工作身份。
		actor.Kind = kanban.ActorHuman
		return fn(ctx, actor, in)
	})
}

type managementDispatchKey struct{}

// Clients with a fixed tool list use the same gateway and normal SDK validation.
// Forwarding through the receiving handler keeps the original session and identity.
func (a API) registerManagementDisclosure(s *mcp.Server) {
	var enable sync.Once
	var enabled atomic.Bool
	var names map[string]bool
	s.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if call, ok := req.(*mcp.CallToolRequest); ok && call.Params.Name == managementGateway {
				ctx = context.WithValue(ctx, managementDispatchKey{}, next)
			}
			return next(ctx, method, req)
		}
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: managementGateway, Annotations: kindWrite.annotations(),
		Description: "仅当用户明确要求管理看板设置、列或自动化时，开启当前会话的管理工具。客户端未刷新时传 include_tools 查看目录，再用 tool 和 arguments 兼容调用。",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in managementIn) (*mcp.CallToolResult, any, error) {
		if _, err := requestActor(req); err != nil {
			return nil, nil, toolError(ctx, err)
		}
		dispatch := ctx.Value(managementDispatchKey{}).(mcp.MethodHandler)
		if in.Tool != "" {
			if !enabled.Load() || !names[in.Tool] {
				return nil, nil, errors.New("请先开启管理工具，再选择已披露的管理操作")
			}
			result, err := invokeManagement(ctx, dispatch, req, in.Tool)
			return result, nil, err
		}
		if in.Arguments != nil {
			return nil, nil, errors.New("传 arguments 时还需指定 tool")
		}

		enable.Do(func() {
			names = a.registerAdminTools(s)
			enabled.Store(true)
		})
		out := managementOut{
			Enabled: true,
			Next:    "可刷新 MCP 工具列表。未刷新的客户端传 include_tools=true 获取目录，再传 tool 和 arguments 调用。",
		}
		if in.IncludeTools {
			list := &mcp.ListToolsRequest{
				Session: req.Session, Extra: req.Extra, Params: &mcp.ListToolsParams{Meta: req.Params.Meta},
			}
			result, err := dispatch(ctx, "tools/list", list)
			if err != nil {
				return nil, nil, err
			}
			for _, tool := range result.(*mcp.ListToolsResult).Tools {
				if names[tool.Name] {
					out.Tools = append(out.Tools, tool)
				}
			}
		}
		return nil, out, nil
	})
}

func invokeManagement(ctx context.Context, dispatch mcp.MethodHandler, req *mcp.CallToolRequest, name string) (*mcp.CallToolResult, error) {
	// Keep the original JSON instead of round-tripping numbers through float64.
	var raw struct {
		Arguments jsontext.Value `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &raw); err != nil {
		return nil, err
	}
	params := *req.Params
	params.Name = name
	params.Arguments = []byte(raw.Arguments)
	forward := &mcp.CallToolRequest{Session: req.Session, Params: &params, Extra: req.Extra}
	result, err := dispatch(ctx, "tools/call", forward)
	if err != nil {
		return nil, err
	}
	return result.(*mcp.CallToolResult), nil
}
