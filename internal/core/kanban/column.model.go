package kanban

import "time"

// Column 看板的列。列由用户自己创建，所以它除了名字，还带三类信息：
// 给人看的说明 Description，给 agent 看的指令 Instruction，
// 以及三个驱动流程的开关：AgentPickup（agent 的 next_card 默认从哪些列取卡，不限制 agent 的操作）、
// IsDone（进入这一列算完成）、NextColumnID（做完以后默认去哪一列）。
type Column struct {
	ID           int64     `gorm:"primaryKey" json:"id"`
	BoardID      int64     `gorm:"column:board_id;notNull;default:0;index:idx_board_columns_board_sort,priority:1;comment:所属看板" json:"board_id"`
	Name         string    `gorm:"column:name;size:60;notNull;default:'';comment:列名称" json:"name"`
	Description  string    `gorm:"column:description;size:500;notNull;default:'';comment:列说明，给人看" json:"description"`
	Instruction  string    `gorm:"column:instruction;type:text;notNull;default:'';comment:给 agent 的指令，说明这一列的卡片该怎么处理" json:"instruction"`
	AgentPickup  bool      `gorm:"column:agent_pickup;notNull;default:FALSE;comment:是否允许 agent 领取这一列的卡片" json:"agent_pickup"`
	IsDone       bool      `gorm:"column:is_done;notNull;default:FALSE;comment:卡片进入这一列是否算完成" json:"is_done"`
	NextColumnID int64     `gorm:"column:next_column_id;notNull;default:0;comment:卡片完成后默认移入的下一列，0 表示没有" json:"next_column_id"`
	Sort         int64     `gorm:"column:sort;notNull;default:0;index:idx_board_columns_board_sort,priority:2;comment:排序值，越小越靠左" json:"sort"`
	CreatedAt    time.Time `gorm:"column:created_at;notNull;default:CURRENT_TIMESTAMP;comment:创建时间" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at;notNull;default:CURRENT_TIMESTAMP;comment:更新时间" json:"updated_at"`
}

// TableName 指定表名。不用默认的 columns，是为了避开各数据库里同名的系统表或关键字。
func (*Column) TableName() string {
	return "board_columns"
}
