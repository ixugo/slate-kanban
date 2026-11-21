package mcpapi

import (
	"fmt"
	"strings"

	"slate-kanban/internal/core/kanban"
)

// watchPromptFormat 是 get_board 返回的守候提示词模板，两个参数依次是看板名和这个看板的并发名额说明（见 slotsText）。
// 它写的是「靠 MCP 长连接持续监听」：agent 反复调用 wait_for_updates，服务端挂住请求、有更新立刻返回，
// 不靠定时拉取。host 的 loop 反复贴同一行提示词，只用来检查监听是否还活着：能被 loop 叫醒，说明上一段监听已经结束，重新挂上即可。
// 做事流程不在这里重复，指向 slate-kanban skill，只补守候场景下的纪律。
const watchPromptFormat = `你是 Slate 看板「%s」的调度 agent，现在要长期守候这个看板，直到用户手动停止。守候靠 MCP 长连接推送：wait_for_updates 会让服务端挂住请求，看板有更新立刻返回，空闲时最多每 50 秒才一次请求，所以不要自己定时拉取。

1. 读 slate-kanban skill（没装就读 MCP 资源 slate://skills/slate-kanban）。第一次启动时用 whoami 确认身份，按 skill 第二节确认项目路径；列的提示词和 engineers 名册以本次 get_board 的结果为准。
2. 先找回自己手上的活：用 list_cards(query="claimed:<你的名称>") 跟进自己还持有的任务，再按 skill 第三节的循环数名额、领任务：有空闲名额就一次领满，并按第 5 条同时派出子 agent。
3. 然后一直守候：反复调用 wait_for_updates(timeout_seconds=50)，把返回的 cursor 作为下一次的 since。timed_out 为 true 只是超时，立刻再等；有更新就按 skill 处理，处理完继续等。守候期间不要输出文字，不要用 list_cards 或 next_card 定时刷新来代替等待。
4. 如果你是被 loop 叫醒才读到这段话：loop 只用来检查监听是否还活着。能被叫醒，说明上一段监听已经结束或断了，所以不用再判断别的，直接从第 2 步重新开始，并重新挂上 wait_for_updates。离线期间的更新会由第一次 wait_for_updates 按你的名字补给你。
5. 分工和并发：你是这个终端里唯一的调度 agent，只负责领任务和派发，不亲自写代码。%s研发和审查的名额各算各的，互不占用。每一轮都要把空闲名额一次领满：有几个空闲名额、手上就同时领几个任务；每个研发任务由你先建好独立的 git worktree 和分支，绝不让多个任务共用一个目录，然后在同一次回复里把这几个子 agent 一起派出，让它们并行开工，不要领一个、等它做完再领下一个。一个任务新开一个子 agent，不要把另一个任务交给做过别的任务的子 agent，只有同一个任务返工才继续用原来的。子 agent 的数量还受你所在 host 的上限限制，超出的任务留在原来的列里排队，等有子 agent 交卷再领。
6. 纪律：不要问用户“是否继续”。只有这几种情况才停下来问用户：MCP 连续 3 次连不上或鉴权失败；看板的 project_path 和当前目录不是同一个仓库；某个任务的要求需要用户拍板。工具报错按 skill 的错误表处理，同一个错误最多重试 2 次。上下文快满时，先把手上任务的进度写成评论交接清楚。`

// boardOut 是 get_board 的输出：看板详情，加上这个看板专用的守候提示词（看板名和并发名额已填好）。
type boardOut struct {
	*kanban.BoardDetail
	WatchPrompt string `json:"watch_prompt"`
}

// watchPromptOf 生成某个看板的守候提示词：填上看板名，并按名册写明这个看板能同时跑几个研发和审查子 agent。
func watchPromptOf(detail *kanban.BoardDetail) string {
	return fmt.Sprintf(watchPromptFormat, detail.Name, slotsText(detail.Engineers))
}

// slotsText 按工程师名册算出并发名额，写成一句话放进守候提示词。
// 由服务端数清楚，是因为 agent 自己数容易出错：曾经有 agent 把审查占的名额也算进研发名额，
// 看板里明明有两位空闲的研发工程师，却只派了一个研发子 agent。
// 研发类（全栈、前端、后端、UI）和测试各算各的；某一类一位都没有时，这一类按 1 张处理，和 skill 第三节一致。
func slotsText(engineers []*kanban.Engineer) string {
	var dev, review []string
	for _, e := range engineers {
		if e.Kind == kanban.EngineerReview {
			review = append(review, e.Name)
		} else {
			dev = append(dev, e.Name)
		}
	}
	devSlots, reviewSlots := max(len(dev), 1), max(len(review), 1)
	return fmt.Sprintf("名册是：%s，%s，所以最多同时做 %d 个研发任务、同时审 %d 个审查任务，共 %d 个子 agent 并行（某一类一位都没有时按 1 个算；用户另有要求时以用户为准，例如「并发 1」）。",
		rosterText(dev, "研发工程师"), rosterText(review, "测试工程师"), devSlots, reviewSlots, devSlots+reviewSlots)
}

// rosterText 把一类工程师写成「2 位研发工程师（Atlas、Orion）」；这一类没有人时写成「没有研发工程师」。
func rosterText(names []string, label string) string {
	if len(names) == 0 {
		return "没有" + label
	}
	return fmt.Sprintf("%d 位%s（%s）", len(names), label, strings.Join(names, "、"))
}
