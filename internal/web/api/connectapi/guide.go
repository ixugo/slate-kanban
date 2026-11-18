package connectapi

import "fmt"

const (
	// tokenEnv 是存放令牌的环境变量名。所有命令都只引用它，令牌明文不会写进任何配置文件。
	tokenEnv = "SLATE_TOKEN"
	// serverName 是 MCP 服务在各个 host 里登记的名字。
	serverName = "slate"
	// skillName 是 skill 的名字，也是它在 host 的 skills 目录里的文件夹名。
	skillName = "slate-kanban"
	// cursorConfig 是 Cursor 的 mcp.json 片段：令牌用 ${env:变量名} 引用。依次填入：服务名、MCP 地址、令牌变量名。
	cursorConfig = `{
  "mcpServers": {
    %q: {
      "url": %q,
      "headers": {
        "Authorization": "Bearer ${env:%s}"
      }
    }
  }
}`
)

// Snippet 是一段可以直接复制的命令或配置。
type Snippet struct {
	Title string `json:"title"`
	Lang  string `json:"lang"` // sh 或 json，网页据此高亮
	Code  string `json:"code"`
}

// Host 是一种 agent 宿主的接入步骤。
type Host struct {
	ID    string    `json:"id"`
	Name  string    `json:"name"`
	MCP   []Snippet `json:"mcp"`   // 安装 MCP 的步骤
	Skill []Snippet `json:"skill"` // 安装 skill 的步骤
}

// Guide 是完整的接入说明。/install.md 是它的 Markdown 形式，/api/v1/connect 是它的 JSON 形式，
// 两者出自同一份数据，网页不用再自己拼命令。令牌不在里面：所有命令都引用环境变量。
type Guide struct {
	BaseURL    string `json:"base_url"`
	MCPURL     string `json:"mcp_url"`
	InstallURL string `json:"install_url"`
	SkillURL   string `json:"skill_url"`
	TokenEnv   string `json:"token_env"`
	Hosts      []Host `json:"hosts"`
}

// buildGuide 按服务地址生成接入说明。base 必须是 baseURL 校验过的地址：它会被直接写进命令里，
// shell 命令里的地址都加了单引号，是因为 IPv6 地址里的方括号在 zsh 里是通配符。
func buildGuide(base string) Guide {
	g := Guide{
		BaseURL:    base,
		MCPURL:     base + "/mcp",
		InstallURL: base + "/install.md",
		SkillURL:   base + "/skills/" + skillName + "/SKILL.md",
		TokenEnv:   tokenEnv,
	}
	g.Hosts = []Host{claudeCodeHost(g), cursorHost(g), codexHost(g)}
	return g
}

// claudeCodeHost 是 Claude Code 的接入步骤。令牌引用放在单引号里：shell 不会展开它，
// Claude Code 原样记录 ${SLATE_TOKEN}，每次加载配置时才读环境变量，令牌明文不会落进它的配置文件。
func claudeCodeHost(g Guide) Host {
	return Host{
		ID:   "claude-code",
		Name: "Claude Code",
		MCP: []Snippet{{
			Title: "添加 MCP 服务，对你的所有项目生效（令牌的引用要用单引号包住，不要让 shell 把它展开）",
			Lang:  "sh",
			Code: fmt.Sprintf("claude mcp add --transport http --scope user %s '%s' --header 'Authorization: Bearer ${%s}'",
				serverName, g.MCPURL, g.TokenEnv),
		}},
		Skill: skillSnippets("~/.claude/skills", g),
	}
}

// cursorHost 是 Cursor 的接入步骤：把配置合并进 mcp.json，令牌用 ${env:变量名} 引用。
func cursorHost(g Guide) Host {
	return Host{
		ID:   "cursor",
		Name: "Cursor",
		MCP: []Snippet{{
			Title: "合并进 ~/.cursor/mcp.json（只给当前项目用，就写进项目里的 .cursor/mcp.json）",
			Lang:  "json",
			Code:  fmt.Sprintf(cursorConfig, serverName, g.MCPURL, g.TokenEnv),
		}},
		Skill: skillSnippets("~/.cursor/skills", g),
	}
}

// codexHost 是 Codex 的接入步骤：命令只记录令牌所在的环境变量名。
func codexHost(g Guide) Host {
	return Host{
		ID:   "codex",
		Name: "Codex",
		MCP: []Snippet{{
			Title: "添加 MCP 服务（先按第 1 步设置好环境变量）",
			Lang:  "sh",
			Code:  fmt.Sprintf("codex mcp add %s --url '%s' --bearer-token-env-var %s", serverName, g.MCPURL, g.TokenEnv),
		}},
		Skill: skillSnippets("~/.codex/skills", g),
	}
}

// skillSnippets 生成把 skill 装进某个 skills 目录的步骤：先建目录，再把服务上的最新版下载下来。
// 服务升级之后重新执行一次，skill 就更新了。
func skillSnippets(skillsDir string, g Guide) []Snippet {
	target := skillsDir + "/" + skillName
	return []Snippet{{
		Title: "把 skill 下载到 " + skillsDir + "（服务升级之后重新执行一次就会更新）",
		Lang:  "sh",
		Code:  fmt.Sprintf("mkdir -p %s && curl -fsSL '%s' -o %s/SKILL.md", target, g.SkillURL, target),
	}}
}
