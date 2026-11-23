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
			if len(h.MCP) != 1 || len(h.Skill) != 1 {
				t.Errorf("%s：应有 1 个 MCP 片段和 1 个 skill 片段", h.ID)
			}
			for _, sn := range slices.Concat(h.MCP, h.Skill) {
				if sn.Kind == "" || sn.Title == "" || sn.Code == "" || (sn.Lang != "sh" && sn.Lang != "json") {
					t.Errorf("%s：步骤不完整: %+v", h.ID, sn)
				}
			}
			if h.MCP[0].Kind != "worker" {
				t.Errorf("%s：MCP kind 应为 worker", h.ID)
			}
			if h.Skill[0].Kind != "skill" {
				t.Errorf("%s：skill kind 应为 skill", h.ID)
			}
			for _, sn := range h.MCP {
				if strings.Contains(strings.ToLower(sn.Code), "token") || strings.Contains(sn.Code, "Authorization") || strings.Contains(sn.Code, "role=manager") {
					t.Errorf("%s：默认 MCP 步骤不应含令牌或旧角色参数: %s", h.ID, sn.Code)
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
				if sn.Lang != "sh" || h.ID == "claude-code" && sn.Kind == "worker" {
					continue
				}
				if strings.Contains(sn.Code, base) && !strings.Contains(sn.Code, "'"+base) {
					t.Errorf("%s：shell 里地址应加引号: %s", h.ID, sn.Code)
				}
			}
		}
	}
}

// hostMCP 返回指定 host 的 MCP 片段。
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

func TestWorkerLongWatchConfiguration(t *testing.T) {
	for _, base := range guideBases {
		for _, passcode := range []bool{false, true} {
			g := buildGuide(base, passcode)
			claude := hostMCP(t, g, "claude-code")
			config, ok := strings.CutPrefix(claude[0].Code, "claude mcp add-json --scope user slate '")
			if !ok || !strings.HasSuffix(config, "'") {
				t.Fatalf("Claude 工作视图应是单引号引用的 JSON: %q", claude[0].Code)
			}
			var worker struct {
				Type    string
				URL     string
				Timeout int
				Headers map[string]string
			}
			if err := json.Unmarshal([]byte(strings.TrimSuffix(config, "'")), &worker); err != nil {
				t.Fatalf("Claude 工作视图配置无效: %v", err)
			}
			if worker.Type != "http" || worker.URL != base+"/mcp?name=claude-code" || worker.Timeout != 3660000 {
				t.Errorf("Claude 工作视图长等待配置错误: %+v", worker)
			}
			if (worker.Headers["Authorization"] == "Bearer "+passcodePlaceholder) != passcode {
				t.Errorf("Claude 工作视图口令配置错误: %+v", worker)
			}
			codex := hostMCP(t, g, "codex")
			if !strings.Contains(codex[0].Code, "tool_timeout_sec = 3660") {
				t.Error("Codex 接入应保留长等待超时")
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
		for _, sn := range h.MCP {
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
			slate, ok := cfg.MCPServers[name]
			if !ok || slate.URL == "" || len(slate.Headers) != 0 {
				t.Errorf("Cursor 配置应对 %s 只有 url: %+v", name, cfg)
			}
		}
	}
}

// TestWatchGuide 验证长期挂着的内容：一行提示词带看板占位符并指向 get_board 的 watch_prompt；
// 每个 host 都有监听步骤。Claude Code 在 screen 中用 /loop，Codex 用 Python 按事件启动。
func TestWatchGuide(t *testing.T) {
	g := buildGuide(guideBases[0], false)
	if !strings.Contains(g.LoopPrompt, boardPlaceholder) || !strings.Contains(g.LoopPrompt, "watch_prompt") || strings.Contains(g.LoopPrompt, "\n") {
		t.Errorf("一行提示词应该是单行、带看板占位符并指向 watch_prompt: %q", g.LoopPrompt)
	}
	if len(g.WatchNotes) == 0 {
		t.Errorf("说明不能为空")
	}
	// 说明要讲清楚持续监听走 wait_for_updates 推送，不能再把 loop 说成定时叫醒来拉任务
	all := strings.Join(g.WatchNotes, "\n")
	if !strings.Contains(all, "wait_for_updates") || !strings.Contains(all, "检查监听") || strings.Contains(all, "定时叫醒") {
		t.Errorf("说明应该讲 wait_for_updates 推送和 loop 检查监听: %q", all)
	}
	for _, h := range g.Hosts {
		if len(h.Watch) < 3 || !strings.Contains(h.Watch[0].Code, "slate://scripts/slate-watch.py") ||
			!strings.Contains(h.Watch[1].Code, `python "slate-watch.py"`) ||
			!strings.Contains(h.Watch[1].Code, g.MCPURL) || !strings.Contains(h.Watch[2].Code, "--resolve done") {
			t.Errorf("%s 缺少获取、启动和恢复 Python 监听器的步骤: %+v", h.ID, h.Watch)
			continue
		}
		host := h.ID
		if host == "claude-code" {
			host = "claude"
		}
		if !strings.Contains(h.Watch[1].Code, "--host "+host) ||
			!strings.Contains(h.Watch[1].Code, `--name "`+h.ID+`"`) {
			t.Errorf("%s 的命令未绑定宿主和工作身份: %q", h.ID, h.Watch[1].Code)
		}
		for _, option := range []string{"--supervise enable", "--supervise status", "--supervise disable", "休眠"} {
			if !strings.Contains(h.Watch[2].Code, option) {
				t.Errorf("%s 的 Python 监听器说明缺少 %q", h.ID, option)
			}
		}
	}
	claude := g.Hosts[slices.IndexFunc(g.Hosts, func(h Host) bool { return h.ID == "claude-code" })]
	if len(claude.Watch) != 5 || !strings.Contains(claude.Watch[3].Code, "screen -S") ||
		!strings.Contains(claude.Watch[3].Code, "CLAUDE_CODE_MCP_AUTO_BACKGROUND_MS=0") ||
		!strings.Contains(claude.Watch[3].Code, "/loop 5m ") ||
		!strings.Contains(claude.Watch[3].Code, "空闲时用 3600 秒") ||
		!strings.Contains(claude.Watch[4].Code, "screen -r") {
		t.Error("Claude 应保留前台 loop 和恢复命令")
	}
	cursor := g.Hosts[slices.IndexFunc(g.Hosts, func(h Host) bool { return h.ID == "cursor" })]
	if len(cursor.Watch) != 4 || cursor.Watch[3].Code != "/loop 5m "+g.LoopPrompt {
		t.Errorf("Cursor 应保留 /loop 命令: %+v", cursor.Watch)
	}
	markdown := renderMarkdown(g, false)
	if strings.Contains(markdown, "role=manager") || !strings.Contains(markdown, "在当前会话开启管理工具") {
		t.Error("安装文档应说明按需开启管理工具")
	}
	for _, want := range []string{"--host opencode", "--host pi", "--host on-my-pi", "--command-json", "--supervise enable", "LaunchAgent"} {
		if !strings.Contains(markdown, want) {
			t.Errorf("安装文档缺少 %q", want)
		}
	}
}
