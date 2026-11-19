package mcpapi

import "slate-kanban/internal/core/kanban"

// unclearTaskNotice 是读取或领取任务卡片时附在结果里的提示，提醒 agent 任务不清楚就先问，别硬做。
// 放在每次返回卡片全文的结果里，而不是只写在连接说明里：连接说明会被上下文压缩冲掉，卡片结果每次都新鲜。
const unclearTaskNotice = "动手之前先判断任务是否明确。目标、范围或验收标准有缺失或歧义，或者描述和评论里的要求互相矛盾时，" +
	"不要直接开始：先用 comment_card 在这张卡片下写出你的疑问和你的理解，再用 move_card 把卡片移出领取列（例如移到「收件箱」，换列会同时结束你的领取）。" +
	"只 release_card 不行：卡片还在领取列里，next_card 会马上又领到它。人回复后会把卡片放回领取列，你再领到时先读新评论。" +
	"任务已经清楚就直接做，不用为了确认而确认。"

// cardDetailOut 是返回卡片全文的工具（get_card、claim_card）的输出：卡片字段保持原样平铺，再多一个 agent_notice。
type cardDetailOut struct {
	*kanban.CardDetail
	Notice string `json:"agent_notice"`
}

// withNotice 给卡片全文加上"任务不清楚先反馈"的提示。
func withNotice(detail *kanban.CardDetail) cardDetailOut {
	return cardDetailOut{CardDetail: detail, Notice: unclearTaskNotice}
}
