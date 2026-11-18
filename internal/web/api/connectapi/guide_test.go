package connectapi

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

var guideBases = []string{"http://localhost:7070", "https://kanban.example.com", "http://[::1]:7070"}

func TestBuildGuideShape(t *testing.T) {
	for _, base := range guideBases {
		g := buildGuide(base, false)
		if g.BaseURL != base || g.MCPURL != base+"/mcp" || g.InstallURL != base+"/install.md" ||
			g.SkillURL != base+"/skills/slate-kanban/SKILL.md" {
			t.Fatalf("地址不对: %+v", g)
		}
		ids := make([]string, 0, len(g.Hosts))
		for _, h := range g.Hosts {
			ids = append(ids, h.ID)
			if len(h.MCP) != 2 || len(h.Skill) != 1 {
				t.Errorf("%s：应有 2 个 MCP 片段和 1 个 skill 片段", h.ID)
			}
			for _, sn := range slices.Concat(h.MCP, h.Skill) {
				if sn.Kind == "" || sn.Title == "" || sn.Code == "" || (sn.Lang != "sh" && sn.Lang != "json") {
					t.Errorf("%s：步骤不完整: %+v", h.ID, sn)
				}
			}
			if h.MCP[0].Kind != "worker" || h.MCP[1].Kind != "manager" {
				t.Errorf("%s：MCP kind 顺序应为 worker、manager", h.ID)
			}
			if h.Skill[0].Kind != "skill" {
				t.Errorf("%s：skill kind 应为 skill", h.ID)
			}
			for _, sn := range h.MCP {
				if strings.Contains(strings.ToLower(sn.Code), "token") || strings.Contains(sn.Code, "Authorization") {
					t.Errorf("%s：MCP 步骤不应含令牌: %s", h.ID, sn.Code)
				}
			}
		}
		if !slices.Equal(ids, []string{"claude-code", "cursor", "codex"}) {
			t.Errorf("host 顺序不对: %v", ids)
		}
	}
}

func TestGuideQuotesURLs(t *testing.T) {
	for _, base := range guideBases {
		g := buildGuide(base, false)
		for _, h := range g.Hosts {
			for _, sn := range slices.Concat(h.MCP, h.Skill) {
				if sn.Lang != "sh" {
					continue
				}
				if strings.Contains(sn.Code, base) && !strings.Contains(sn.Code, "'"+base) {
					t.Errorf("%s：shell 里地址应加引号: %s", h.ID, sn.Code)
				}
			}
		}
	}
}

// hostMCP 返回指定 host 的两个 MCP 片段（worker、manager）。
func hostMCP(t *testing.T, g Guide, id string) []Snippet {
	t.Helper()
	for _, h := range g.Hosts {
		if h.ID == id {
			return h.MCP
		}
	}
	t.Fatalf("没有找到 host %s", id)
	return nil
}

// TestClaudeCodePasscodeHeaderAfterURL claude 的 --header 可以带多个值，写在地址前面会把地址当成请求头吞掉，
// 所以启用口令时必须把它放在地址后面。
func TestClaudeCodePasscodeHeaderAfterURL(t *testing.T) {
	for _, base := range guideBases {
		for _, sn := range hostMCP(t, buildGuide(base, true), "claude-code") {
			url := strings.Index(sn.Code, "'"+base)
			header := strings.Index(sn.Code, `--header "Authorization: Bearer `+passcodePlaceholder+`"`)
			if url < 0 || header < 0 || header < url {
				t.Errorf("--header 应出现在地址之后: %s", sn.Code)
			}
		}
	}
}

// TestCodexPasscodeUsesEnvVar 启用口令时 Codex 步骤用环境变量传口令，配置文件里不留明文。
func TestCodexPasscodeUsesEnvVar(t *testing.T) {
	for _, sn := range hostMCP(t, buildGuide(guideBases[0], true), "codex") {
		for _, want := range []string{
			"export " + codexPasscodeEnv + "='" + passcodePlaceholder + "'",
			"--bearer-token-env-var " + codexPasscodeEnv,
			"codex mcp add",
		} {
			if !strings.Contains(sn.Code, want) {
				t.Errorf("Codex 步骤缺少 %q: %s", want, sn.Code)
			}
		}
	}
}

// TestCursorPasscodeHeaderJSON 启用口令时 Cursor 的 mcp.json 片段是合法 JSON，并带 Authorization 头占位。
func TestCursorPasscodeHeaderJSON(t *testing.T) {
	for _, sn := range hostMCP(t, buildGuide(guideBases[0], true), "cursor") {
		var cfg struct {
			MCPServers map[string]struct {
				URL     string            `json:"url"`
				Headers map[string]string `json:"headers"`
			} `json:"mcpServers"`
		}
		if err := json.Unmarshal([]byte(sn.Code), &cfg); err != nil {
			t.Fatalf("Cursor JSON 无效: %v\n%s", err, sn.Code)
		}
		if len(cfg.MCPServers) != 1 {
			t.Fatalf("应只有一个服务: %s", sn.Code)
		}
		for name, srv := range cfg.MCPServers {
			if srv.Headers["Authorization"] != "Bearer "+passcodePlaceholder {
				t.Errorf("%s 缺少 Authorization 头占位: %+v", name, srv)
			}
		}
	}
}

// TestGuideWithoutPasscodeHasNoAuthHeader 未启用口令时，任何步骤都不应提到 Authorization 或占位符。
func TestGuideWithoutPasscodeHasNoAuthHeader(t *testing.T) {
	for _, h := range buildGuide(guideBases[0], false).Hosts {
		for _, sn := range h.MCP {
			if strings.Contains(sn.Code, "Authorization") || strings.Contains(sn.Code, passcodePlaceholder) ||
				strings.Contains(sn.Title, passcodePlaceholder) {
				t.Errorf("%s 未启用口令不应出现口令相关内容: %+v", h.ID, sn)
			}
		}
	}
}

func TestGuideCursorConfigIsJSON(t *testing.T) {
	base := guideBases[0]
	g := buildGuide(base, false)
	for _, h := range g.Hosts {
		if h.ID != "cursor" {
			continue
		}
		for i, sn := range h.MCP {
			var cfg struct {
				MCPServers map[string]struct {
					URL     string            `json:"url"`
					Headers map[string]string `json:"headers"`
				} `json:"mcpServers"`
			}
			if err := json.Unmarshal([]byte(sn.Code), &cfg); err != nil {
				t.Fatalf("Cursor JSON 无效: %v", err)
			}
			name := workerServerName
			if i == 1 {
				name = managerServerName
			}
			slate, ok := cfg.MCPServers[name]
			if !ok || slate.URL == "" || len(slate.Headers) != 0 {
				t.Errorf("Cursor 配置应对 %s 只有 url: %+v", name, cfg)
			}
		}
	}
}
