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

func TestConnectPublicPages(t *testing.T) {
	for _, path := range []string{"/install.md", "/skills/slate-kanban/SKILL.md"} {
		if resp := call(t, http.MethodGet, path, "", nil); resp.Code != http.StatusOK {
			t.Errorf("%s 应该免认证: %d %s", path, resp.Code, resp.Body)
		}
	}
	if resp := call(t, http.MethodGet, "/api/v1/connect", "", nil); resp.Code != http.StatusOK {
		t.Errorf("/connect 应免认证: %d", resp.Code)
	}
}

func TestAdvertisedURLsWork(t *testing.T) {
	srv := startServer(t)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/v1/connect", http.NoBody)
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}
	var guide struct {
		BaseURL  string `json:"base_url"`
		MCPURL   string `json:"mcp_url"`
		SkillURL string `json:"skill_url"`
		Hosts    []struct {
			MCP []struct {
				Code string `json:"code"`
			} `json:"mcp"`
		} `json:"hosts"`
	}
	getJSON(t, req, &guide)
	if guide.BaseURL != srv.URL {
		t.Fatalf("base_url 应为 %s，实际 %s", srv.URL, guide.BaseURL)
	}
	mcpURL := guide.Hosts[0].MCP[0].Code
	if !strings.Contains(mcpURL, "name=claude-code") {
		t.Fatalf("工作视图 URL 应含 name: %s", mcpURL)
	}
	endpoint := srv.URL + "/mcp?name=guide-bot"
	sess := mcpConnectURL(t, endpoint, nil)
	who, err := sess.CallTool(t.Context(), &mcp.CallToolParams{Name: "whoami"})
	if err != nil || who.IsError || !strings.Contains(resultText(who), `"guide-bot"`) {
		t.Fatalf("MCP 应认出 guide-bot: %v", err)
	}
	downloaded := getText(t, guide.SkillURL)
	if downloaded != skills.SlateKanban() {
		t.Error("skill 应与编译进服务的一致")
	}
}

func mcpConnectURL(t *testing.T, endpoint string, opts *mcp.ClientOptions) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "connect-test", Version: "0"}, opts)
	sess, err := client.Connect(t.Context(), &mcp.StreamableClientTransport{
		Endpoint: endpoint, HTTPClient: http.DefaultClient,
	}, nil)
	if err != nil {
		t.Fatalf("连接 %s 失败: %v", endpoint, err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess
}

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
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("下载 %s 失败: %d %v", url, resp.StatusCode, err)
	}
	return string(body)
}

func getJSON(t *testing.T, req *http.Request, out any) {
	t.Helper()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("请求 %s 失败: %v", req.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("请求 %s 失败: %d %v %s", req.URL, resp.StatusCode, err, body)
	}
	if err := json.Unmarshal(body, out); err != nil {
		t.Fatalf("响应不是合法 JSON: %v\n%s", err, body)
	}
}
