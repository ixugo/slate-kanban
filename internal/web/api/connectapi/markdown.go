package connectapi

import (
	"fmt"
	"strings"
)

const (
	introFormat = `# 把 agent 接到 Slate 看板

- 服务地址：%s
- MCP 地址：%s
- 这份说明：%s

按顺序做。每个 agent 用自己的名字连 MCP（网址参数 name=你的名称）。

`
	mcpIntro   = "## 1. 安装 MCP\n\n按你使用的 host 选一种。工作视图日常领任务；只有要建看板、改列、录任务时才装管理视图（role=manager）。\n\n"
	skillIntro = "## 2. 安装 skill\n\n" +
		"skill 是 agent 使用看板的操作手册：怎么确认项目路径、怎么守候、怎么开分支做事、做完怎么交接。" +
		"装在 host 的 skills 目录里，agent 需要的时候会自己加载。\n\n"
	verifyStep = "## 3. 验证\n\n" +
		"装完 MCP 和 skill 之后，在 host 里新开一个会话（已经打开的会话不会获得新装的 MCP 和 skill）。让 agent 调用 whoami：" +
		"返回的名称应该和网址里 name 一致，工作视图类型是 agent。确认之后，让它按 " + skillName + " skill 开始工作。\n\n" +
		"不想开会话时，也可以先在终端看一眼：\n\n" +
		"- Claude Code：运行 `claude mcp list`，slate 那一行应该显示 `✔ Connected`。\n" +
		"- Codex：运行 `codex mcp list`，能列出 slate 就说明配置写进去了。这个命令不会真的去连服务，" +
		"所以 Auth 一栏显示 `Unsupported` 是正常的（Slate 不用 OAuth）；启用口令后会显示 `Bearer token`。\n" +
		"- Cursor：命令行版运行 `agent mcp list`，slate 那一行显示 `ready` 就是连上了。写在项目里的 `.cursor/mcp.json` 要先批准：`agent mcp enable slate`；`~/.cursor/mcp.json` 里的服务不用。" +
		"图形界面里，按 Cursor 官方文档，改完 `mcp.json` 要重启 Cursor，再到设置的 MCP 页面确认 slate 已启用。\n\n"
	faqSection = "## 常见问题\n\n" +
		"- 连接返回 400：检查 name 格式（小写字母数字开头，最长 64，不能用 human 或 system）。\n" +
		"- Claude Code 添加成功时打印的地址里没有 `?name=…`，那只是提示文字省略了查询参数，配置里存的是完整地址；以 whoami 返回的名称为准。\n" +
		"- skill 要更新：服务升级之后，重新执行第 2 步里的命令。\n" +
		"- 管理视图只在建项目、改列、改规则、录入任务时使用，日常干活用工作视图。\n"
)

// renderMarkdown 把接入说明渲染成 Markdown。
func renderMarkdown(g Guide, passcodeRequired bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, introFormat, g.BaseURL, g.MCPURL, g.InstallURL)
	b.WriteString(mcpIntro)
	writeHostSteps(&b, g.Hosts, func(h Host) []Snippet { return h.MCP })
	b.WriteString(skillIntro)
	writeHostSteps(&b, g.Hosts, func(h Host) []Snippet { return h.Skill })
	b.WriteString(verifyStep)
	b.WriteString(faqSection)
	if passcodeRequired {
		b.WriteString("- 服务启用了访问口令：网页需先登录；MCP 与 curl 要在请求头带 `Authorization: Bearer <口令>`（把 `<口令>` 换成你在配置里设置的值，不要写进仓库）。\n")
		b.WriteString("- 漏带口令时服务返回 401，各 host 的表现不一样：Claude Code 的 `claude mcp list` 显示 `✘ Failed to connect`（后面的文字不固定）；" +
			"Cursor 命令行的 `agent mcp list` 显示 `Error: Connection failed`；" +
			"Codex 不报错，只是 slate 的工具不出现，启动日志里有 `AuthRequired`。补上口令后重新开一个会话。\n")
		b.WriteString("- Cursor 的 `mcp.json` 不想写明文口令时，可以把请求头写成 `\"Authorization\": \"Bearer ${env:SLATE_PASSCODE}\"`，" +
			"再设置环境变量 `SLATE_PASSCODE`（命令行版已实测可用，图形界面按 Cursor 官方文档也支持）。\n")
	}
	return b.String()
}

// writeHostSteps 按 host 依次写出一组步骤。
func writeHostSteps(b *strings.Builder, hosts []Host, pick func(Host) []Snippet) {
	for _, h := range hosts {
		fmt.Fprintf(b, "### %s\n\n", h.Name)
		for _, sn := range pick(h) {
			fmt.Fprintf(b, "%s\n\n", sn.Title)
			fmt.Fprintf(b, "```%s\n%s\n```\n\n", sn.Lang, sn.Code)
		}
	}
}
