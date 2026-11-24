package api

import (
	json "encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"slate-kanban/internal/conf"
	"slate-kanban/internal/core/kanban/kanbantest"
	"slate-kanban/internal/web/api/connectapi"
	"slate-kanban/internal/web/api/kanbanapi"
	"slate-kanban/internal/web/mcpapi"
	"slate-kanban/internal/web/restactor"
	"slate-kanban/skills"
)

// 验证已建立 MCP 会话的每一次请求仍需口令；自定义 Skill 不走免认证的旧下载入口。
func TestSkillPasscodeOnRESTAndMCP(t *testing.T) {
	core := kanbantest.NewCore(t)
	api, stop := mcpapi.New(core, "test")
	t.Cleanup(stop)
	bc := &conf.Bootstrap{Server: conf.Server{HTTP: conf.ServerHTTP{Passcode: testPasscode}}}
	g := gin.New()
	g.Use(passcodeMiddleware(newPasscodeGuard(bc, newAttemptLimiter(nil))))
	g.Any("/mcp", gin.WrapH(api.Handler()))
	connectapi.RegisterPublic(g, connectapi.New(true))
	kanbanapi.Register(g.Group(apiPrefix, restactor.Middleware(), limitBody(maxRequestBody), requireJSON()), kanbanapi.New(core))
	request := func(method, path, body, session string, auth bool) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		if session != "" {
			req.Header.Set("Mcp-Session-Id", session)
			req.Header.Set("MCP-Protocol-Version", "2025-06-18")
		}
		if auth {
			req.Header.Set("Authorization", "Bearer "+testPasscode)
		}
		return serve(t, g, req)
	}
	const custom = `{"name":"private-guide","description":"test","body":"# private text"}`
	for _, call := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/skills", ""},
		{http.MethodPost, "/api/v1/skills", custom},
		{http.MethodGet, "/api/v1/skills/private-guide", ""},
		{http.MethodPut, "/api/v1/skills/private-guide", custom},
		{http.MethodDelete, "/api/v1/skills/private-guide", ""},
	} {
		if res := request(call.method, call.path, call.body, "", false); res.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s 未被口令保护: %d", call.method, call.path, res.Code)
		}
	}
	if res := request(http.MethodPost, "/api/v1/skills", custom, "", true); res.Code != http.StatusOK {
		t.Fatalf("已认证创建失败: %d %s", res.Code, res.Body)
	}
	if res := request(http.MethodGet, "/skills/private-guide/SKILL.md", "", "", false); res.Code != http.StatusNotFound {
		t.Fatalf("自定义 Skill 不应公开下载: %d %s", res.Code, res.Body)
	}
	if res := request(http.MethodGet, "/skills/slate-kanban/SKILL.md", "", "", false); res.Code != http.StatusOK || res.Body.String() != skills.SlateKanban() {
		t.Fatal("旧公开下载兼容失败")
	}
	const initialize = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{"extensions":{"io.modelcontextprotocol/skills":{}}},"clientInfo":{"name":"test","version":"0"}}}`
	path := "/mcp?name=skills-test"
	if res := request(http.MethodPost, path, initialize, "", false); res.Code != http.StatusUnauthorized {
		t.Fatal("未认证 MCP 初始化未拒绝")
	}
	init := request(http.MethodPost, path, initialize, "", true)
	session := init.Header().Get("Mcp-Session-Id")
	if init.Code != http.StatusOK || session == "" {
		t.Fatalf("已认证握手失败: %d %s", init.Code, init.Body)
	}
	t.Cleanup(func() { request(http.MethodDelete, path, "", session, true) })
	if res := request(http.MethodPost, path, `{"jsonrpc":"2.0","method":"notifications/initialized"}`, session, true); res.Code != http.StatusAccepted {
		t.Fatalf("initialized: %d %s", res.Code, res.Body)
	}
	for _, payload := range []string{
		`{"jsonrpc":"2.0","id":2,"method":"skills/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"skills/get","params":{"uri":"skill://private-guide/SKILL.md"}}`,
		`{"jsonrpc":"2.0","id":4,"method":"resources/read","params":{"uri":"skill://private-guide/SKILL.md"}}`,
		`{"jsonrpc":"2.0","id":5,"method":"resources/read","params":{"uri":"slate://skills/catalog"}}`,
	} {
		if res := request(http.MethodPost, path, payload, session, false); res.Code != http.StatusUnauthorized {
			t.Fatalf("已建立会话无口令仍应 401: %d %s", res.Code, res.Body)
		}
		res := request(http.MethodPost, path, payload, session, true)
		var reply map[string]any
		if err := json.Unmarshal(res.Body.Bytes(), &reply); err != nil || res.Code != http.StatusOK || reply["error"] != nil {
			t.Fatalf("带口令读取失败: %d %s %v", res.Code, res.Body, err)
		}
	}
}
