package kanban

import "time"

// Board 看板，是列、任务、自动化规则和事件的归属根。
// CardSeq 记录已经分配出去的最后一个任务编号，只在持有看板行锁的事务里递增，
// 所以同一个看板里的任务编号不会重复，也不会回收。
type Board struct {
	ID          int64       `gorm:"primaryKey" json:"id"`
	Name        string      `gorm:"column:name;size:100;notNull;default:'';comment:看板名称" json:"name"`
	Slug        string      `gorm:"column:slug;size:10;notNull;default:'';uniqueIndex:uq_boards_slug;comment:看板短标识，任务编号的前缀，创建后不可修改" json:"slug"`
	Description string      `gorm:"column:description;type:text;notNull;default:'';comment:看板说明" json:"description"`
	ProjectPath string      `gorm:"column:project_path;size:512;notNull;default:'';comment:看板关联的本地项目路径" json:"project_path"`
	Tags        []*Tag      `gorm:"-" json:"tags"`      // 标签库，存在 tags 表里，由 Core 读出看板时填入
	Engineers   []*Engineer `gorm:"-" json:"engineers"` // 工程师名册，存在 board_engineers 表里，由 Core 读出看板时填入
	CardSeq     int64       `gorm:"column:card_seq;notNull;default:0;comment:已经分配到的最后一个任务编号" json:"-"`
	CreatedAt   time.Time   `gorm:"column:created_at;notNull;default:CURRENT_TIMESTAMP;comment:创建时间" json:"created_at"`
	UpdatedAt   time.Time   `gorm:"column:updated_at;notNull;default:CURRENT_TIMESTAMP;comment:更新时间" json:"updated_at"`
	// LastActivityAt 是最后活动时间：看板自身更新时间和最新事件时间里较晚的那个，不存库，列表读取时算出。
	LastActivityAt time.Time `gorm:"-" json:"last_activity_at"`
}

// TableName 指定表名，这是 GORM 约定的方法。
func (*Board) TableName() string {
	return "boards"
}

// Normalize 把空的标签库和工程师名册统一成空切片，保证 JSON 里是 [] 而不是 null。
// 存储层每次读出或写入看板时都要调用它。
func (b *Board) Normalize() {
	if b.Tags == nil {
		b.Tags = []*Tag{}
	}
	if b.Engineers == nil {
		b.Engineers = []*Engineer{}
	}
	if b.LastActivityAt.IsZero() {
		b.LastActivityAt = b.UpdatedAt
	}
}
