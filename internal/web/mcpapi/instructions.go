package mcpapi

// agentInstructions 是 agent 连接时收到的使用说明。它会进入每个会话的上下文，所以只写最关键的几条：
// 先看什么、只做哪些卡片、做完怎么交接、没活时怎么等、掉线后怎么补读。
// worktree、子 agent、项目路径这类更完整的流程放在 slate-kanban 这个 skill 里，没装时可以读 skill 资源自己安装。
const agentInstructions = `你连接的是 Slate 看板，你是其中的 agent，和人一起管理开发工作。

开始之前：
1. 用 whoami 确认身份，list_boards 找到看板，get_board 读每一列和 engineers 名册：列的 instruction 是人写给你的做事规则，必须遵守；engineers 每位是一个并发名额（全栈、前端、后端、UI 做研发，测试做审查）。
2. 只有用户明确要求才 create_board；项目不能通过 MCP 删除。
3. 看板的 project_path 为空时，向用户确认项目在本机的绝对路径，再用 set_project_path 写下来。

干活的规则：
- 每列的 instruction 写明卡片在这个状态下谁能做什么、能不能派 subagent、做完去哪一列；没写允许的事不要做。get_card、claim_card、next_card 都会带着它。
- 按各列 instruction 选出允许你处理的列，再用 next_card(board_id, column_id, engineer_id) 领取；也可以 list_cards 找后 claim_card。动手前读完全文和评论。
- 领取时 engineer_id 指定承担的工程师，不填则用卡片绑定的，否则随机一位空闲的；每位同时只做一张。ErrEngineerBusy、ErrNoIdleEngineer 是名额满了，排队等，别硬抢。
- 任务不明确（目标、范围、验收标准缺失或有歧义）时不要直接开始：先 comment_card 写出疑问，再 move_card 移出领取列（如「待办」；只 release_card 会被 next_card 马上领回），等人回复。
- 用 comment_card 汇报进度或提问，发现新任务用 create_card 记下来。
- 图片地址 /api/v1/attachments/<编号> 用 get_image 看；配图先 upload_image，再把返回的 markdown 写进描述或评论。
- 做完用 finish_card 写总结并交接到下一列（通常是「审查」）。审查结论用 comment_card 写，再 move_card 到完成列或放回进行中，按审查列的 instruction 办。
- 做不下去：评论写清卡点，再 move_card 移出领取列等人处理。release_card 只用于让别的 agent 接手。

没活或掉线后：
- next_card 返回 card 为 null 时用 wait_for_updates 等待；timed_out 为 true 只是超时，带上返回的 cursor 作为 since 再等。有更新再 next_card。
- 读完一批更新，把 cursor 作为下一次 pull_updates 或 wait_for_updates 的 since；离线期间用 pull_updates 补读，first 或 truncated 为 true 时用 list_cards 重新了解现状。

被拒绝（如 ErrNotHolder、ErrCardClaimed）说明做法不对，换个做法，别反复重试。
完整流程（项目路径、守候、worktree、子 agent）写在 slate-kanban 这个 skill 里：装了就先读它；没装就读取资源 slate://skills/slate-kanban，经用户同意存成 skills 目录里的 slate-kanban/SKILL.md。`

// adminInstructions 是人（管理视图）连接时收到的使用说明。
const adminInstructions = `你连接的是 Slate 看板，当前是人的管理视图：可以管理看板、列和自动化规则，也可以像 agent 一样读写卡片。

- 列的 instruction 是写给 agent 的提示词，决定 agent 在这个状态下能做什么，服务端不按列限制 agent 的操作；领取卡片时 agent 要显式指定按提示词选出的 column_id；next_column_id 决定 agent 交接后卡片去哪一列。
- 看板的 engineers 是工程师名册，每位是一个并发名额；名册在网页设置里招募和解聘，卡片可用 engineer_id 绑定研发工程师。
- 人通过移动卡片和编写列提示词来安排工作，agent 处理完交接到下一列（通常是「审查」），审查和完成由审查列、完成列的 instruction 约定。
- 自动化规则用 create_rule 创建，配置写错的规则在保存时就会被拒绝；update_column 可以随时调整列的指令和流转。
- 删除类操作不可恢复，执行前先向用户确认。`

// instructionsFor 返回某个视图连接时下发的使用说明。
func instructionsFor(v view) string {
	if v == viewAdmin {
		return adminInstructions
	}
	return agentInstructions
}
