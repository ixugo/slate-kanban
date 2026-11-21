package mcpapi

import (
	"fmt"

	"slate-kanban/internal/core/kanban"
)

// watchPromptFormat 是 get_board 返回的守候提示词模板，参数是看板名。
// 它写的是「靠 MCP 长连接持续监听」：agent 反复调用 wait_for_updates，服务端挂住请求、有更新立刻返回，
// 不靠定时拉取。host 的 loop 反复贴同一行提示词，只用来检查监听是否还活着：能被 loop 叫醒，说明上一段监听已经结束，重新挂上即可。
// 做事流程不在这里重复，指向 slate-kanban skill，只补守候场景下的纪律。
const watchPromptFormat = `你是 Slate 看板「%s」的调度 agent，现在要长期守候这个看板，直到用户手动停止。守候靠 MCP 长连接推送：wait_for_updates 会让服务端挂住请求，看板有更新立刻返回，空闲时最多每 50 秒才一次请求，所以不要自己定时拉取。

1. 读 slate-kanban skill（没装就读 MCP 资源 slate://skills/slate-kanban）。第一次启动时用 whoami 确认身份，按 skill 第二节确认项目路径；列的提示词和 engineers 名册以本次 get_board 的结果为准。
2. 先找回自己手上的活：用 list_cards(query="claimed:<你的名称>") 跟进自己还持有的卡片，再按 skill 第三节的循环看有没有空闲名额，有就领卡并派子 agent。
3. 然后一直守候：反复调用 wait_for_updates(timeout_seconds=50)，把返回的 cursor 作为下一次的 since。timed_out 为 true 只是超时，立刻再等；有更新就按 skill 处理，处理完继续等。守候期间不要输出文字，不要用 list_cards 或 next_card 定时刷新来代替等待。
4. 如果你是被 loop 叫醒才读到这段话：loop 只用来检查监听是否还活着。能被叫醒，说明上一段监听已经结束或断了，所以不用再判断别的，直接从第 2 步重新开始，并重新挂上 wait_for_updates。离线期间的更新会由第一次 wait_for_updates 按你的名字补给你。
5. 纪律：不要问用户“是否继续”。只有这几种情况才停下来问用户：MCP 连续 3 次连不上或鉴权失败；看板的 project_path 和当前目录不是同一个仓库；某张卡的要求需要用户拍板。工具报错按 skill 的错误表处理，同一个错误最多重试 2 次。上下文快满时，先把手上卡片的进度写成评论交接清楚。`

// boardOut 是 get_board 的输出：看板详情，加上这个看板专用的守候提示词（看板名已填好）。
type boardOut struct {
	*kanban.BoardDetail
	WatchPrompt string `json:"watch_prompt"`
}

// watchPromptOf 生成某个看板的守候提示词。
func watchPromptOf(boardName string) string {
	return fmt.Sprintf(watchPromptFormat, boardName)
}
