package connectapi

import "strings"

const (
	// boardPlaceholder 是提示词里看板名的占位符；网页按用户选的看板替换它，install.md 里原样保留让用户自己改。
	boardPlaceholder = "<看板名>"
	// projectDirPlaceholder 是命令里项目目录的占位符，对应看板的 project_path。
	projectDirPlaceholder = "<项目目录>"
	// screenSession 是终端会话的名字，分离和回来时都用它。
	screenSession = "slate-agent"
)

// watchPrompt 是「长期挂着」用的提示词：粘进 host 的会话后，agent 按 skill 的守候循环一直工作，直到用户手动停止。
// 它不重复 skill 的流程，只补三件事：先接入、永不收工、守候时别浪费上下文。
const watchPrompt = `你是 Slate 看板「` + boardPlaceholder + `」的调度 agent，要长期挂着守候这个看板，直到我手动停止你。

1. 先读 slate-kanban skill（没装就读 MCP 资源 slate://skills/slate-kanban）。用 whoami 确认身份，list_boards 找到「` + boardPlaceholder + `」，get_board 读各列提示词和 engineers 名册，按 skill 第二节确认项目路径。
2. 然后严格按 skill 第三节的守候循环干活：先找回自己还持有的卡片，有空闲名额就领卡并派子 agent，没有就 wait_for_updates（timeout_seconds 用 30），超时只是没动静，立刻再调用。
3. 长期运行的纪律：
   - 不要收工，不要问我“是否继续”，不要做总结后停下；没有任务时的正常状态就是继续 wait_for_updates。
   - 守候期间不要输出任何文字，不要复述状态，直接进入下一次工具调用，省上下文。
   - 只有这几种情况才停下来问我：MCP 连续 3 次连不上或鉴权失败；看板的 project_path 和当前目录不是同一个仓库；某张卡的要求需要我拍板。
   - 工具报错按 skill 的错误表处理，同一个错误最多重试 2 次。
   - 上下文快满时，先把手上卡片的进度写成评论交接清楚，再继续循环；被重启后凭 claimed:<你的名称> 找回自己的卡片。`

// loopPrompt 是配合 host 自带的 loop 或定时功能用的「单轮」提示词：每次只做一轮，没事就结束，由 host 隔一段时间再叫醒。
const loopPrompt = `执行一轮 Slate 看板「` + boardPlaceholder + `」的调度，不要长时间等待：
按 slate-kanban skill 第三节，先找回自己还持有的卡片并跟进，再看有没有空闲名额：有就领卡并派子 agent。
没有可做的卡片、也没有还在跑的子 agent 时，不要调用 wait_for_updates，直接结束这一轮，只回一句“本轮无事”。`

// watchNotes 是和具体 host 无关的说明：长期挂着到底是什么、挂不住怎么办。
var watchNotes = []string{
	"Slate 服务端不替你跑 agent。“挂着”的是 agent 自己的会话：它不停调用 wait_for_updates（每次最多等 50 秒），有卡片就领、派子 agent，没有就继续等。",
	"会话会在上下文或额度用完、关掉终端、电脑休眠后结束。结束后用下面的恢复命令回到会话，再粘一次同一段提示词即可：它会按 claimed:<名称> 找回自己持有的卡片，并从游标补拉离线期间的更新。",
	"停机超过租约（默认 30 分钟）时，卡片会被释放，重新上线后重新领取。",
	"没人看着的时候，权限弹窗会把会话卡住：先把 slate 的工具和常用的 git、编辑操作设成“总是允许”。只有在容器或专用机器这类隔离环境里，才考虑关闭权限检查。",
}

// screenSnippet 生成“开一个关掉终端也不会停的会话再启动 host”的命令。launch 是 host 的启动命令。
func screenSnippet(launch string) Snippet {
	return Snippet{
		Kind:  "watch",
		Title: "长期挂着：在项目目录开一个终端会话（终端窗口关掉也不会停），再启动 agent 并粘贴「长期守候提示词」",
		Lang:  "sh",
		Code: strings.Join([]string{
			"cd " + projectDirPlaceholder + "   # 看板的项目目录，即 get_board 里的 project_path",
			"screen -S " + screenSession + "    # 或 tmux new -s " + screenSession + "；脱离会话按 Ctrl-A 再按 D",
			launch,
		}, "\n"),
	}
}

// resumeSnippet 生成“会话断了怎么回来”的命令。resume 是 host 接上最近对话的命令。
func resumeSnippet(resume string) Snippet {
	return Snippet{
		Kind:  "resume",
		Title: "回来看看，或会话已经结束时恢复（恢复后再粘贴一次同一段提示词）",
		Lang:  "sh",
		Code: strings.Join([]string{
			"screen -r " + screenSession + "    # 会话还在就直接回去",
			resume,
		}, "\n"),
	}
}

// claudeWatch 是 Claude Code 的长期挂着步骤。
func claudeWatch() []Snippet {
	return []Snippet{
		screenSnippet("claude                 # 进入后把「长期守候提示词」整段粘贴进去，回车"),
		resumeSnippet("claude --continue      # 接上最近一次对话"),
	}
}

// codexWatch 是 Codex 的长期挂着步骤。
func codexWatch() []Snippet {
	return []Snippet{
		screenSnippet("codex                  # 进入后把「长期守候提示词」整段粘贴进去，回车"),
		resumeSnippet("codex resume --last    # 接上最近一次对话"),
	}
}

// cursorWatch 是 Cursor 的长期挂着步骤：Cursor 的会话在编辑器里，没有终端可挂，改用 /loop 隔一段时间叫醒一轮。
func cursorWatch() []Snippet {
	return []Snippet{{
		Kind:  "loop",
		Title: "Cursor：在项目里新开 Agent 会话，用 /loop 隔几分钟叫醒一轮（粘贴「单轮提示词」；编辑器和会话窗口要一直开着）",
		Lang:  "sh",
		Code:  "/loop 5m " + strings.ReplaceAll(loopPrompt, "\n", " "),
	}}
}
