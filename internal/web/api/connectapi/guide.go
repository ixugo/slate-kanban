package connectapi

import "fmt"

const (
	// workerServerName 是各 host 工作视图登记的 MCP 服务名。
	workerServerName = "slate"
	// skillName 是 skill 的名字，也是它在 host 的 skills 目录里的文件夹名。
	skillName = "slate-kanban"
	// passcodePlaceholder 是文档里写的口令占位符，绝不填入真实口令。
	passcodePlaceholder = "<口令>"
	// codexPasscodeEnv 是 Codex 读取访问口令用的环境变量名；口令只放在环境变量里，不写进 Codex 的配置文件。
	codexPasscodeEnv = "SLATE_PASSCODE"
	// 工作视图支持长等待，客户端超时多留一分钟用来完成请求和读取响应。
	codexWatchTimeoutSeconds = 3660
	claudeWatchTimeoutMillis = codexWatchTimeoutSeconds * 1000
	// cursorConfig 是 Cursor 的 mcp.json 片段：只需 url，不带令牌。
	cursorConfig = `{
  "mcpServers": {
    %q: {
      "url": %q
    }
  }
}`
	// cursorConfigWithPasscode 在启用访问口令时给 Cursor 增加 Authorization 请求头占位。
	cursorConfigWithPasscode = `{
  "mcpServers": {
    %q: {
      "url": %q,
      "headers": {
        "Authorization": "Bearer ` + passcodePlaceholder + `"
      }
    }
  }
}`
)

// Snippet 是一段可以直接复制的命令或配置。
type Snippet struct {
	Kind  string `json:"kind"`
	Title string `json:"title"`
	Lang  string `json:"lang"`
	Code  string `json:"code"`
}

// Host 是一种 agent 宿主的接入步骤。
type Host struct {
	ID    string    `json:"id"`
	Name  string    `json:"name"`
	MCP   []Snippet `json:"mcp"`
	Skill []Snippet `json:"skill"`
	// Watch 是长期挂着的步骤：怎么在这个 host 里一直守候看板，以及断了怎么恢复。
	Watch []Snippet `json:"watch"`
}

// Guide 是完整的接入说明。/install.md 是它的 Markdown 形式，/api/v1/connect 是它的 JSON 形式。
type Guide struct {
	BaseURL    string `json:"base_url"`
	MCPURL     string `json:"mcp_url"`
	InstallURL string `json:"install_url"`
	SkillURL   string `json:"skill_url"`
	Hosts      []Host `json:"hosts"`
	// LoopPrompt 是用户唯一要贴的一行守候提示词，里面的看板名是占位符；完整提示词由 MCP 的 get_board 返回。
	LoopPrompt string `json:"loop_prompt"`
	// WatchNotes 是长期挂着的注意事项，和具体 host 无关。
	WatchNotes []string `json:"watch_notes"`
}

// buildGuide 按服务地址生成接入说明；shell 里的地址加单引号，避免 IPv6 方括号被 zsh 当通配符。
func buildGuide(base string, passcodeRequired bool) Guide {
	g := Guide{
		BaseURL:    base,
		MCPURL:     base + "/mcp",
		InstallURL: base + "/install.md",
		SkillURL:   base + "/skills/" + skillName + "/SKILL.md",
		LoopPrompt: loopPrompt,
		WatchNotes: watchNotes,
	}
	g.Hosts = []Host{
		claudeCodeHost(g, passcodeRequired),
		cursorHost(g, passcodeRequired),
		codexHost(g, passcodeRequired),
	}
	return g
}

// mcpURL 用各宿主的默认名称拼接 MCP 地址。
func mcpURL(base, agentName string) string {
	return fmt.Sprintf("%s/mcp?name=%s", base, agentName)
}

// passcodeTitleSuffix 在启用口令时追加到步骤标题，提示用户替换占位符。
func passcodeTitleSuffix(passcodeRequired bool) string {
	if !passcodeRequired {
		return ""
	}
	return "（把 " + passcodePlaceholder + " 换成你在配置里设置的访问口令）"
}

