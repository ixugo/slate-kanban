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

	"slate-kanban/internal/core/access"
	"slate-kanban/internal/core/access/accesstest"
	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/kanbantest"
	"slate-kanban/internal/lifecycle"
	"slate-kanban/internal/web/mcpapi"
)

const adminToken = "admin-secret"

// stack 是一套接好内存数据库的完整 MCP 服务：令牌、看板，监听真实的端口，
// 和正式运行一样用 lifecycle.Wrap 包了一层，所以测试可以模拟服务开始关闭。
type stack struct {
	URL          string
	access       access.Core
	core         kanban.Core
	api          mcpapi.API
	tokens       map[string]string  // agent 名称 -> 令牌明文
	closeNow     context.CancelFunc // 调用它模拟服务开始关闭
	stopNotifier func()             // 提前停止通知器，测试结束时还会再调用一次，所以必须可以重复调用
}

// newStack 搭一套全新的服务，opts 调整 MCP 适配器的可选行为。
func newStack(t *testing.T, opts ...mcpapi.Option) *stack {
	t.Helper()
	acore := accesstest.NewCore(t, adminToken)
	core := kanban.NewCore(kanbantest.NewDB(t))
	api, stop := mcpapi.New(acore, core, "test", opts...)
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
		URL: srv.URL + "/mcp", access: acore, core: core, api: api, tokens: map[string]string{},
		closeNow: closeNow, stopNotifier: stop,
	}
}

// token 返回名为 name 的 agent 令牌，第一次用到时才创建。
func (s *stack) token(t *testing.T, name string) string {
	t.Helper()
	if tok, ok := s.tokens[name]; ok {
		return tok
	}
	made, err := s.access.CreateToken(t.Context(), access.Principal{Admin: true, Name: access.AdminName}, name)
	if err != nil {
		t.Fatalf("创建令牌 %s 失败: %v", name, err)
	}
	s.tokens[name] = made.Plain
	return made.Plain
}

// bearer 给每个请求加上 Authorization 头；host 不为空时同时改写 Host 头，用来模拟反向代理。
type bearer struct {
	token string
	host  string
}

// RoundTrip 实现 http.RoundTripper。
func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	if b.token != "" {
		r.Header.Set("Authorization", "Bearer "+b.token)
	}
	if b.host != "" {
		r.Host = b.host
	}
	return http.DefaultTransport.RoundTrip(r)
}

// connect 用 SDK 客户端连上服务并完成初始化，测试结束时关闭会话。token 为管理令牌或 agent 令牌。
func (s *stack) connect(t *testing.T, token string, opts *mcp.ClientOptions) *mcp.ClientSession {
	t.Helper()
	return s.connectVia(t, bearer{token: token}, opts)
}

// connectVia 和 connect 一样，但可以自己指定带认证头的 RoundTripper。
func (s *stack) connectVia(t *testing.T, rt http.RoundTripper, opts *mcp.ClientOptions) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, opts)
	sess, err := client.Connect(t.Context(), &mcp.StreamableClientTransport{
		Endpoint: s.URL, HTTPClient: &http.Client{Transport: rt},
	}, nil)
	if err != nil {
		t.Fatalf("连接 MCP 服务失败: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess
}

// agent 以名为 name 的 agent 的身份连接。
func (s *stack) agent(t *testing.T, name string) *mcp.ClientSession {
	t.Helper()
	return s.connect(t, s.token(t, name), nil)
}

// human 以管理令牌（人）的身份连接。
func (s *stack) human(t *testing.T) *mcp.ClientSession {
	t.Helper()
	return s.connect(t, adminToken, nil)
}

// result 是一次工具调用的结果：是否是工具错误、文本内容、结构化内容。
type result struct {
	IsError bool
	Text    string
	Data    map[string]any
}

// call 调用工具并整理结果。调用本身失败（比如参数不符合 Schema）会让测试终止；
// 工具返回的错误不会，调用方用 IsError 判断。
func call(t *testing.T, sess *mcp.ClientSession, name string, args map[string]any) result {
	t.Helper()
	res, err := sess.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("调用 %s 失败: %v", name, err)
	}
	return unpack(t, res)
}

// unpack 把 SDK 的调用结果整理成 result。
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

// ok 要求工具调用成功，否则让测试终止，并返回结果。
func ok(t *testing.T, sess *mcp.ClientSession, name string, args map[string]any) result {
	t.Helper()
	out := call(t, sess, name, args)
	if out.IsError {
		t.Fatalf("调用 %s 应该成功，实际是工具错误: %s", name, out.Text)
	}
	return out
}

// toolNames 返回会话里全部工具的名称。
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

// raw 是一次手写的 HTTP 请求，用来测试 SDK 客户端不会发出的情况。
type raw struct {
	token   string
	session string
	method  string
	body    string
	headers map[string]string
}

// do 发出请求，返回状态码、响应头和响应体。
func (s *stack) do(t *testing.T, r raw) (int, http.Header, string) {
	t.Helper()
	method := r.method
	if method == "" {
		method = http.MethodPost
	}
	req, err := http.NewRequestWithContext(t.Context(), method, s.URL, strings.NewReader(r.body))
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if r.token != "" {
		req.Header.Set("Authorization", "Bearer "+r.token)
	}
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

// initializeBody 是标准的初始化请求。
const initializeBody = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"raw","version":"0"}}}`
