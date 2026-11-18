package connectapi

import (
	"fmt"
	"strings"
)

const (
	// introFormat 是说明的开头，依次填入：服务地址、MCP 地址、这份说明的地址。
	introFormat = `# 把 agent 接到 Slate 看板

- 服务地址：%s
- MCP 地址：%s
- 这份说明：%s

按顺序做。第 1 步需要人来做，其余的步骤 agent 可以代劳。

`
	// tokenStep 是第 1 步：创建令牌并放进环境变量。令牌只出现在人的终端里，不进任何文件。
	tokenStep = "## 1. 创建令牌（人来做）\n\n" +
		"在网页的 Agents 页创建一个 agent 令牌，名称写这个 agent 的名字（例如 claude）。令牌只显示一次，请立刻复制。\n\n" +
		"然后把令牌放进环境变量 `" + tokenEnv + "`。后面所有命令都只引用这个变量，令牌不会被写进任何会提交到仓库的文件：\n\n" +
		"```sh\nexport " + tokenEnv + "='<粘贴令牌>'\n```\n\n" +
		"想长期使用，就把这一行写进 `~/.zshrc` 或 `~/.bashrc`：启动 agent 的进程必须读得到这个变量。\n\n"
	// mcpIntro 和 skillIntro 是第 2、3 步的开头。
	mcpIntro   = "## 2. 安装 MCP\n\n按你使用的 host 选一种。\n\n"
	skillIntro = "## 3. 安装 skill\n\n" +
		"skill 是 agent 使用看板的操作手册：怎么确认项目路径、怎么守候、怎么开分支做事、做完怎么交接。" +
		"装在 host 的 skills 目录里，agent 需要的时候会自己加载。\n\n"
	// verifyStep 和 faqSection 是最后的验证步骤和常见问题。
	verifyStep = "## 4. 验证\n\n" +
		"重新打开 host 的会话，让新装的 MCP 和 skill 生效。然后让 agent 调用 `whoami`：" +
		"返回的名称应该是你创建令牌时写的名字，类型是 `agent`。确认之后，让它按 " + skillName + " skill 开始工作。\n\n"
	faqSection = "## 常见问题\n\n" +
		"- 连接返回 401：令牌不对或者已经被删除，也可能是启动 host 的进程读不到 `" + tokenEnv + "`。" +
		"Cursor 桌面版没有继承终端的环境变量时，请从设置好变量的终端里启动它，或者在 mcp.json 里直接写令牌（这个文件不要提交到仓库）。\n" +
		"- skill 要更新：服务升级之后，重新执行第 3 步里的命令。\n" +
		"- 令牌泄露了：在网页的 Agents 页删除它，再创建一个新的。\n"
)

// renderMarkdown 把接入说明渲染成 Markdown，给 agent 和人直接阅读：创建令牌、安装 MCP、安装 skill、验证。
// 步骤的文字是固定的，每个 host 的命令来自 Guide，和 JSON 形式是同一份数据。
func renderMarkdown(g Guide) string {
	var b strings.Builder
	fmt.Fprintf(&b, introFormat, g.BaseURL, g.MCPURL, g.InstallURL)
	b.WriteString(tokenStep)
	b.WriteString(mcpIntro)
	writeHostSteps(&b, g.Hosts, func(h Host) []Snippet { return h.MCP })
	b.WriteString(skillIntro)
	writeHostSteps(&b, g.Hosts, func(h Host) []Snippet { return h.Skill })
	b.WriteString(verifyStep)
	b.WriteString(faqSection)
	return b.String()
}

// writeHostSteps 按 host 依次写出一组步骤：每个 host 一个三级标题，每段步骤是一句说明加一个代码块。
// pick 决定写 host 的哪一组步骤（安装 MCP 或安装 skill）。
func writeHostSteps(b *strings.Builder, hosts []Host, pick func(Host) []Snippet) {
	for _, h := range hosts {
		fmt.Fprintf(b, "### %s\n\n", h.Name)
		for _, sn := range pick(h) {
			fmt.Fprintf(b, "%s\n\n```%s\n%s\n```\n\n", sn.Title, sn.Lang, sn.Code)
		}
	}
}
