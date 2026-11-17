package kanban

// CreateRuleInput 是创建规则的入参。Enabled 不填按启用处理；
// ColumnID 为 0 表示任意列；Condition 为空表示无条件，语法见卡片筛选语法。
type CreateRuleInput struct {
	Name      string       `json:"name"`
	Enabled   *bool        `json:"enabled"`
	Trigger   string       `json:"trigger"`
	ColumnID  int64        `json:"column_id"`
	Condition string       `json:"condition"`
	Actions   []RuleAction `json:"actions"`
}
