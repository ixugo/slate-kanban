package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"slate-kanban/skills"
)

// 这里验证接入说明接进了整套路由：哪些免认证、哪些要令牌，以及说明里给出的地址真的能用。
// 说明的内容、地址校验和渲染在 connectapi 包里测。

// TestConnectAuth 验证认证边界：安装说明和 skill 下载免认证（agent 在装好 MCP 之前就要读）；
// 结构化说明要令牌，没有令牌或令牌不对是 401；响应里不会出现请求带来的令牌。
func TestConnectAuth(t *testing.T) {
	for _, path := range []string{"/install.md", "/skills/slate-kanban/SKILL.md"} {
		if resp := call(t, http.MethodGet, path, "", nil); resp.Code != http.StatusOK {
			t.Errorf("%s 应该免认证: %d %s", path, resp.Code, resp.Body)
		}
	}
	const path = "/api/v1/connect"
	if resp := call(t, http.MethodGet, path, "", nil); resp.Code != http.StatusUnauthorized {
		t.Errorf("没有令牌应该返回 401: %d", resp.Code)
	}
	if resp := call(t, http.MethodGet, path, "wrong-token", nil); resp.Code != http.StatusUnauthorized {
		t.Errorf("令牌不对应该返回 401: %d", resp.Code)
	}
	resp := call(t, http.MethodGet, path, testAdminToken, nil)
	if resp.Code != http.StatusOK || strings.Contains(string(resp.Body), testAdminToken) {
		t.Errorf("管理令牌应该能读，并且响应里不能有令牌: %d %s", resp.Code, resp.Body)
	}
	if resp := call(t, http.MethodGet, path, createAgentToken(t, "connect-bot"), nil); resp.Code != http.StatusOK {
		t.Errorf("agent 令牌也应该能读: %d %s", resp.Code, resp.Body)
	}
}

// TestAdvertisedURLsWork 验证说明里给出的地址是真的能用的：从结构化说明取到 MCP 地址和 skill 地址，
// 用它们真的连上 MCP 并调用工具、下载 skill；下载到的内容、MCP 资源里读到的内容、编译进服务的内容三者完全一致。
func TestAdvertisedURLsWork(t *testing.T) {
	srv := startServer(t)
	plain := createAgentToken(t, "guide-bot")

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/v1/connect", http.NoBody)
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+plain)
	var guide struct {
		BaseURL  string `json:"base_url"`
		MCPURL   string `json:"mcp_url"`
		SkillURL string `json:"skill_url"`
	}
	getJSON(t, req, &guide)
	if guide.BaseURL != srv.URL || guide.MCPURL != srv.URL+"/mcp" {
		t.Fatalf("地址应该取自请求里的主机，期望 %s，实际 %+v", srv.URL, guide)
	}

	sess := mcpConnectURL(t, guide.MCPURL, plain)
	who, err := sess.CallTool(t.Context(), &mcp.CallToolParams{Name: "whoami"})
	if err != nil || who.IsError || !strings.Contains(resultText(who), `"guide-bot"`) {
		t.Fatalf("用说明里的 MCP 地址应该能连上并认出 guide-bot: %v %+v", err, who)
	}

	downloaded := getText(t, guide.SkillURL)
	if downloaded != skills.SlateKanban() {
		t.Errorf("下载到的 skill 应该和编译进服务的一致")
	}
	res, err := sess.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: "slate://skills/slate-kanban"})
	if err != nil || len(res.Contents) != 1 || res.Contents[0].Text != downloaded {
		t.Errorf("MCP 资源里的 skill 应该和下载到的一致: %v", err)
	}
}

// createAgentToken 用管理令牌创建一个 agent 令牌，返回令牌明文；创建失败就让测试终止。
func createAgentToken(t *testing.T, name string) string {
	t.Helper()
	created := call(t, http.MethodPost, "/api/v1/agent-tokens", testAdminToken, map[string]string{"name": name})
	plain, ok := field(t, created, "token").(string)
	if !ok || plain == "" {
		t.Fatalf("创建令牌 %s 没有拿到明文: %s", name, created.Body)
	}
	return plain
}

// mcpConnectURL 以 token 的身份连上指定地址的 MCP 服务，测试结束时关闭会话。
func mcpConnectURL(t *testing.T, endpoint, token string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "connect-test", Version: "0"}, nil)
	sess, err := client.Connect(t.Context(), &mcp.StreamableClientTransport{
		Endpoint:   endpoint,
		HTTPClient: &http.Client{Transport: bearerTransport{token: token}},
	}, nil)
	if err != nil {
		t.Fatalf("连接 %s 失败: %v", endpoint, err)
	}
	t.Cleanup(func() {
		if err := sess.Close(); err != nil {
			t.Logf("关闭 MCP 会话: %v", err)
		}
	})
	return sess
}

// getText 不带令牌下载一个地址的内容，状态码不是 200 就让测试终止。
func getText(t *testing.T, url string) string {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("请求 %s 失败: %v", url, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("关闭响应体: %v", err)
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("下载 %s 失败: %d %v", url, resp.StatusCode, err)
	}
	return string(body)
}

// getJSON 发送请求并把 JSON 响应解析到 out，状态码不是 200 或解析失败就让测试终止。
func getJSON(t *testing.T, req *http.Request, out any) {
	t.Helper()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("请求 %s 失败: %v", req.URL, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("关闭响应体: %v", err)
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("请求 %s 失败: %d %v %s", req.URL, resp.StatusCode, err, body)
	}
	if err := json.Unmarshal(body, out); err != nil {
		t.Fatalf("响应不是合法 JSON: %v\n%s", err, body)
	}
}
