package mcpapi

import (
	"fmt"

	"slate-kanban/internal/core/kanban"
)

// watchPromptFormat 是 get_board 返回的守候提示词模板，参数是看板名。
// 它写的是「loop 每次叫醒时做一轮」：长期挂着靠 host 的 loop 反复叫醒，不靠提示词让会话阻塞；
// 每轮的做事流程不在这里重复，指向 slate-kanban skill，只补叫醒场景下的纪律。
const watchPromptFormat = `你是 Slate 看板「%s」的调度 agent，由 loop 定时叫醒来守候这个看板，直到用户手动停止 loop。每次被叫醒只做一轮：

1. 读 slate-kanban skill（没装就读 MCP 资源 slate://skills/slate-kanban）。第一次叫醒时用 whoami 确认身份，按 skill 第二节确认项目路径；列的提示词和 engineers 名册以本次 get_board 的结果为准。
2. 按 skill 第三节干这一轮：先用 list_cards(query="claimed:<你的名称>") 找回自己还持有的卡片并跟进，再看有没有空闲名额，有就领卡并派子 agent。
3. 本轮不要长时间等待，不要调用 wait_for_updates。没有可做的卡片、也没有还在跑的子 agent 时，直接结束这一轮，只回一句“本轮无事”。
4. 纪律：不要问用户“是否继续”。只有这几种情况才停下来问用户：MCP 连续 3 次连不上或鉴权失败；看板的 project_path 和当前目录不是同一个仓库；某张卡的要求需要用户拍板。工具报错按 skill 的错误表处理，同一个错误最多重试 2 次。上下文快满时，先把手上卡片的进度写成评论交接清楚。`

// boardOut 是 get_board 的输出：看板详情，加上这个看板专用的守候提示词（看板名已填好）。
type boardOut struct {
	*kanban.BoardDetail
	WatchPrompt string `json:"watch_prompt"`
}

// watchPromptOf 生成某个看板的守候提示词。
func watchPromptOf(boardName string) string {
	return fmt.Sprintf(watchPromptFormat, boardName)
}
