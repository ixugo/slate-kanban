package mcpapi_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"slate-kanban/internal/core/access"
)

// agentTools 和 adminTools 是两个视图里应该有的全部工具，每增加一个工具就要同步这里，
// 这样"哪个视图能看到什么"永远有一份明确的清单。
var (
	agentTools = []string{"whoami"}
	adminTools = []string{"whoami"}
)

// sorted 把工具名集合换成排好序的切片，方便比较和打印。
func sorted(names map[string]bool) []string {
	out := make([]string, 0, len(names))
	for n := range names {
		out = append(out, n)
	}
	slices.Sort(out)
	return out
}

// TestRejectsBadTokens 验证认证：没有令牌、格式不对、不存在的令牌、已删除的令牌都返回 401，
// 并且认证排在请求体检查之前：没有令牌的人即使发超大的请求体，看到的也只是 401。
func TestRejectsBadTokens(t *testing.T) {
	s := newStack(t)
	made, err := s.access.CreateToken(t.Context(), access.Principal{Admin: true, Name: access.AdminName}, "gone")
	if err != nil {
		t.Fatalf("创建令牌失败: %v", err)
	}
	if err := s.access.DeleteToken(t.Context(), access.Principal{Admin: true, Name: access.AdminName}, made.ID); err != nil {
		t.Fatalf("删除令牌失败: %v", err)
	}
	cases := []struct {
		name string
		r    raw
	}{
		{"没有令牌", raw{body: initializeBody}},
		{"不存在的令牌", raw{token: "slate_not-a-real-token", body: initializeBody}},
		{"已删除的令牌", raw{token: made.Plain, body: initializeBody}},
		{"方案不对", raw{body: initializeBody, headers: map[string]string{"Authorization": "Basic " + adminToken}}},
		{"没有令牌的超大请求体", raw{body: strings.Repeat("x", 2<<20)}},
	}
	for _, tc := range cases {
		if code, _, body := s.do(t, tc.r); code != http.StatusUnauthorized {
			t.Errorf("%s：期望 401，实际 %d %s", tc.name, code, body)
		}
	}
}

// TestToolViews 验证两套视图：agent 令牌只看到 agent 能用的工具，管理令牌只看到管理工具。
func TestToolViews(t *testing.T) {
	s := newStack(t)
	if got := sorted(toolNames(t, s.agent(t, "claude"))); !slices.Equal(got, sorted(setOf(agentTools))) {
		t.Errorf("agent 视图的工具不对: %v", got)
	}
	if got := sorted(toolNames(t, s.human(t))); !slices.Equal(got, sorted(setOf(adminTools))) {
		t.Errorf("管理视图的工具不对: %v", got)
	}
}

// setOf 把工具名列表换成集合。
func setOf(names []string) map[string]bool {
	out := make(map[string]bool, len(names))
	for _, n := range names {
		out[n] = true
	}
	return out
}

// TestWhoami 验证身份工具：返回名称、类型、视图、服务版本和订阅用的资源地址。
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
			map[string]any{"name": "admin", "kind": "human", "view": "admin"},
		},
	}
	for _, tc := range cases {
		got := tc.sess()
		for k, want := range tc.want {
			if got[k] != want {
				t.Errorf("%s：%s 期望 %v，实际 %v", tc.name, k, want, got[k])
			}
		}
		if got["server_version"] != "test" || got["board_resource_template"] != "slate://boards/{board_id}" {
			t.Errorf("%s：版本或资源地址不对: %v", tc.name, got)
		}
	}
}

// initSession 手写一次初始化请求，返回会话编号。
func initSession(t *testing.T, s *stack, token string) string {
	t.Helper()
	code, header, body := s.do(t, raw{token: token, body: initializeBody})
	if code != http.StatusOK || header.Get("Mcp-Session-Id") == "" {
		t.Fatalf("初始化失败: %d %s", code, body)
	}
	return header.Get("Mcp-Session-Id")
}

