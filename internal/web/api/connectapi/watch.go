package connectapi

import "strings"

const (
	// boardPlaceholder 是提示词里看板名的占位符；网页按用户选的看板替换它，install.md 里原样保留让用户自己改。
	boardPlaceholder = "<看板名>"
	// projectDirPlaceholder 是命令里项目目录的占位符，对应看板的 project_path。
	projectDirPlaceholder = "<项目目录>"
	// screenSession 是终端会话的名字，分离和回来时都用它。
	screenSession = "slate-agent"
	// loopInterval 是 loop 检查监听是否还活着的间隔，小于卡片领取租约（默认 30 分钟），agent 掉线后卡片也不会被误释放。
	loopInterval = "5m"
	// codexLoopSeconds 是 Codex 的 shell 循环在一次 codex exec 结束后、重新拉起前的等待秒数，和 loopInterval 一致。
	codexLoopSeconds = "300"
)

// loopPrompt 是用户唯一需要贴的一行提示词：agent 靠 wait_for_updates 长连接持续监听，
// loop 只是反复贴这一行来检查监听是否还活着；完整的守候提示词由 MCP 的 get_board 返回
// （watch_prompt，看板名已填好），所以这里只指路。
const loopPrompt = `守候 Slate 看板「` + boardPlaceholder + `」：调用 get_board 读取返回里的 watch_prompt，照它做。`

// watchNotes 是和具体 host 无关的说明：持续监听怎么做到的、loop 负责什么、挂不住怎么办。
var watchNotes = []string{
	"持续监听走 MCP 协议：agent 反复调用 wait_for_updates，服务端把这次调用挂住，看板有更新立刻返回；没有更新时每次最多等 50 秒再重新挂上，所以空闲时大约每 50 秒一次请求，不是定时拉取任务。",
	"loop（或下面的 shell 循环）只负责检查监听还活着没有：监听在时 agent 正忙在这次调用里，多数 host 的 loop 要等它空闲才会触发；监听断了（会话结束、上下文满、网络断）agent 空闲，loop 叫醒它，它重新挂上 wait_for_updates。重启时沿用同一个 ?name=，服务端记着的游标会把离线期间的更新补给它。",
	"停机超过租约（默认 30 分钟）时，任务会被释放，重新上线后重新领取。",
	"没人看着的时候，权限弹窗会把会话卡住：先把 slate 的工具和常用的 git、编辑操作设成“总是允许”。只有在容器或专用机器这类隔离环境里，才考虑关闭权限检查。",
}

// loopCommand 生成 host 里的 loop 命令：隔 loopInterval 检查一次监听，内容是一行提示词。
func loopCommand() string {
	return "/loop " + loopInterval + " " + loopPrompt
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
func claudeWatch() []Snippet {
	return []Snippet{
		screenSnippet("claude                 # 进入后执行：\n" + loopCommand()),
		resumeSnippet("claude --continue      # 接上最近一次对话"),
	}
}

// codexWatch 是 Codex 的长期挂着步骤：用 shell 循环执行 codex exec。每次 codex exec 会一直停在 wait_for_updates 里，
// 它结束（会话断了、上下文满了）后循环等一会儿再拉起新会话，状态都在看板里。
func codexWatch() []Snippet {
	loop := "while true; do codex exec '" + loopPrompt + "'; sleep " + codexLoopSeconds + "; done"
	return []Snippet{
		screenSnippet(loop),
		resumeSnippet(""),
	}
}

// cursorWatch 是 Cursor 的长期挂着步骤：Cursor 的会话在编辑器里，没有终端可挂，直接用 /loop 隔一段时间检查监听。
func cursorWatch() []Snippet {
	return []Snippet{{
		Kind:  "loop",
		Title: "Cursor：在项目里新开 Agent 会话，执行 /loop（编辑器和会话窗口要一直开着）",
		Lang:  "sh",
		Code:  loopCommand(),
	}}
}
