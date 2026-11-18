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
		"重新打开 host 的会话，让新装的 MCP 和 skill 生效。让 agent 调用 whoami：" +
		"返回的名称应该和网址里 name 一致，工作视图类型是 agent。确认之后，让它按 " + skillName + " skill 开始工作。\n\n"
	faqSection = "## 常见问题\n\n" +
		"- 连接返回 400：检查 name 格式（小写字母数字开头，最长 64，不能用 human 或 system）。\n" +
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
