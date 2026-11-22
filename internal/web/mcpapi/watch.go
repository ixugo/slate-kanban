package mcpapi

import (
	"fmt"
	"strings"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/skills"
)

// watchPromptFormat 是 get_board 返回的守候提示词模板，两个参数依次是看板名和这个看板的并发名额说明（见 slotsText）。
// 它写的是「靠 MCP 长连接持续监听」：agent 反复调用 wait_for_updates，服务端挂住请求、有更新立刻返回，
// 不靠定时拉取。host 的 loop 反复贴同一行提示词，只用来检查监听是否还活着：能被 loop 叫醒，说明上一段监听已经结束，重新挂上即可。
// 做事流程不在这里重复，指向 slate-kanban skill，只补守候场景下的纪律。
const watchPromptFormat = `你是 Slate 看板「%s」的调度 agent，现在要长期守候这个看板，直到用户手动停止。守候靠 MCP 长连接推送：wait_for_updates 会让服务端挂住请求，看板有更新立刻返回，空闲时可以连续等到 60 分钟。

1. 读 slate-kanban skill（没装就读 MCP 资源 slate://skills/slate-kanban）。第一次启动时用 whoami 确认身份，按 skill 第二节确认项目路径；列的提示词和 engineers 名册以本次 get_board 的结果为准。
   启动时先恢复持有的任务。启动、收到更新或子 agent 交卷后，都在再次等待前按下列顺序调度。根据 get_board 返回的 instruction 和 next_column_id 识别实际入口和去向，调用工具时带实际的 column_id。以下是默认五列的流程。
   - 进行中：有空闲研发名额时先领取已在进行中的任务。名额还有剩余时，必须主动检查待办列，即使进行中的 next_card 返回 card 为 null。
   - 待办：有研发空位就用 next_card 领取，再读完整描述、评论和依赖。任务明确时用 move_card 移到进行中，在那里重新领取后才派发。任务不明确时先评论提问，再移回收件箱。研发名额已满时，待办留在原列排队。
   - 审查：独立使用审查名额领任务。研发结束用 finish_card 交接到审查；审查通过后写评论再移到完成，不通过则写明原因后退回进行中。
   - 收件箱与完成：平时跳过这两列的任务，只按各列指令做上述流转。检查完有空闲名额的各入口后再等待。
   所有审查入口都由调度者收集子 agent 的证据，用 comment_card 发布审查结论和证据。页面验收用 upload_image 上传实际截图并附上图片 Markdown，功能验收逐项记录（- [x] 通过，- [ ] 未通过或未验证）及依据。缺少必要证据时写清原因，暂不判通过。结论评论成功后按列规则流转。
2. 先找回自己手上的活：用 list_cards(query="claimed:<你的名称>") 跟进自己还持有的任务，再按 skill 第三节的循环数名额、领任务：有空闲名额就一次领满，并按第 5 条同时派出子 agent。
3. 然后一直守候：客户端和连接链路已配置支持长等待，且没有正在处理的任务、子 agent 或近期需检查的租约时，用 wait_for_updates(timeout_seconds=3600)。配置未知、连接提前断开或有任务需跟进时，用 wait_for_updates(timeout_seconds=50) 或更短。把返回的 cursor 作为下一次的 since。空超时直接再次等待；有更新就按 skill 处理，处理完继续等。守候期间不要输出文字，不要用 list_cards 或 next_card 定时刷新来代替等待。
   收到人的 board.updated 且 payload.fields 包含 description，或更新已截断（truncated）时，重新调用 get_board，以最新 watch_prompt 末尾的 AGENTS.md 为准。项目说明已清空时停止应用旧内容。
4. 如果你是被 loop 叫醒才读到这段话：loop 只用来检查监听是否还活着。能被叫醒，说明上一段监听已经结束或断了，所以不用再判断别的，直接从第 2 步重新开始，并重新挂上 wait_for_updates。离线期间的更新会由第一次 wait_for_updates 按你的名字补给你。
5. 分工和并发：你是这个终端里唯一的调度 agent，只负责领任务和派发，不亲自写代码。%s研发和审查的名额各算各的，互不占用。每一轮都要把空闲名额一次领满：有几个空闲名额、手上就同时领几个任务；每个研发任务由你先建好独立的 git worktree 和分支，绝不让多个任务共用一个目录，然后在同一次回复里把这几个子 agent 一起派出，让它们并行开工，不要领一个、等它做完再领下一个。一个任务新开一个子 agent，不要把另一个任务交给做过别的任务的子 agent，只有同一个任务返工才继续用原来的。研发和审查子 agent 都要先只读拟定方案并回传给你，由你用 comment_card 代发到任务下，评论成功后才通知子 agent 开始开发或验收；失败按 skill 处理。子 agent 的数量还受你所在 host 的上限限制，超出的任务留在原来的列里排队，等有子 agent 交卷再领。
6. 标签和任务项：领取任务后，先对照任务内容用 update_card 调整标签，只能从 get_board 返回的 tags 里挑已有的标签（用 tag_ids），不要新建标签，没有合适的就不贴。交接前让研发子 agent 把实现了什么回传给你，由你用 update_card 在描述末尾补上「任务项」清单（每项一行：已实现的写「- [x] 内容」，没做完的写「- [ ] 内容」），再调用 finish_card。
7. 确认事项都走看板，禁止在终端触发阻塞式提问。任务的范围、验收或授权需人决定时，用 comment_card 留下问题，再按列指令移回收件箱，腾出名额后继续其他可做的任务。看板 project_path 缺失或与当前仓库不匹配时，按 skill 第二节在收件箱复用或新建配置任务，暂停依赖该路径的工作，监听配置任务回复和看板设置变更；核对正确路径再恢复。路径写入须有人明确确认，不绕过宿主权限。MCP 连续 3 次连不上、鉴权失败或没有可写的看板时，只报告状态，不启动终端问答。其他工具报错按 skill 处理，同一个错误最多重试 2 次。上下文快满时，用任务评论交接进度。`

// boardOut 是 get_board 的输出：看板详情，加上这个看板专用的守候提示词（看板名和并发名额已填好）。
type boardOut struct {
	*kanban.BoardDetail
	WatchPrompt string `json:"watch_prompt"`
}

// watchPromptOf 生成某个看板的守候提示词：填上看板名，并按名册写明这个看板能同时跑几个研发和审查子 agent。
func watchPromptOf(detail *kanban.BoardDetail) string {
	setup := fmt.Sprintf("空闲时无需调用模型的守候方式：读取 MCP 资源 %s，将脚本保存为 slate-watch.py。"+
		"运行 python slate-watch.py --url \"<Slate MCP 地址>\" --board %d --name \"<whoami 的名称>\" --project \"<项目目录>\"。"+
		"脚本根据标准身份或唯一可用的本地命令选宿主，也可用 --host 明确选 codex、cursor、claude、opencode、pi、on-my-pi。"+
		"自定义客户端用本地 --command-json。地址使用当前 Slate 的 /mcp，重启时沿用同一个 name。"+
		"先按接入说明验证宿主的 Slate 工作视图；脚本发来的事件任务只执行一轮调度，不进入下面的守候循环。\n\n",
		skills.WatchURI, detail.ID)
	prompt := fmt.Sprintf(watchPromptFormat, detail.Name, slotsText(detail.Engineers))
	if strings.TrimSpace(detail.Description) != "" {
		prompt += "\n\n## 项目说明（AGENTS.md）\n\n" + detail.Description
	}
	return setup + prompt
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
