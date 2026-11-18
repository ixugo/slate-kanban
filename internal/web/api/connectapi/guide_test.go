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
		g := buildGuide(base)
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
		g := buildGuide(base)
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

func TestGuideCursorConfigIsJSON(t *testing.T) {
	base := guideBases[0]
	g := buildGuide(base)
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
