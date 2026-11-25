package mcpapi

import "slate-kanban/skills"

// 初始化只下发入口和交接约束。按需从 skill 资源获取完整流程。
const agentInstructions = `开始调度前读取 slate://skills/slate-kanban，然后用 whoami、list_boards、get_board 核对身份、项目路径、列规则和名册。
遵守列指令，研发和审查名额分开算；研发有空位先查进行中，再查待办。子 agent 先只读拟定方案，评论成功后开工。任务澄清和路径确认在看板留言。
用 finish_card 交接，审查按 skill 提供证据。
长期守候可用 Python 监听脚本 ` + skills.WatchURI + `，配置见看板的 watch_prompt。由监听器触发的一轮调度不重复等待。
只有用户明确要求管理看板设置、列或自动化时，才调用 enable_management。`

const adminInstructions = `管理看板设置、列和自动化时，先用 get_board 读取当前配置。删除前请用户确认。工作任务的完整流程见 slate://skills/slate-kanban。`

func instructionsFor(v view) string {
	if v == viewAdmin {
		return adminInstructions + skills.CardWritingInstructions
	}
	return agentInstructions + skills.CardWritingInstructions
}

// 支持 Skills 扩展的宿主先获得入口及动态任务约束，再按实际场景加载正文。
func skillInstructionsFor(v view) string {
	writing := "\n写卡片、评论或附图时按需加载 " + skills.URI(skills.WritingName) + "。"
	if v == viewAdmin {
		return "仅按用户授权管理。先用 get_board 核对当前配置，按需加载 " + skills.URI(skills.ManagementName) + "。调度工作见 " + skills.URI(skills.CoreName) + "。" + writing
	}
	return "开始调度前加载 " + skills.URI(skills.CoreName) + "，用 whoami、list_boards、get_board 核对身份、路径、列规则和名册。" +
		"研发与审查名额分开，研发有空位先查进行中再查待办。子 agent 先只读拟定方案，评论成功才开工；任务澄清、路径确认走看板。用 finish_card 交接。" +
		"长期守候按 watch_prompt 及 " + skills.URI(skills.WatchName) + "，监听器触发的一轮调度不等待。仅在用户明确要求管理时加载 " + skills.URI(skills.ManagementName) + "、调用 enable_management。" + writing
}
