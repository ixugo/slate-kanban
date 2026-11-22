package kanban

import "time"

// Rule 自动化规则：当某件事发生（触发），并且任务满足条件时，按顺序执行一组动作。
// TriggerType 的取值是 card_created、card_entered、card_left；
// ColumnID 为 0 表示任意列；Condition 使用任务筛选语法，为空表示无条件。
type Rule struct {
	ID          int64        `gorm:"primaryKey" json:"id"`
	BoardID     int64        `gorm:"column:board_id;notNull;default:0;index:idx_rules_board_sort,priority:1;comment:所属看板" json:"board_id"`
	Name        string       `gorm:"column:name;size:100;notNull;default:'';comment:规则名称" json:"name"`
	Enabled     bool         `gorm:"column:enabled;notNull;default:FALSE;comment:是否启用" json:"enabled"`
	TriggerType string       `gorm:"column:trigger_type;size:32;notNull;default:'';comment:触发类型：card_created、card_entered、card_left" json:"trigger"`
	ColumnID    int64        `gorm:"column:column_id;notNull;default:0;comment:触发所在的列，0 表示任意列" json:"column_id"`
	Condition   string       `gorm:"column:condition;size:500;notNull;default:'';comment:条件，使用任务筛选语法，空表示无条件" json:"condition"`
	Actions     []RuleAction `gorm:"column:actions;type:text;serializer:json;notNull;default:'[]';comment:动作列表，JSON 数组" json:"actions"`
	Sort        int64        `gorm:"column:sort;notNull;default:0;index:idx_rules_board_sort,priority:2;comment:排序值，越小越先执行" json:"sort"`
	CreatedAt   time.Time    `gorm:"column:created_at;notNull;default:CURRENT_TIMESTAMP;comment:创建时间" json:"created_at"`
	UpdatedAt   time.Time    `gorm:"column:updated_at;notNull;default:CURRENT_TIMESTAMP;comment:更新时间" json:"updated_at"`
}

// RuleAction 是规则里的一个动作。所有动作共用同一个结构，
// 每种动作只会用到其中的一两个字段，具体见自动化设计文档里的动作表。
type RuleAction struct {
	Type     string `json:"type"`
	Mode     string `json:"mode,omitempty"`
	Tag      string `json:"tag,omitempty"`
	Text     string `json:"text,omitempty"`
	ColumnID int64  `json:"column_id,omitempty"`
	Days     int    `json:"days,omitempty"`
	Priority int    `json:"priority,omitempty"`
	Flag     bool   `json:"flag,omitempty"`
}

// TableName 指定表名，这是 GORM 约定的方法。
func (*Rule) TableName() string {
	return "rules"
}

// Normalize 把空的动作列表统一成空切片，保证 JSON 里是 [] 而不是 null。
// 存储层每次读出规则后都要调用它。
func (r *Rule) Normalize() {
	if r.Actions == nil {
		r.Actions = []RuleAction{}
	}
}
