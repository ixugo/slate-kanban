package mcpapi_test

import (
	"context"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestManagementDisclosureIsSessionLocal(t *testing.T) {
	s := newStack(t)
	changed := make(chan struct{}, 1)
	session := s.connect(t, "planner", false, &mcp.ClientOptions{
		ToolListChangedHandler: func(context.Context, *mcp.ToolListChangedRequest) {
			select {
			case changed <- struct{}{}:
			default:
			}
		},
	})
	other := s.agent(t, "planner")
	board := ok(t, session, "create_board", map[string]any{"name": "管理测试", "template": "dev"}).Data
	args := map[string]any{"board_id": board["id"], "name": "发布"}
	hidden := []string{"update_board", "create_column", "update_column", "reorder_columns", "delete_column", "create_rule", "update_rule", "delete_rule"}
	names := toolNames(t, session)
	for _, name := range hidden {
		if names[name] {
			t.Fatalf("管理工具 %s 在显式启用前已出现", name)
		}
	}
	if reply, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "create_column", Arguments: args}); err == nil && !reply.IsError {
		t.Fatal("显式启用前调用管理工具应失败")
	}
	for range 2 {
		ok(t, session, "enable_management", nil)
	}
	select {
	case <-changed:
	case <-time.After(3 * time.Second):
		t.Fatal("客户端没有收到工具列表更新通知")
	}
	names = toolNames(t, session)
	otherNames := toolNames(t, other)
	for _, name := range hidden {
		if !names[name] {
			t.Errorf("启用后缺少 %s", name)
		}
		if otherNames[name] {
			t.Errorf("同名的另一条连接泄露了 %s", name)
		}
	}
	legacyNames := toolNames(t, s.human(t))
	for _, name := range []string{"list_events", "delete_card", "delete_comment"} {
		if names[name] || legacyNames[name] {
			t.Errorf("废弃工具 %s 仍在 MCP 工具表", name)
		}
	}
	if identity := ok(t, session, "whoami", nil).Data; identity["kind"] != "agent" || identity["view"] != "agent" {
		t.Fatalf("工作工具的调用身份发生变化: %v", identity)
	}
	if created := ok(t, session, "create_column", args).Data; created["name"] != "发布" {
		t.Fatalf("列创建失败: %v", created)
	}
	ok(t, session, "update_board", map[string]any{"board_id": board["id"], "description": "验收通过"})
	detail := ok(t, other, "get_board", map[string]any{"board_id": board["id"]}).Data
	if detail["description"] != "验收通过" {
		t.Fatalf("更新没有写入数据库: %v", detail["description"])
	}
	column := detail["columns"].([]any)[0].(map[string]any)
	ok(t, session, "create_rule", map[string]any{
		"board_id": board["id"], "name": "开始记录", "trigger": "card_entered", "column_id": column["id"],
		"actions": []map[string]any{{"type": "set_started_at"}},
	})
	if reply, err := other.CallTool(t.Context(), &mcp.CallToolParams{Name: "create_column", Arguments: args}); err == nil && !reply.IsError {
		t.Fatal("另一条未启用连接执行了管理动作")
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	if toolNames(t, s.agent(t, "planner"))["create_column"] {
		t.Fatal("重新连接不应继承管理工具")
	}
}

func TestManagementCompatibilityCallUsesCurrentSession(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	session := s.agent(t, "planner")
	other := s.agent(t, "planner")

	settings := map[string]any{
		"tool": "update_board", "arguments": map[string]any{"board_id": b.ID, "description": "管理配置"},
	}
	if !call(t, session, "enable_management", settings).IsError {
		t.Fatal("兼容调用不能隐式开启管理")
	}

	enabled := ok(t, session, "enable_management", map[string]any{"include_tools": true}).Data
	catalog := items(t, enabled, "tools")
	if len(catalog) != 8 {
		t.Fatalf("只应披露 8 个管理工具: %v", catalog)
	}
	found := false
	for _, tool := range catalog {
		if tool["name"] == "update_board" && tool["inputSchema"] != nil {
			found = true
		}
	}
	if !found {
		t.Fatal("兼容工具目录缺少看板更新的 SDK 入参 schema")
	}
	if !call(t, other, "enable_management", settings).IsError {
		t.Fatal("另一条连接不应获得兼容管理入口")
	}

	got := ok(t, session, "enable_management", settings).Data
	if got["description"] != "管理配置" {
		t.Fatalf("兼容管理工具未执行: %v", got)
	}
	badSettings := map[string]any{
		"tool": "update_board", "arguments": map[string]any{"board_id": b.ID, "unsupported": "bad"},
	}
	if !call(t, session, "enable_management", badSettings).IsError {
		t.Fatal("兼容入口必须复用实际管理工具的 schema 校验")
	}
	if !call(t, session, "enable_management", map[string]any{"tool": "whoami"}).IsError {
		t.Fatal("兼容入口只转发管理操作")
	}
}
