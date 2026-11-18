package connectapi

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// guideBases 是测试用的几种服务地址：本机、反向代理后的域名、IPv6（方括号在 shell 里会被当成通配符，必须加引号）。
var guideBases = []string{"http://localhost:7070", "https://kanban.example.com", "http://[::1]:7070"}

// TestBuildGuideShape 验证说明的整体结构：地址、令牌变量名正确，三种 host 一种不少，
// 每一段步骤都有标题、语言和内容，并且都带着服务地址或技能文件名。
func TestBuildGuideShape(t *testing.T) {
	for _, base := range guideBases {
		g := buildGuide(base)
		if g.BaseURL != base || g.MCPURL != base+"/mcp" || g.InstallURL != base+"/install.md" ||
			g.SkillURL != base+"/skills/slate-kanban/SKILL.md" || g.TokenEnv != "SLATE_TOKEN" {
			t.Fatalf("地址或令牌变量名不对: %+v", g)
		}
		ids := make([]string, 0, len(g.Hosts))
		for _, h := range g.Hosts {
			ids = append(ids, h.ID)
			if h.Name == "" || len(h.MCP) == 0 || len(h.Skill) == 0 {
				t.Errorf("%s：名称、MCP 步骤、skill 步骤一个都不能少: %+v", h.ID, h)
			}
			for _, sn := range slices.Concat(h.MCP, h.Skill) {
				if sn.Title == "" || sn.Code == "" || (sn.Lang != "sh" && sn.Lang != "json") {
					t.Errorf("%s：步骤的标题、语言、内容不完整: %+v", h.ID, sn)
				}
			}
			for _, sn := range h.MCP {
				if !strings.Contains(sn.Code, base+"/mcp") || !strings.Contains(sn.Code, "SLATE_TOKEN") {
					t.Errorf("%s：MCP 步骤里要有 MCP 地址和令牌变量名: %s", h.ID, sn.Code)
				}
			}
			for _, sn := range h.Skill {
				if !strings.Contains(sn.Code, base+"/skills/slate-kanban/SKILL.md") {
					t.Errorf("%s：skill 步骤里要有下载地址: %s", h.ID, sn.Code)
				}
			}
		}
		if !slices.Equal(ids, []string{"claude-code", "cursor", "codex"}) {
			t.Errorf("host 应该依次是 claude-code、cursor、codex，实际 %v", ids)
		}
	}
}

// hostByID 取出指定的 host，找不到就让测试终止。
func hostByID(t *testing.T, g Guide, id string) Host {
	t.Helper()
	for _, h := range g.Hosts {
		if h.ID == id {
			return h
		}
	}
	t.Fatalf("没有 host %s", id)
	return Host{}
}

// TestGuideNeverHoldsToken 验证令牌不会落进文件：所有写了 Authorization 的步骤，令牌都只是对环境变量的引用，
// 并且 Claude Code 的命令把引用放在单引号里，让 shell 原样传给它（不然 shell 会当场展开，明文就写进它的配置了）。
func TestGuideNeverHoldsToken(t *testing.T) {
	for _, base := range guideBases {
		g := buildGuide(base)
		for _, h := range g.Hosts {
			for _, sn := range h.MCP {
				if strings.Contains(sn.Code, "Authorization") && !strings.Contains(sn.Code, "${SLATE_TOKEN}") && !strings.Contains(sn.Code, "${env:SLATE_TOKEN}") {
					t.Errorf("%s：Authorization 必须引用环境变量: %s", h.ID, sn.Code)
				}
			}
		}
		claude := hostByID(t, g, "claude-code").MCP[0].Code
		if !strings.Contains(claude, "'Authorization: Bearer ${SLATE_TOKEN}'") || !strings.Contains(claude, "--scope user") {
			t.Errorf("Claude Code 的命令要用单引号包住令牌引用，并装在用户级: %s", claude)
		}
		codex := hostByID(t, g, "codex").MCP[0].Code
		if !strings.Contains(codex, "--bearer-token-env-var SLATE_TOKEN") || strings.Contains(codex, "$") {
			t.Errorf("Codex 的命令只传环境变量名，不能带 $: %s", codex)
		}
	}
}

// TestGuideQuotesURLs 验证 shell 命令里的地址都加了单引号：IPv6 地址里的方括号在 zsh 里是通配符，不加引号会直接报错。
func TestGuideQuotesURLs(t *testing.T) {
	for _, base := range guideBases {
		g := buildGuide(base)
		for _, h := range g.Hosts {
			for _, sn := range slices.Concat(h.MCP, h.Skill) {
				if sn.Lang != "sh" {
					continue
				}
				for _, url := range []string{g.MCPURL, g.SkillURL} {
					if strings.Contains(sn.Code, url) && !strings.Contains(sn.Code, "'"+url+"'") {
						t.Errorf("%s：命令里的 %s 要加单引号: %s", h.ID, url, sn.Code)
					}
				}
			}
		}
	}
}

// TestGuideCursorConfigIsJSON 验证 Cursor 的配置片段是合法的 JSON，并且结构是 Cursor 要求的：
// mcpServers 下的 slate 有 url 和 headers，令牌用 ${env:变量名} 引用。用户是直接复制进配置文件的，写坏了就整个失效。
func TestGuideCursorConfigIsJSON(t *testing.T) {
	for _, base := range guideBases {
		code := hostByID(t, buildGuide(base), "cursor").MCP[0]
		if code.Lang != "json" {
			t.Fatalf("Cursor 的 MCP 步骤应该是 json: %+v", code)
		}
		var cfg struct {
			MCPServers map[string]struct {
				URL     string            `json:"url"`
				Headers map[string]string `json:"headers"`
			} `json:"mcpServers"`
		}
		if err := json.Unmarshal([]byte(code.Code), &cfg); err != nil {
			t.Fatalf("Cursor 配置不是合法 JSON: %v\n%s", err, code.Code)
		}
		slate, ok := cfg.MCPServers["slate"]
		if !ok || len(cfg.MCPServers) != 1 || slate.URL != base+"/mcp" || slate.Headers["Authorization"] != "Bearer ${env:SLATE_TOKEN}" {
			t.Errorf("Cursor 配置的内容不对: %+v", cfg)
		}
	}
}

// TestGuideSkillTargets 验证三种 host 的 skill 都装到各自约定的目录，并且命令会先建目录再下载。
func TestGuideSkillTargets(t *testing.T) {
	want := map[string]string{
		"claude-code": "~/.claude/skills/slate-kanban",
		"cursor":      "~/.cursor/skills/slate-kanban",
		"codex":       "~/.codex/skills/slate-kanban",
	}
	g := buildGuide(guideBases[0])
	for id, dir := range want {
		code := hostByID(t, g, id).Skill[0].Code
		if !strings.Contains(code, "mkdir -p "+dir+" ") || !strings.Contains(code, "-o "+dir+"/SKILL.md") || !strings.Contains(code, "curl -fsSL") {
			t.Errorf("%s：skill 的安装命令不对: %s", id, code)
		}
	}
}
