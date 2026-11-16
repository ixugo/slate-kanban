package kanban

import "time"

// Board 看板，是列、卡片、自动化规则和事件的归属根。
// CardSeq 记录已经分配出去的最后一个卡片编号，只在持有看板行锁的事务里递增，
// 所以同一个看板里的卡片编号不会重复，也不会回收。
type Board struct {
	ID          int64     `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"column:name;size:100;notNull;default:'';comment:看板名称" json:"name"`
	Slug        string    `gorm:"column:slug;size:10;notNull;default:'';uniqueIndex:uq_boards_slug;comment:看板短标识，卡片编号的前缀，创建后不可修改" json:"slug"`
	Description string    `gorm:"column:description;type:text;notNull;default:'';comment:看板说明" json:"description"`
	ProjectPath string    `gorm:"column:project_path;size:512;notNull;default:'';comment:看板关联的本地项目路径" json:"project_path"`
	CardSeq     int64     `gorm:"column:card_seq;notNull;default:0;comment:已经分配到的最后一个卡片编号" json:"-"`
	CreatedAt   time.Time `gorm:"column:created_at;notNull;default:CURRENT_TIMESTAMP;comment:创建时间" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;notNull;default:CURRENT_TIMESTAMP;comment:更新时间" json:"updated_at"`
}

// TableName 指定表名，这是 GORM 约定的方法。
func (*Board) TableName() string {
	return "boards"
}
