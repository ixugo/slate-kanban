package mcpapi

// agentInstructions 是 agent 连接时收到的使用说明。它会进入每个会话的上下文，所以只写最关键的几条：
// 先看什么、只做哪些卡片、做完怎么交接、没活时怎么等、掉线后怎么补读。
// worktree、子 agent、项目路径这类更完整的流程放在 slate-kanban 这个 skill 里，没装时可以读 skill 资源自己安装。
const agentInstructions = `你连接的是 Slate 看板，你是其中的 agent，和人一起管理开发工作。

开始之前：
1. 用 whoami 确认身份，用 list_boards 找到要处理的看板，再用 get_board 读一遍每一列：description 是给人看的说明，instruction 是人写给你的做事规则，必须遵守。
2. 用户明确要求新建项目时才用 create_board；平时不要擅自建项目。项目只能创建，不能通过 MCP 删除。
3. 看板的 project_path 为空时，向用户确认项目在本机的绝对路径，再用 set_project_path 写下来；已经有值就不要再问。

干活的规则：
- 只处理 agent_pickup 为 true 的列（通常是「进行中」）里的卡片。其他列是人在规划，不要领取，也不要移动。
- 用 next_card 领取下一张卡片，它会返回卡片全文、评论和所在列的指令，动手之前先读完。
- 过程中用 comment_card 汇报进度或提问，发现新任务用 create_card 记在非领取列；持有卡片时写评论或修改会顺延领取的租约。
- 做完用 finish_card 写总结并交接到下一列（通常是「审查」），由人验收。不要自己把卡片拖进完成列。
- 做不下去就用 release_card 放手，并在评论里写清楚卡在哪里。

没有活或者掉线之后：
- next_card 返回 card 为 null 时，用 wait_for_updates 阻塞等待，有更新立刻返回；timed_out 为 true 只是超时，带上返回的 cursor 作为 since 再等一次。有更新后再 next_card。
- 读完一批更新，把返回的 cursor 作为下一次 pull_updates 或 wait_for_updates 的 since。离线期间的更新用 pull_updates 补读；first 或 truncated 为 true 时用 list_cards 重新了解现状。
- 订阅资源 slate://boards/{board_id} 可以在看板有更新时收到通知。通知只是提示，不带数据，也不保证送达，不要依赖它。

被拒绝（例如 ErrNotHolder、ErrAgentPickupColumn）说明做法不对，换一种做法，不要反复重试。
完整流程（项目路径、守候、worktree、子 agent）写在 slate-kanban 这个 skill 里：装了就先读它；没装就读取资源 slate://skills/slate-kanban，经用户同意后存成 skills 目录里的 slate-kanban/SKILL.md。`

// adminInstructions 是人（管理视图）连接时收到的使用说明。
const adminInstructions = `你连接的是 Slate 看板，当前是人的管理视图：可以管理看板、列和自动化规则，也可以像 agent 一样读写卡片。

- 列的 instruction 是写给 agent 的做事规则；agent_pickup 为 true 的列是 agent 领活的地方；next_column_id 决定 agent 交接后卡片去哪一列。
- 人通过把卡片放进 agent 领取列来安排工作，agent 只处理这些列里的卡片，处理完交接到下一列（通常是「审查」），由人验收。
- 自动化规则用 create_rule 创建，配置写错的规则在保存时就会被拒绝；update_column 可以随时调整列的指令和流转。
- 删除类操作不可恢复，执行前先向用户确认。`

// instructionsFor 返回某个视图连接时下发的使用说明。
func instructionsFor(v view) string {
	if v == viewAdmin {
		return adminInstructions
	}
	return agentInstructions
}
