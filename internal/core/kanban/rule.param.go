package kanban

// CreateRuleInput 是创建规则的入参。Enabled 不填按启用处理；
// ColumnID 为 0 表示任意列；Condition 为空表示无条件，语法见任务筛选语法。
type CreateRuleInput struct {
	Name      string       `json:"name"`
	Enabled   *bool        `json:"enabled"`
	Trigger   string       `json:"trigger"`
	ColumnID  int64        `json:"column_id"`
	Condition string       `json:"condition"`
	Actions   []RuleAction `json:"actions"`
}

// UpdateRuleInput 是修改规则的入参，指针字段为 nil 表示不修改。
// Actions 传空数组会被拒绝（规则至少要有一个动作），要去掉规则请用删除或者停用。
type UpdateRuleInput struct {
	Name      *string       `json:"name"`
	Enabled   *bool         `json:"enabled"`
	Trigger   *string       `json:"trigger"`
	ColumnID  *int64        `json:"column_id"`
	Condition *string       `json:"condition"`
	Actions   *[]RuleAction `json:"actions"`
}

// apply 把出现的字段写到规则上，不做校验，校验由随后的 normalizeRule 完成。
func (in UpdateRuleInput) apply(r *Rule) {
	if in.Name != nil {
		r.Name = *in.Name
	}
	if in.Enabled != nil {
		r.Enabled = *in.Enabled
	}
	if in.Trigger != nil {
		r.TriggerType = *in.Trigger
	}
	if in.ColumnID != nil {
		r.ColumnID = *in.ColumnID
	}
	if in.Condition != nil {
		r.Condition = *in.Condition
	}
	if in.Actions != nil {
		r.Actions = *in.Actions
	}
}
