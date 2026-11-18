package mcpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/kanbantest"
	"slate-kanban/internal/lifecycle"
	"slate-kanban/internal/web/mcpapi"
)

// stack 是一套接好内存数据库的完整 MCP 服务，和正式运行一样包了一层 lifecycle。
type stack struct {
	URL          string
	core         kanban.Core
	api          mcpapi.API
	closeNow     context.CancelFunc
	stopNotifier func()
}

// newStack 搭一套全新的服务。
func newStack(t *testing.T, opts ...mcpapi.Option) *stack {
	t.Helper()
	core := kanban.NewCore(kanbantest.NewDB(t))
	api, stop := mcpapi.New(core, "test", opts...)
	t.Cleanup(stop)
	closing, closeNow := context.WithCancel(t.Context())
	mux := http.NewServeMux()
	mux.Handle("/mcp", api.Handler())
	srv := httptest.NewServer(lifecycle.Wrap(mux, closing))
	t.Cleanup(func() {
		srv.CloseClientConnections()
		srv.Close()
	})
	return &stack{
		URL: srv.URL + "/mcp", core: core, api: api,
		closeNow: closeNow, stopNotifier: stop,
	}
}

// mcpQuery 给 MCP 请求加 ?name= 或管理视图 &role=manager；admin 非空时用管理视图。
type mcpQuery struct {
	name  string
	admin bool
	host  string
}

// RoundTrip 实现 http.RoundTripper。
func (q mcpQuery) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	u := *r.URL
	qv := u.Query()
	if q.admin {
		qv.Set("role", "manager")
		if q.name != "" {
			qv.Set("name", q.name)
		}
	} else if q.name != "" {
		qv.Set("name", q.name)
	}
	u.RawQuery = qv.Encode()
	r.URL = &u
	if q.host != "" {
		r.Host = q.host
	}
	return http.DefaultTransport.RoundTrip(r)
}

// connect 连上 MCP 并完成初始化；admin 为 true 时用管理视图。
func (s *stack) connect(t *testing.T, name string, admin bool, opts *mcp.ClientOptions) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, opts)
	sess, err := client.Connect(t.Context(), &mcp.StreamableClientTransport{
		Endpoint: s.URL, HTTPClient: &http.Client{Transport: mcpQuery{name: name, admin: admin}},
	}, nil)
	if err != nil {
		t.Fatalf("连接 MCP 服务失败: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess
}

// agent 以名为 name 的 agent 身份连接。
func (s *stack) agent(t *testing.T, name string) *mcp.ClientSession {
	t.Helper()
	return s.connect(t, name, false, nil)
}

// human 以管理视图（人）身份连接。
func (s *stack) human(t *testing.T) *mcp.ClientSession {
	t.Helper()
	return s.connect(t, "manager", true, nil)
}

// token 兼容旧 harness：返回名字，供基于 URL 的身份使用。
func (s *stack) token(t *testing.T, name string) string {
	t.Helper()
	return name
}

// adminToken 兼容旧 harness：管理视图用固定占位串即可。
func (s *stack) adminToken() string { return "manager" }

type result struct {
	IsError bool
	Text    string
	Data    map[string]any
}

func call(t *testing.T, sess *mcp.ClientSession, name string, args map[string]any) result {
	t.Helper()
	res, err := sess.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("调用 %s 失败: %v", name, err)
	}
	return unpack(t, res)
}

func unpack(t *testing.T, res *mcp.CallToolResult) result {
	t.Helper()
	out := result{IsError: res.IsError}
	for _, c := range res.Content {
		if text, ok := c.(*mcp.TextContent); ok {
			out.Text += text.Text
		}
	}
	if res.StructuredContent != nil {
		raw, err := json.Marshal(res.StructuredContent)
		if err != nil {
			t.Fatalf("序列化结构化内容失败: %v", err)
		}
		if err := json.Unmarshal(raw, &out.Data); err != nil {
			t.Fatalf("结构化内容不是 JSON 对象: %v\n%s", err, raw)
		}
	}
	return out
}

func ok(t *testing.T, sess *mcp.ClientSession, name string, args map[string]any) result {
	t.Helper()
	out := call(t, sess, name, args)
	if out.IsError {
		t.Fatalf("调用 %s 应该成功，实际是工具错误: %s", name, out.Text)
	}
	return out
}

func toolNames(t *testing.T, sess *mcp.ClientSession) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	for tool, err := range sess.Tools(t.Context(), nil) {
		if err != nil {
			t.Fatalf("列出工具失败: %v", err)
		}
		names[tool.Name] = true
	}
	return names
}

type raw struct {
	name    string
	admin   bool
	session string
	method  string
	body    string
	headers map[string]string
}

func (s *stack) do(t *testing.T, r raw) (int, http.Header, string) {
	t.Helper()
	url := s.URL
	if r.name != "" || r.admin {
		sep := "?"
		if strings.Contains(url, "?") {
			sep = "&"
		}
		if r.admin {
			url += sep + "role=manager"
			sep = "&"
			if r.name != "" {
				url += sep + "name=" + r.name
			}
		} else {
			url += sep + "name=" + r.name
		}
	}
	method := r.method
	if method == "" {
		method = http.MethodPost
	}
	req, err := http.NewRequestWithContext(t.Context(), method, url, strings.NewReader(r.body))
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if r.session != "" {
		req.Header.Set("Mcp-Session-Id", r.session)
	}
	for k, v := range r.headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应失败: %v", err)
	}
	return resp.StatusCode, resp.Header, string(body)
}

const initializeBody = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"raw","version":"0"}}}`
