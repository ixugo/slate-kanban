package mcpapi

import "slate-kanban/internal/core/kanban"

// unclearTaskNotice 是读取或领取时附在结果里的提示，提醒 agent 任务不清楚就先问，别硬做。
// 放在每次返回任务全文的结果里，而不是只写在连接说明里：连接说明会被上下文压缩冲掉，任务结果每次都新鲜。
const unclearTaskNotice = "动手之前先判断任务是否明确。目标、范围、验收标准或授权有疑问时，" +
	"不触发终端阻塞式提问。先用 comment_card 在任务下写清疑问，再用 move_card 把任务移回「收件箱」或列指令约定的澄清列，腾出名额后继续其他可办任务。" +
	"只 release_card 不行：任务还在领取列里，next_card 会马上又领到它。人回复后会把任务放回领取列，你再领到时先读新评论。" +
	"任务已经清楚就直接做，不用为了确认而确认。"

// cardDetailOut 是返回任务全文的工具（get_card、claim_card）的输出：任务字段保持原样平铺，再多一个 agent_notice。
type cardDetailOut struct {
	*kanban.CardDetail
	Notice string `json:"agent_notice"`
}

// withNotice 给任务全文加上"任务不清楚先反馈"的提示。
func withNotice(detail *kanban.CardDetail) cardDetailOut {
	return cardDetailOut{CardDetail: detail, Notice: unclearTaskNotice}
}

// MarshalJSON agent 读任务不需要未读角标，去掉 REST 专有的 unread_count。
func (o cardDetailOut) MarshalJSON() ([]byte, error) {
	type alias cardDetailOut
	return kanban.MarshalJSONOmitField(alias(o), "unread_count")
}