// TestSessionBoundToIdentity 验证会话绑定创建它的身份：知道了别人的会话编号也没用，
// 换成别的 agent 令牌、管理令牌都返回 403，只有原来的令牌能继续用。
func TestSessionBoundToIdentity(t *testing.T) {
	s := newStack(t)
	session := initSession(t, s, s.token(t, "alice"))
	list := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`
	for name, token := range map[string]string{"别的 agent": s.token(t, "bob"), "管理令牌": adminToken} {
		if code, _, body := s.do(t, raw{token: token, session: session, body: list}); code != http.StatusForbidden {
			t.Errorf("%s 用别人的会话应该返回 403: %d %s", name, code, body)
		}
	}
	if code, _, body := s.do(t, raw{token: s.token(t, "alice"), session: session, body: list}); code != http.StatusOK {
		t.Errorf("原来的令牌应该能继续用: %d %s", code, body)
	}
}

// TestRevokedTokenStopsSession 验证令牌被删除后，已经建立的会话立刻失效：
// 每个请求都重新认证，不用等会话过期。
func TestRevokedTokenStopsSession(t *testing.T) {
	s := newStack(t)
	made, err := s.access.CreateToken(t.Context(), access.Principal{Admin: true, Name: access.AdminName}, "temp")
	if err != nil {
		t.Fatalf("创建令牌失败: %v", err)
	}
	sess := s.connect(t, made.Plain, nil)
	ok(t, sess, "whoami", nil)
	if err := s.access.DeleteToken(t.Context(), access.Principal{Admin: true, Name: access.AdminName}, made.ID); err != nil {
		t.Fatalf("删除令牌失败: %v", err)
	}
	if _, err := sess.CallTool(t.Context(), &mcp.CallToolParams{Name: "whoami"}); err == nil {
		t.Error("令牌删除后，已有会话的下一次调用应该失败")
	}
}

// TestWorksBehindReverseProxy 验证放在反向代理后面能用：代理从本机连过来，Host 头是域名。
// SDK 默认会把"本机服务收到非本机 Host"当成 DNS 重绑定攻击返回 403，这里必须关掉它，
// 因为 /mcp 一定要令牌，不会因此多出攻击面。
func TestWorksBehindReverseProxy(t *testing.T) {
	s := newStack(t)
	sess := s.connectVia(t, bearer{token: s.token(t, "claude"), host: "slate.example.com"}, nil)
	if got := ok(t, sess, "whoami", nil).Data["name"]; got != "claude" {
		t.Errorf("通过域名访问应该能正常调用: %v", got)
	}
}

// TestRequestBodyLimit 验证请求体上限：带着有效令牌发超过 1 MiB 的请求体返回 413。
func TestRequestBodyLimit(t *testing.T) {
	s := newStack(t)
	huge := `{"jsonrpc":"2.0","id":1,"method":"ping","params":{"pad":"` + strings.Repeat("x", 2<<20) + `"}}`
	if code, _, body := s.do(t, raw{token: adminToken, body: huge}); code != http.StatusRequestEntityTooLarge {
		t.Errorf("超大请求体应该返回 413: %d %.200s", code, body)
	}
}

// TestNewProtocolGetsClearError 验证新版协议（2026-07-28 起的无状态协议）的请求得到明确的答复，而不是卡住或被当成别的错误：
// 返回 JSON-RPC 的"不支持的协议版本"错误，并列出服务支持的旧版本，遵循规范的客户端会据此自动降级。
func TestNewProtocolGetsClearError(t *testing.T) {
	s := newStack(t)
	body := `{"jsonrpc":"2.0","id":7,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`
	code, _, resp := s.do(t, raw{token: adminToken, body: body, headers: map[string]string{"MCP-Protocol-Version": "2026-07-28"}})
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
		t.Errorf("期望 400 和带支持版本列表的协议版本错误，实际 %d %s", code, resp)
	}
}
