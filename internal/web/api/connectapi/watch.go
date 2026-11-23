package connectapi

import (
	"fmt"
	"strings"

	"slate-kanban/skills"
)

const (
	// boardPlaceholder 是提示词里看板名的占位符；网页按用户选的看板替换它，install.md 里原样保留让用户自己改。
	boardPlaceholder = "<看板名>"
	// projectDirPlaceholder 是命令里项目目录的占位符，对应看板的 project_path。
	projectDirPlaceholder = "<项目目录>"
	// screenSession 是终端会话的名字，分离和回来时都用它。
	screenSession = "slate-agent"
	// loopInterval 是 loop 检查监听是否还活着的间隔，小于任务领取租约（默认 30 分钟），agent 掉线后任务也不会被误释放。
	loopInterval = "5m"
)

// loopPrompt 是用户唯一需要贴的一行提示词：agent 靠 wait_for_updates 长连接持续监听，
// loop 只是反复贴这一行来检查监听是否还活着；完整的守候提示词由 MCP 的 get_board 返回
// （watch_prompt，看板名已填好），所以这里只指路。
const loopPrompt = `守候 Slate 看板「` + boardPlaceholder + `」：调用 get_board 读取返回里的 watch_prompt，照它做。`

// longWatchPrompt 给按长等待配置启动的会话说明它可尝试一小时等待。
const longWatchPrompt = loopPrompt + " 已按接入说明设置长等待，空闲时用 3600 秒，连接提前断开时回退到短等待。"

// watchNotes 是和具体 host 无关的说明：持续监听怎么做到的、loop 负责什么、挂不住怎么办。
var watchNotes = []string{
	"Codex、Cursor、Claude Code、OpenCode、Pi、Oh My Pi 可通过 Python 监听器按事件启动本机调度。空闲时不调用模型。",
	"macOS 登录会话可启用 LaunchAgent，每五分钟检查监听进程。它独立于 Slate 服务和交互会话的 /loop 5m。",
	"OpenCode、Pi、Oh My Pi 先在客户端连接 Slate 工作视图，用 whoami 验证。复用下方启动命令，将 --host 换成 opencode、pi、on-my-pi，--name 使用 whoami 的名称。各客户端的原生配置方法见安装文档。",
	"省略 --host 时优先使用标准 Slate 身份对应的宿主，其次选唯一可用的 CLI。装了多个或使用自定义程序时，用 --host 明确选择，或用 --command-json 传本地 JSON 参数数组。自定义命令从标准输入接收提示词，成功结束时标准输出只写 SLATE_DISPATCH_DONE。",
	"Codex 默认用结构化回执确认单轮派发。旧版 Codex CLI 如果没有 --output-schema，可在启动时加 --codex-legacy-receipt。补验留言和交接已保存、领取已处置、子 agent 已结束时，即使卡片仍在等待人工，也可确认本轮已处理。",
	"持续监听走 MCP 协议：agent 反复调用 wait_for_updates，服务端在有更新时立即返回。配置长等待后，空闲时单次最多等 60 分钟。",
	"使用 Claude Code 或手动运行 Codex 长等待前，先按工作视图说明设置客户端超时，并给 /mcp 的反向代理或网关留出超过 60 分钟的响应等待时间。客户端、代理或线路不支持时用 50 秒或更短。手上有任务需检查子 agent 或续租时也用短等待。",
	"Claude Code 已按旧说明添加工作视图时，可先执行 claude mcp remove --scope user slate，再运行新的工作视图安装命令；自定义了地址或凭证的用户，保留现有设置并只更新该服务的 timeout。",
	"Claude Code 和 Cursor 的 /loop 只负责检查监听还活着没有：监听在时 agent 正忙在这次调用里，多数 host 的 loop 要等它空闲才会触发；监听断了（会话结束、上下文满、网络断）agent 空闲，loop 叫醒它，它重新挂上 wait_for_updates。重启时沿用同一个 ?name=，服务端记着的游标会把离线期间的更新补给它。",
	"停机超过租约（默认 30 分钟）时，任务会被释放，重新上线后重新领取。",
	"没人看着的时候，权限弹窗会把会话卡住：先把 slate 的工具和常用的 git、编辑操作设成“总是允许”。只有在容器或专用机器这类隔离环境里，才考虑关闭权限检查。",
}

// loopCommand 生成 host 里的 loop 命令：隔 loopInterval 检查一次监听。
func loopCommand(prompt string) string {
	return "/loop " + loopInterval + " " + prompt
}

