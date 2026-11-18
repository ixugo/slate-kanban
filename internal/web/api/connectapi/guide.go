package connectapi

import "fmt"

const (
	// workerServerName 是各 host 工作视图登记的 MCP 服务名。
	workerServerName = "slate"
	// managerServerName 是管理视图登记的 MCP 服务名。
	managerServerName = "slate-manager"
	// skillName 是 skill 的名字，也是它在 host 的 skills 目录里的文件夹名。
	skillName = "slate-kanban"
	// cursorConfig 是 Cursor 的 mcp.json 片段：只需 url，不带令牌。
	cursorConfig = `{
  "mcpServers": {
    %q: {
      "url": %q
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
}

// Guide 是完整的接入说明。/install.md 是它的 Markdown 形式，/api/v1/connect 是它的 JSON 形式。
type Guide struct {
	BaseURL    string `json:"base_url"`
	MCPURL     string `json:"mcp_url"`
	InstallURL string `json:"install_url"`
	SkillURL   string `json:"skill_url"`
	Hosts      []Host `json:"hosts"`
}

// buildGuide 按服务地址生成接入说明；shell 里的地址加单引号，避免 IPv6 方括号被 zsh 当通配符。
func buildGuide(base string) Guide {
	g := Guide{
		BaseURL:    base,
		MCPURL:     base + "/mcp",
		InstallURL: base + "/install.md",
		SkillURL:   base + "/skills/" + skillName + "/SKILL.md",
	}
	g.Hosts = []Host{claudeCodeHost(g), cursorHost(g), codexHost(g)}
	return g
}

// mcpURL 拼工作视图或管理视图的 MCP 地址；每个 host 用不同的默认 name。
func mcpURL(base, agentName string, manager bool) string {
	if manager {
		return fmt.Sprintf("%s/mcp?name=%s-manager&role=manager", base, agentName)
	}
	return fmt.Sprintf("%s/mcp?name=%s", base, agentName)
}

// claudeCodeHost 是 Claude Code 的接入步骤。
func claudeCodeHost(g Guide) Host {
	worker := mcpURL(g.BaseURL, "claude-code", false)
	manager := mcpURL(g.BaseURL, "claude-code", true)
	return Host{
		ID:   "claude-code",
		Name: "Claude Code",
		MCP: []Snippet{
			{
				Kind:  "worker",
				Title: "添加工作视图 MCP（agent 日常领任务、改卡片）",
				Lang:  "sh",
				Code:  fmt.Sprintf("claude mcp add --transport http --scope user %s '%s'", workerServerName, worker),
			},
			{
				Kind:  "manager",
				Title: "添加管理视图 MCP（建看板、改列、录任务时再用）",
				Lang:  "sh",
				Code:  fmt.Sprintf("claude mcp add --transport http --scope user %s '%s'", managerServerName, manager),
			},
		},
		Skill: skillSnippets("~/.claude/skills", g),
	}
}

// cursorHost 是 Cursor 的接入步骤。
func cursorHost(g Guide) Host {
	worker := mcpURL(g.BaseURL, "cursor", false)
	manager := mcpURL(g.BaseURL, "cursor", true)
	return Host{
		ID:   "cursor",
		Name: "Cursor",
		MCP: []Snippet{
			{
				Kind:  "worker",
				Title: "合并进 ~/.cursor/mcp.json（工作视图）",
				Lang:  "json",
				Code:  fmt.Sprintf(cursorConfig, workerServerName, worker),
			},
			{
				Kind:  "manager",
				Title: "合并进 ~/.cursor/mcp.json（管理视图，服务名 slate-manager）",
				Lang:  "json",
				Code:  fmt.Sprintf(cursorConfig, managerServerName, manager),
			},
		},
		Skill: skillSnippets("~/.cursor/skills", g),
	}
}

// codexHost 是 Codex 的接入步骤。
func codexHost(g Guide) Host {
	worker := mcpURL(g.BaseURL, "codex", false)
	manager := mcpURL(g.BaseURL, "codex", true)
	return Host{
		ID:   "codex",
		Name: "Codex",
		MCP: []Snippet{
			{
				Kind:  "worker",
				Title: "添加工作视图 MCP",
				Lang:  "sh",
				Code:  fmt.Sprintf("codex mcp add %s --url '%s'", workerServerName, worker),
			},
			{
				Kind:  "manager",
				Title: "添加管理视图 MCP（服务名 slate-manager）",
				Lang:  "sh",
				Code:  fmt.Sprintf("codex mcp add %s --url '%s'", managerServerName, manager),
			},
		},
		Skill: skillSnippets("~/.codex/skills", g),
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
