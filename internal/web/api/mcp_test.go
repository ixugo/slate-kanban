package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// 这里只验证 /mcp 接进了整套路由：路径、认证、和 REST 共用同一批令牌与同一个看板领域。
// 工具、资源和通知本身的行为在 mcpapi 包里测。

// mcpPatience 是等一条异步通知的最长时间。
const mcpPatience = 5 * time.Second

// bearerTransport 给每个请求加上令牌，让 SDK 客户端以这个身份访问 /mcp。
type bearerTransport struct{ token string }

// RoundTrip 实现 http.RoundTripper。
func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

// startServer 用共用的路由起一个真实的 HTTP 服务，测试结束时关掉它，包括还挂着的通知流。
func startServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(shared.engine)
	t.Cleanup(func() {
		srv.CloseClientConnections()
		srv.Close()
	})
	return srv
}

// mcpConnect 以 token 的身份连上 /mcp 并完成初始化，测试结束时关闭会话。
func mcpConnect(t *testing.T, srv *httptest.Server, token string, opts *mcp.ClientOptions) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "router-test", Version: "0"}, opts)
	sess, err := client.Connect(t.Context(), &mcp.StreamableClientTransport{
		Endpoint:   srv.URL + mcpPath,
		HTTPClient: &http.Client{Transport: bearerTransport{token: token}},
	}, nil)
	if err != nil {
		t.Fatalf("连接 /mcp 失败: %v", err)
	}
	t.Cleanup(func() {
		if err := sess.Close(); err != nil {
			t.Logf("关闭 MCP 会话: %v", err)
		}
	})
	return sess
}

// createRESTBoard 通过 REST 创建一个看板，返回它的 id 和第一列的 id。
func createRESTBoard(t *testing.T, name, slug string) (boardID, firstColumnID int64) {
	t.Helper()
	resp := call(t, http.MethodPost, "/api/v1/boards", testAdminToken, map[string]string{"name": name, "slug": slug})
	var out struct {
		ID      int64 `json:"id"`
		Columns []struct {
			ID int64 `json:"id"`
		} `json:"columns"`
	}
	resp.JSON(t, &out)
	if resp.Code != http.StatusOK || out.ID == 0 || len(out.Columns) == 0 {
		t.Fatalf("创建看板失败: %d %s", resp.Code, resp.Body)
	}
	return out.ID, out.Columns[0].ID
}

// TestMCPRequiresToken 验证 /mcp 和 REST 一样必须带令牌：没有令牌、令牌不对，POST 和 GET 都是 401；
// 管理令牌能通过认证（通过之后协议层可能因为请求不合法而拒绝，但一定不是 401）。
func TestMCPRequiresToken(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodGet} {
		if resp := call(t, method, mcpPath, "", nil); resp.Code != http.StatusUnauthorized {
			t.Errorf("%s /mcp 没有令牌应该返回 401: %d %s", method, resp.Code, resp.Body)
		}
		if resp := call(t, method, mcpPath, "wrong-token", nil); resp.Code != http.StatusUnauthorized {
			t.Errorf("%s /mcp 令牌不对应该返回 401: %d", method, resp.Code)
		}
	}
	if resp := call(t, http.MethodPost, mcpPath, testAdminToken, map[string]string{"x": "y"}); resp.Code == http.StatusUnauthorized {
		t.Errorf("管理令牌应该能通过 /mcp 的认证: %d %s", resp.Code, resp.Body)
	}
}