// screenSnippet 生成“开一个关掉终端也不会停的会话再启动 host”的命令。launch 是 host 的启动命令。
func screenSnippet(launch string) Snippet {
	return Snippet{
		Kind:  "watch",
		Title: "长期挂着：在项目目录开一个终端会话（终端窗口关掉也不会停），再启动 agent 并执行 loop",
		Lang:  "sh",
		Code: strings.Join([]string{
			"cd " + projectDirPlaceholder + "   # 看板的项目目录，即 get_board 里的 project_path",
			"screen -S " + screenSession + "    # 或 tmux new -s " + screenSession + "；脱离会话按 Ctrl-A 再按 D",
			launch,
		}, "\n"),
	}
}

// resumeSnippet 生成“会话断了怎么回来”的命令。resume 是 host 接上最近对话的命令，为空表示只需回到终端会话。
func resumeSnippet(resume string) Snippet {
	lines := []string{"screen -r " + screenSession + "    # 会话还在就直接回去"}
	if resume != "" {
		lines = append(lines, resume)
	}
	return Snippet{
		Kind:  "resume",
		Title: "回来看看，或会话已经结束时恢复（接上对话后再执行一次 loop）",
		Lang:  "sh",
		Code:  strings.Join(lines, "\n"),
	}
}

// claudeWatch 是 Claude Code 的长期挂着步骤：在终端会话里启动 claude，再执行 /loop。
func claudeWatch(endpoint string) []Snippet {
	return append(eventWatch(endpoint, "claude", "claude-code"),
		screenSnippet("CLAUDE_CODE_MCP_AUTO_BACKGROUND_MS=0 claude   # 进入后执行：\n"+loopCommand(longWatchPrompt)),
		resumeSnippet("CLAUDE_CODE_MCP_AUTO_BACKGROUND_MS=0 claude --continue   # 接上最近一次对话"),
	)
}

// eventWatch 给用户提供由 Python 监听、按事件启动本机宿主的步骤。
func eventWatch(endpoint, host, name string) []Snippet {
	return []Snippet{
		{
			Kind:  "script",
			Title: "获取跨宿主监听脚本",
			Lang:  "text",
			Code: "在已接入 Slate 的客户端读取 MCP 资源 " + skills.WatchURI + "，将原文保存为 slate-watch.py。需要 Python 3.9+ 和可用的本机宿主 CLI。\n" +
				"先用 whoami 验证所选客户端的工作视图和 --name 一致。Cursor 需要安装 Cursor CLI。OpenCode、Pi、Oh My Pi 可按安装文档接入，再将命令的 --host 改为 opencode、pi、on-my-pi。",
		},
		{
			Kind:  "event-watch",
			Title: "启动 Python 监听器（Python 3.9+；部分系统用 python3 或 py -3）",
			Lang:  "sh",
			Code: fmt.Sprintf(`python "slate-watch.py" --url "%s" --board "<看板编号>" --name "%s" --project "%s" --host %s`,
				endpoint, name, projectDirPlaceholder, host),
		},
		{
			Kind:  "event-resume",
			Title: "管理和恢复 Python 监听器",
			Lang:  "text",
			Code: "启用访问口令时设置 SLATE_PASSCODE。\n\n" +
				"macOS 图形登录会话：把脚本放在固定目录，用上方启动命令加 --supervise enable。用相同参数加 --supervise status 查看进程锁、最近检查和游标；加 --supervise disable 停止托管，保留游标。启用会保存启动参数、PATH 和环境中的访问口令，配置文件仅当前用户可读写。\n" +
				"LaunchAgent 每五分钟检查进程。它不唤醒休眠的 Mac，错过的检查在唤醒后的下个周期继续。其他平台使用前台启动命令。Claude Code 和 Cursor 的 /loop 5m 只用于交互会话。\n\n" +
				"回执缺失、无效或宿主异常退出时，监听器保留待确认游标并暂停派发。用原启动参数加 --status 查询。确认旧调度已停止并核对卡片后，已处理用 --resolve done，确认可安全重跑时用 --resolve retry。托管监听器会在下次检查恢复；手动监听器用原命令重启。沿用同名、同地址和同一状态目录。\n" +
				"连续三次启动都未通过存活检查时，LaunchAgent 暂停重启。用 --supervise status 查日志目录，排除问题后停用再启用。",
		},
	}
}

// cursorWatch 是 Cursor 的长期挂着步骤：Cursor 的会话在编辑器里，没有终端可挂，直接用 /loop 隔一段时间检查监听。
func cursorWatch(endpoint string) []Snippet {
	return append(eventWatch(endpoint, "cursor", "cursor"), Snippet{
		Kind:  "loop",
		Title: "Cursor：在项目里新开 Agent 会话，执行 /loop（编辑器和会话窗口要一直开着）",
		Lang:  "sh",
		Code:  loopCommand(loopPrompt),
	})
}
