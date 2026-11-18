package mcpapi_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	agentTools = []string{
		"whoami", "list_boards", "get_board", "list_cards", "get_card", "pull_updates", "wait_for_updates",
		"next_card", "claim_card", "release_card", "finish_card", "comment_card", "create_card", "update_card",
		"move_card", "set_project_path", "create_board", "upload_image", "get_image",
	}
	adminTools = []string{
		"whoami", "list_boards", "get_board", "list_cards", "get_card", "list_events",
		"release_card", "finish_card", "comment_card", "create_card", "update_card", "move_card",
		"create_board", "update_board", "create_column", "update_column", "reorder_columns", "delete_column",
		"create_rule", "update_rule", "delete_rule", "delete_card", "delete_comment", "upload_image", "get_image",
	}
)

func sorted(names map[string]bool) []string {
	out := make([]string, 0, len(names))
	for n := range names {
		out = append(out, n)
	}
	slices.Sort(out)
	return out
}

func setOf(names []string) map[string]bool {
	out := make(map[string]bool, len(names))
	for _, n := range names {
		out[n] = true
	}
	return out
}

// TestBadIdentityQuery 验证非法 name/role 返回 400。
func TestBadIdentityQuery(t *testing.T) {
	s := newStack(t)
	if code, _, _ := s.do(t, raw{name: "Bad Name", body: initializeBody}); code != http.StatusBadRequest {
		t.Errorf("非法 name 应 400: %d", code)
	}
	if code, _, _ := s.do(t, raw{admin: true, name: "human", body: initializeBody}); code != http.StatusBadRequest {
		t.Errorf("保留名应 400: %d", code)
	}
}

func TestToolViews(t *testing.T) {
	s := newStack(t)
	if got := sorted(toolNames(t, s.agent(t, "claude"))); !slices.Equal(got, sorted(setOf(agentTools))) {
		t.Errorf("agent 视图的工具不对: %v", got)
	}
	if got := sorted(toolNames(t, s.human(t))); !slices.Equal(got, sorted(setOf(adminTools))) {
		t.Errorf("管理视图的工具不对: %v", got)
	}
}

func TestWhoami(t *testing.T) {
	s := newStack(t)
	cases := []struct {
		name string
		sess func() map[string]any
		want map[string]any
	}{
		{
			"agent", func() map[string]any { return ok(t, s.agent(t, "claude"), "whoami", nil).Data },
			map[string]any{"name": "claude", "kind": "agent", "view": "agent"},
		},
		{
			"人", func() map[string]any { return ok(t, s.human(t), "whoami", nil).Data },
			map[string]any{"name": "manager", "kind": "human", "view": "admin"},
		},
	}
	for _, tc := range cases {
		got := tc.sess()
		for k, want := range tc.want {
			if got[k] != want {
				t.Errorf("%s：%s 期望 %v，实际 %v", tc.name, k, want, got[k])
			}
		}
	}
}

func initSession(t *testing.T, s *stack, name string) string {
	t.Helper()
	code, header, body := s.do(t, raw{name: name, body: initializeBody})
	if code != http.StatusOK || header.Get("Mcp-Session-Id") == "" {
		t.Fatalf("初始化失败: %d %s", code, body)
	}
	return header.Get("Mcp-Session-Id")
}

func TestResponsesAreJSON(t *testing.T) {
	s := newStack(t)
	name := s.token(t, "claude")
	_, header, _ := s.do(t, raw{name: name, body: initializeBody})
	if got := header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("初始化的响应应该是 application/json，实际 %q", got)
	}
	session := header.Get("Mcp-Session-Id")
	call := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"whoami","arguments":{}}}`
	code, header, body := s.do(t, raw{name: name, session: session, body: call})
	if got := header.Get("Content-Type"); code != http.StatusOK || !strings.HasPrefix(got, "application/json") {
		t.Errorf("工具调用的响应应该是 200 和 application/json，实际 %d %q %.200s", code, got, body)
	}
}

func TestSessionBoundToIdentity(t *testing.T) {
	s := newStack(t)
	session := initSession(t, s, "alice")
	list := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`
	if code, _, body := s.do(t, raw{name: "bob", session: session, body: list}); code != http.StatusForbidden {
		t.Errorf("换 name 用别人的会话应该 403: %d %s", code, body)
	}
	if code, _, body := s.do(t, raw{name: "alice", session: session, body: list}); code != http.StatusOK {
		t.Errorf("原 name 应继续用: %d %s", code, body)
	}
}

func TestWorksBehindReverseProxy(t *testing.T) {
	s := newStack(t)
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	sess, err := client.Connect(t.Context(), &mcp.StreamableClientTransport{
		Endpoint: s.URL, HTTPClient: &http.Client{Transport: mcpQuery{name: "claude", host: "slate.example.com"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	if got := ok(t, sess, "whoami", nil).Data["name"]; got != "claude" {
		t.Errorf("通过域名 Host 应能调用: %v", got)
	}
}

func TestRequestBodyLimit(t *testing.T) {
	s := newStack(t)
	huge := `{"jsonrpc":"2.0","id":1,"method":"ping","params":{"pad":"` + strings.Repeat("x", 2<<20) + `"}}`
	if code, _, body := s.do(t, raw{admin: true, body: huge}); code != http.StatusRequestEntityTooLarge {
		t.Errorf("超大请求体应该返回 413: %d %.200s", code, body)
	}
}

func TestNewProtocolGetsClearError(t *testing.T) {
	s := newStack(t)
	body := `{"jsonrpc":"2.0","id":7,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`
	code, _, resp := s.do(t, raw{admin: true, body: body, headers: map[string]string{"MCP-Protocol-Version": "2026-07-28"}})
	var out struct {
		ID    int `json:"id"`
		Error struct {
			Code int `json:"code"`
			Data struct {
				Supported []string `json:"supported"`
			} `json:"data"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(resp), &out); err != nil {
		t.Fatalf("响应不是 JSON-RPC: %v\n%s", err, resp)
	}
	if code != http.StatusBadRequest || out.ID != 7 || out.Error.Code != -32022 || !slices.Contains(out.Error.Data.Supported, "2025-11-25") {
		t.Errorf("期望 400 和协议版本错误，实际 %d %s", code, resp)
	}
}
