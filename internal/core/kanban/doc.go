// Package kanban 是看板领域：看板、列、卡片、评论、自动化规则、事件日志和 agent 游标。
//
// 人和 agent 共用同一套业务方法，区别只在调用时传入的 Actor：
// 权限判断、事件记录和自动化规则都集中在这里，网页 API 和 MCP 只做协议转换。
//
// 所有会改数据的方法都通过事务执行器 Core.run 完成：
// 先锁定看板行，在事务里改数据并追加事件，提交成功后才广播给等待者。
// 设计文档见 .cursor/scopes/kanban.*.md。
package kanban
