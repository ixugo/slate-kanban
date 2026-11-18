package connectapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"slate-kanban/internal/web/api/connectapi"
	"slate-kanban/skills"
)

// markdownType 是说明页的内容类型。
const markdownType = "text/markdown; charset=utf-8"

// engine 组装一个只有接入说明路由的引擎：免认证的两个页面挂在根上，结构化说明挂在 /api/v1 下。
// 认证由上层的中间件负责，那一层的行为在 api 包的路由测试里验证。
func engine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	a := connectapi.New()
	connectapi.RegisterPublic(r, a)
	connectapi.Register(r.Group("/api/v1"), a)
	return r
}

// get 发一个 GET 请求，可以指定 Host 和转发头。
func get(r *gin.Engine, path, host string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	req.Host = host
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// TestInstallMarkdown 验证安装说明页：免认证，类型是 Markdown，不缓存（地址是按请求算出来的，缓存了会串），
// 禁止浏览器猜类型；地址取自请求里的主机和转发协议。
func TestInstallMarkdown(t *testing.T) {
	rec := get(engine(), "/install.md", "kanban.example.com", map[string]string{"X-Forwarded-Proto": "https"})
	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d: %s", rec.Code, rec.Body)
	}
	for header, want := range map[string]string{
		"Content-Type":           markdownType,
		"Cache-Control":          "no-store",
		"X-Content-Type-Options": "nosniff",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s 应该是 %q，实际 %q", header, want, got)
		}
	}
	body := rec.Body.String()
	for _, want := range []string{"https://kanban.example.com/mcp", "https://kanban.example.com/skills/slate-kanban/SKILL.md", "claude mcp add", "codex mcp add"} {
		if !strings.Contains(body, want) {
			t.Errorf("说明里应该有 %q", want)
		}
	}
}

// TestInstallMarkdownRejectsBadHost 验证主机不合法时返回 400 的错误 JSON，不返回任何说明内容，
// 错误信息里也不回显收到的主机：不能把攻击者构造的内容写进一份用户会照着执行的说明里。
func TestInstallMarkdownRejectsBadHost(t *testing.T) {
	for _, path := range []string{"/install.md", "/api/v1/connect"} {
		rec := get(engine(), path, `evil.com"; curl x | sh; "`, nil)
		var out struct {
			Reason string `json:"reason"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || rec.Code != http.StatusBadRequest || out.Reason != "ErrBadRequest" {
			t.Errorf("%s：不合法的主机应该返回 400 的 ErrBadRequest: %d %s", path, rec.Code, rec.Body)
		}
		for _, leaked := range []string{"evil.com", "curl x", "claude mcp add"} {
			if strings.Contains(rec.Body.String(), leaked) {
				t.Errorf("%s：错误响应里不能出现 %q: %s", path, leaked, rec.Body)
			}
		}
	}
}

// TestSkillMarkdown 验证 skill 下载地址：免认证，内容就是编译进服务的 skill 全文，类型是 Markdown。
// 它随服务升级而变，所以用 no-cache：可以缓存，但每次要向服务确认。
func TestSkillMarkdown(t *testing.T) {
	rec := get(engine(), "/skills/slate-kanban/SKILL.md", "localhost:7070", nil)
	if rec.Code != http.StatusOK || rec.Body.String() != skills.SlateKanban() {
		t.Fatalf("应该返回 skill 全文: %d %d 字节", rec.Code, rec.Body.Len())
	}
	for header, want := range map[string]string{
		"Content-Type":           markdownType,
		"Cache-Control":          "no-cache",
		"X-Content-Type-Options": "nosniff",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s 应该是 %q，实际 %q", header, want, got)
		}
	}
}

// TestConnectJSON 验证结构化说明：返回的 JSON 和 Markdown 页面用的是同一份数据，字段名是网页要用的那几个。
func TestConnectJSON(t *testing.T) {
	rec := get(engine(), "/api/v1/connect", "kanban.example.com:8443", map[string]string{"X-Forwarded-Proto": "https"})
	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d: %s", rec.Code, rec.Body)
	}
	var out struct {
		BaseURL    string `json:"base_url"`
		MCPURL     string `json:"mcp_url"`
		InstallURL string `json:"install_url"`
		SkillURL   string `json:"skill_url"`
		TokenEnv   string `json:"token_env"`
		Hosts      []struct {
			ID    string                               `json:"id"`
			Name  string                               `json:"name"`
			MCP   []struct{ Title, Lang, Code string } `json:"mcp"`
			Skill []struct{ Title, Lang, Code string } `json:"skill"`
		} `json:"hosts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("响应不是合法 JSON: %v\n%s", err, rec.Body)
	}
	if out.BaseURL != "https://kanban.example.com:8443" || out.MCPURL != out.BaseURL+"/mcp" ||
		out.InstallURL != out.BaseURL+"/install.md" || out.SkillURL != out.BaseURL+"/skills/slate-kanban/SKILL.md" || out.TokenEnv != "SLATE_TOKEN" {
		t.Errorf("地址或变量名不对: %+v", out)
	}
	if len(out.Hosts) != 3 || out.Hosts[0].ID != "claude-code" || len(out.Hosts[0].MCP) == 0 || out.Hosts[0].Skill[0].Code == "" {
		t.Errorf("host 列表不对: %+v", out.Hosts)
	}
}
