package mcpapi

import "slate-kanban/skills"

// 初始化只下发入口和交接约束。按需从 skill 资源获取完整流程。
const agentInstructions = `开始调度前读取 slate://skills/slate-kanban，然后用 whoami、list_boards、get_board 核对身份、项目路径、列规则和名册。
遵守列指令，研发和审查名额分开算；研发有空位先查进行中，再查待办。子 agent 先只读拟定方案，评论成功后开工。任务澄清和路径确认在看板留言。
用 finish_card 交接，审查按 skill 提供证据。
长期守候可用 Python 监听脚本 ` + skills.WatchURI + `，配置见看板的 watch_prompt。由监听器触发的一轮调度不重复等待。
只有用户明确要求管理看板设置、列或自动化时，才调用 enable_management。`

const adminInstructions = `管理看板设置、列和自动化时，先用 get_board 读取当前配置。删除前请用户确认。工作任务的完整流程见 slate://skills/slate-kanban。`

const cardWritingInstructions = `

用 Markdown 写卡片描述和评论。短段落或列表，**加粗重点与待决事项**。

1. 短句，一句一事。规则编号。删开篇多余依据、套话、常识和重复。
2. 需求先写用户何时能做什么、做完看到什么。
3. 限量时写谁来计数、怎么算、到上限会怎样。
4. 复用时写明对象和接法。字段、接口和验收写到可实施。
5. Git 提交哈希只写前 6 位。`

func instructionsFor(v view) string {
	if v == viewAdmin {
		return adminInstructions + cardWritingInstructions
	}
	return agentInstructions + cardWritingInstructions
}
