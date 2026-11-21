package mcpapi

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"slate-kanban/internal/core/kanban"
)

// ruleActionIn 是规则里的一个动作。所有动作共用同一个结构，每种动作只用到其中的一两个字段，
// 在这里给每个字段写清说明，模型才能不看别的文档就配对规则。
type ruleActionIn struct {
	Type     string `json:"type" jsonschema:"动作类型，可选值见 create_rule 的说明"`
	Mode     string `json:"mode,omitempty" jsonschema:"只用于 set_started_at 和 set_finished_at：if_empty 只在时间为空时写（默认），always 覆盖已有的值"`
	Tag      string `json:"tag,omitempty" jsonschema:"add_tag 和 remove_tag 用：标签，1 到 30 字"`
	Text     string `json:"text,omitempty" jsonschema:"add_comment 用：要自动写下的评论，最多 2000 字"`
	ColumnID int64  `json:"column_id,omitempty" jsonschema:"move_to_column 用：目标列编号"`
	Days     int    `json:"days,omitempty" jsonschema:"set_due 用：到期时间设为从现在起几天后，1 到 365"`
	Priority int    `json:"priority,omitempty" jsonschema:"set_priority 用：0 到 3，0 表示未设置"`
	Flag     bool   `json:"flag,omitempty" jsonschema:"set_human_only 用：true 设为只能由人处理，false 取消"`
}

// createRuleIn 是 create_rule 的入参。
type createRuleIn struct {
	BoardID   int64          `json:"board_id" jsonschema:"看板编号，来自 list_boards"`
	Name      string         `json:"name" jsonschema:"规则名称，1 到 100 字，单行"`
	Enabled   *bool          `json:"enabled,omitempty" jsonschema:"是否启用；不填启用"`
	Trigger   string         `json:"trigger" jsonschema:"触发时机：card_created、card_entered 或 card_left"`
	ColumnID  int64          `json:"column_id,omitempty" jsonschema:"在哪一列发生才触发；不填表示任意列"`
	Condition string         `json:"condition,omitempty" jsonschema:"条件，写法同 list_cards 的 query，任务满足才执行；不填表示无条件"`
	Actions   []ruleActionIn `json:"actions" jsonschema:"按顺序执行的动作，至少 1 个，最多 20 个"`
}

// updateRuleIn 是 update_rule 的入参，指针字段不填或为 null 都表示不改。
type updateRuleIn struct {
	RuleID    int64           `json:"rule_id" jsonschema:"规则编号，来自 get_board 返回的 rules"`
	Name      *string         `json:"name,omitempty" jsonschema:"新名称；不填不改"`
	Enabled   *bool           `json:"enabled,omitempty" jsonschema:"是否启用；不填不改"`
	Trigger   *string         `json:"trigger,omitempty" jsonschema:"新的触发时机；不填不改"`
	ColumnID  *int64          `json:"column_id,omitempty" jsonschema:"新的触发列；传 0 表示任意列；不填不改"`
	Condition *string         `json:"condition,omitempty" jsonschema:"新的条件；传空字符串表示无条件；不填不改"`
	Actions   *[]ruleActionIn `json:"actions,omitempty" jsonschema:"新的动作列表，整体替换，至少 1 个；不填不改"`
}

// ruleIn 是只需要规则编号的工具的入参。
type ruleIn struct {
	RuleID int64 `json:"rule_id" jsonschema:"规则编号，来自 get_board 返回的 rules"`
}

// toRuleActions 把工具入参里的动作换成 Core 的动作。
func toRuleActions(in []ruleActionIn) []kanban.RuleAction {
	out := make([]kanban.RuleAction, len(in))
	for i, a := range in {
		out[i] = kanban.RuleAction{
			Type: a.Type, Mode: a.Mode, Tag: a.Tag, Text: a.Text,
			ColumnID: a.ColumnID, Days: a.Days, Priority: a.Priority, Flag: a.Flag,
		}
	}
	return out
}

// registerRuleTools 注册自动化规则的创建、修改和删除。
func (a API) registerRuleTools(s *mcp.Server) {
	register(s, spec{
		name: "create_rule", kind: kindWrite,
		desc: "创建自动化规则：某件事发生并且任务满足条件时，按顺序执行一组动作，人和 agent 的操作都会触发。\n" +
			"触发时机 trigger：card_created（创建任务时）、card_entered（任务进入某一列，创建任务也算进入初始列）、" +
			"card_left（任务离开某一列）；column_id 指定发生在哪一列，不填表示任意列。" +
			"condition 用 list_cards 的筛选语法，任务满足才执行，不填表示无条件。\n" +
			"动作 actions 的 type：set_started_at、set_finished_at（把时间写成现在，mode 为 if_empty 只在为空时写，是默认，always 覆盖已有的值）；" +
			"clear_started_at、clear_finished_at、clear_due（清除时间）；set_due（days：从现在起几天后到期）；" +
			"set_priority（priority：0 到 3）；add_tag、remove_tag（tag）；set_human_only（flag）；" +
			"add_comment（text：自动写一条评论）；move_to_column（column_id：把任务移到另一列，会继续触发那一列的规则，连锁最多 3 层）。\n" +
			"保存时就会校验整条规则，写错的规则会被拒绝，不会保存成永远不触发的样子。",
	}, a.createRule)
	register(s, spec{
		name: "update_rule", kind: kindWrite,
		desc: "修改规则，只更新填写的字段，修改后整条规则重新校验；actions 是整体替换。想临时关掉规则，把 enabled 设为 false。",
	}, a.updateRule)
	register(s, spec{
		name: "delete_rule", kind: kindDestroy,
		desc: "删除规则，不可恢复；只是想暂停的话用 update_rule 把 enabled 设为 false。规则已经不存在也返回成功。",
	}, a.deleteRule)
}

// createRule 在看板上新增一条规则，排在现有规则的最后。
func (a API) createRule(ctx context.Context, actor kanban.Actor, in createRuleIn) (any, error) {
	return a.core.CreateRule(ctx, actor, in.BoardID, kanban.CreateRuleInput{
		Name: in.Name, Enabled: in.Enabled, Trigger: in.Trigger, ColumnID: in.ColumnID,
		Condition: in.Condition, Actions: toRuleActions(in.Actions),
	})
}

// updateRule 修改规则，只更新填写的字段。
func (a API) updateRule(ctx context.Context, actor kanban.Actor, in updateRuleIn) (any, error) {
	patch := kanban.UpdateRuleInput{
		Name: in.Name, Enabled: in.Enabled, Trigger: in.Trigger, ColumnID: in.ColumnID, Condition: in.Condition,
	}
	if in.Actions != nil {
		converted := toRuleActions(*in.Actions)
		patch.Actions = &converted
	}
	return a.core.UpdateRule(ctx, actor, in.RuleID, patch)
}

// deleteRule 删除一条规则。
func (a API) deleteRule(ctx context.Context, actor kanban.Actor, in ruleIn) (any, error) {
	return finished(a.core.DeleteRule(ctx, actor, in.RuleID))
}