// TestMCPSharesTokensAndBoards 验证 MCP 和 REST 共用同一批令牌和同一份看板数据：
// REST 创建的 agent 令牌能连上 /mcp 并被认成 agent，REST 创建的看板能从 MCP 读到；令牌在 REST 删除后，MCP 立刻不认。
func TestMCPSharesTokensAndBoards(t *testing.T) {
	srv := startServer(t)
	created := call(t, http.MethodPost, "/api/v1/agent-tokens", testAdminToken, map[string]string{"name": "mcp-bot"})
	plain, _ := field(t, created, "token").(string)
	tokenID := int64(field(t, created, "id").(float64))
	createRESTBoard(t, "MCP 共用看板", "RTMCPSHARE")

	sess := mcpConnect(t, srv, plain, nil)
	if got := sess.InitializeResult().ServerInfo.Version; got != testBuildVersion {
		t.Errorf("MCP 服务应该报告构建版本号 %q，实际 %q", testBuildVersion, got)
	}
	who, err := sess.CallTool(t.Context(), &mcp.CallToolParams{Name: "whoami"})
	if err != nil || who.IsError {
		t.Fatalf("whoami 失败: %v %+v", err, who)
	}
	if text := resultText(who); !strings.Contains(text, `"mcp-bot"`) || !strings.Contains(text, `"agent"`) {
		t.Errorf("REST 创建的令牌在 MCP 里应该是 agent mcp-bot: %s", text)
	}
	boards, err := sess.CallTool(t.Context(), &mcp.CallToolParams{Name: "list_boards"})
	if err != nil || boards.IsError {
		t.Fatalf("list_boards 失败: %v %+v", err, boards)
	}
	if !strings.Contains(resultText(boards), "RTMCPSHARE") {
		t.Errorf("REST 创建的看板应该能从 MCP 读到: %s", resultText(boards))
	}

	if resp := call(t, http.MethodDelete, "/api/v1/agent-tokens/"+strconv.FormatInt(tokenID, 10), testAdminToken, nil); resp.Code != http.StatusOK {
		t.Fatalf("删除令牌失败: %d %s", resp.Code, resp.Body)
	}
	if _, err := sess.CallTool(t.Context(), &mcp.CallToolParams{Name: "whoami"}); err == nil {
		t.Error("令牌被删除后，已有会话的下一次调用也应该失败")
	}
}

// TestMCPNotifiedByREST 验证 MCP 和 REST 共用同一个看板 Core：agent 订阅看板之后，
// 别人通过 REST 改了看板，agent 会收到更新通知。这证明通知器接在了 REST 也在用的那个 Core 上。
func TestMCPNotifiedByREST(t *testing.T) {
	srv := startServer(t)
	boardID, columnID := createRESTBoard(t, "MCP 通知看板", "RTMCPNOTE")
	notices := make(chan string, 16)
	sess := mcpConnect(t, srv, testAdminToken, &mcp.ClientOptions{
		ResourceUpdatedHandler: func(_ context.Context, r *mcp.ResourceUpdatedNotificationRequest) {
			notices <- r.Params.URI
		},
	})
	// 等创建看板自己触发的通知发完（合并窗口只有毫秒级），再订阅，这样后面收到的一定是建卡片引起的
	time.Sleep(time.Second)
	uri := "slate://boards/" + strconv.FormatInt(boardID, 10)
	if err := sess.Subscribe(t.Context(), &mcp.SubscribeParams{URI: uri}); err != nil {
		t.Fatalf("订阅看板失败: %v", err)
	}

	resp := call(t, http.MethodPost, "/api/v1/boards/"+strconv.FormatInt(boardID, 10)+"/cards", testAdminToken,
		map[string]any{"title": "REST 建的卡片", "column_id": columnID})
	if resp.Code != http.StatusOK {
		t.Fatalf("REST 建卡片失败: %d %s", resp.Code, resp.Body)
	}
	select {
	case got := <-notices:
		if got != uri {
			t.Errorf("通知的地址 = %q，期望 %q", got, uri)
		}
	case <-time.After(mcpPatience):
		t.Fatal("REST 改了看板，订阅的 agent 没有收到通知")
	}
}

// resultText 拼出工具结果里的全部文本内容。
func resultText(res *mcp.CallToolResult) string {
	var sb strings.Builder
	for _, c := range res.Content {
		if text, ok := c.(*mcp.TextContent); ok {
			sb.WriteString(text.Text)
		}
	}
	return sb.String()
}