// claudeCodeHost 是 Claude Code 的接入步骤。
func claudeCodeHost(g Guide, passcodeRequired bool) Host {
	worker := mcpURL(g.BaseURL, "claude-code")
	suffix := passcodeTitleSuffix(passcodeRequired)
	var workerHeaders string
	if passcodeRequired {
		workerHeaders = `,"headers":{"Authorization":"Bearer ` + passcodePlaceholder + `"}`
	}
	workerConfig := fmt.Sprintf(`{"type":"http","url":%q,"timeout":%d%s}`, worker, claudeWatchTimeoutMillis, workerHeaders)
	return Host{
		ID:   "claude-code",
		Name: "Claude Code",
		MCP: []Snippet{
			{
				Kind:  "worker",
				Title: "添加 Slate MCP（含长等待超时。已登记的用户可重装这一项）" + suffix,
				Lang:  "sh",
				Code:  fmt.Sprintf("claude mcp add-json --scope user %s '%s'", workerServerName, workerConfig),
			},
		},
		Skill: skillSnippets("~/.claude/skills", g),
		Watch: claudeWatch(g.MCPURL),
	}
}

// cursorHost 是 Cursor 的接入步骤。
func cursorHost(g Guide, passcodeRequired bool) Host {
	worker := mcpURL(g.BaseURL, "cursor")
	suffix := passcodeTitleSuffix(passcodeRequired)
	tmpl := cursorConfig
	if passcodeRequired {
		tmpl = cursorConfigWithPasscode
	}
	return Host{
		ID:   "cursor",
		Name: "Cursor",
		MCP: []Snippet{
			{
				Kind:  "worker",
				Title: "合并进 ~/.cursor/mcp.json" + suffix,
				Lang:  "json",
				Code:  fmt.Sprintf(tmpl, workerServerName, worker),
			},
		},
		Skill: skillSnippets("~/.cursor/skills", g),
		Watch: cursorWatch(g.MCPURL),
	}
}

// codexWithPasscode 在 codex mcp add 命令前后补上口令的配置：先把口令放进环境变量，
// 再用 --bearer-token-env-var 让 Codex 每次连接时从该环境变量读取并作为 Bearer 头发送。
func codexWithPasscode(addCmd string) string {
	return fmt.Sprintf("export %s='%s'   # 写进 ~/.zshrc 等 shell 配置，Codex 每次启动都要能读到它\n%s --bearer-token-env-var %s",
		codexPasscodeEnv, passcodePlaceholder, addCmd, codexPasscodeEnv)
}

// codexHost 是 Codex 的接入步骤。
func codexHost(g Guide, passcodeRequired bool) Host {
	worker := mcpURL(g.BaseURL, "codex")
	suffix := passcodeTitleSuffix(passcodeRequired)
	workerCode := fmt.Sprintf("codex mcp add %s --url '%s'", workerServerName, worker)
	if passcodeRequired {
		workerCode = codexWithPasscode(workerCode)
	}
	workerCode += fmt.Sprintf("\n# 在 ~/.codex/config.toml 已有的 [mcp_servers.%s] 段内添加或更新：\n# tool_timeout_sec = %d",
		workerServerName, codexWatchTimeoutSeconds)
	return Host{
		ID:   "codex",
		Name: "Codex",
		MCP: []Snippet{
			{
				Kind:  "worker",
				Title: "添加 Slate MCP" + suffix,
				Lang:  "sh",
				Code:  workerCode,
			},
		},
		Skill: skillSnippets("~/.codex/skills", g),
		Watch: eventWatch(g.MCPURL, "codex", "codex"),
	}
}

// skillSnippets 生成把 skill 装进某个 skills 目录的步骤。
func skillSnippets(skillsDir string, g Guide) []Snippet {
	target := skillsDir + "/" + skillName
	return []Snippet{{
		Kind:  "skill",
		Title: "把 skill 下载到 " + skillsDir + "（服务升级之后重新执行一次就会更新）",
		Lang:  "sh",
		Code:  fmt.Sprintf("mkdir -p %s && curl -fsSL '%s' -o %s/SKILL.md", target, g.SkillURL, target),
	}}
}
