package kanban

import "time"

// Column 保存列名称、agent 提示词、完成标记和交接目标。
// 是否处理这一列由 agent 按提示词决定，服务端不存储接手开关。
type Column struct {
	ID           int64     `gorm:"primaryKey" json:"id"`
	BoardID      int64     `gorm:"column:board_id;notNull;default:0;index:idx_board_columns_board_sort,priority:1;comment:所属看板" json:"board_id"`
	Name         string    `gorm:"column:name;size:60;notNull;default:'';comment:列名称" json:"name"`
	Instruction  string    `gorm:"column:instruction;type:text;notNull;default:'';comment:给 agent 的指令，说明这一列的任务该怎么处理" json:"instruction"`
	IsDone       bool      `gorm:"column:is_done;notNull;default:FALSE;comment:任务进入这一列是否算完成" json:"is_done"`
	NextColumnID int64     `gorm:"column:next_column_id;notNull;default:0;comment:任务完成后默认移入的下一列，0 表示没有" json:"next_column_id"`
	Sort         int64     `gorm:"column:sort;notNull;default:0;index:idx_board_columns_board_sort,priority:2;comment:排序值，越小越靠左" json:"sort"`
	CreatedAt    time.Time `gorm:"column:created_at;notNull;default:CURRENT_TIMESTAMP;comment:创建时间" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at;notNull;default:CURRENT_TIMESTAMP;comment:更新时间" json:"updated_at"`
}

// TableName 指定表名。不用默认的 columns，是为了避开各数据库里同名的系统表或关键字。
func (*Column) TableName() string {
	return "board_columns"
}
