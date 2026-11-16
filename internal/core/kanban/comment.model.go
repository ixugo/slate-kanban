package kanban

import "time"

// Comment 卡片评论。人、agent 和自动化规则都可以写，AuthorKind 区分来源。
// 评论只能新增和删除，不能修改，这样 agent 读到的历史不会被悄悄改写。
type Comment struct {
	ID         int64     `gorm:"primaryKey" json:"id"`
	BoardID    int64     `gorm:"column:board_id;notNull;default:0;index:idx_comments_board;comment:所属看板" json:"board_id"`
	CardID     int64     `gorm:"column:card_id;notNull;default:0;index:idx_comments_card;comment:所属卡片" json:"card_id"`
	AuthorKind string    `gorm:"column:author_kind;size:16;notNull;default:'';comment:作者类型：human、agent、system" json:"author_kind"`
	Author     string    `gorm:"column:author;size:64;notNull;default:'';comment:作者名称" json:"author"`
	Body       string    `gorm:"column:body;type:text;notNull;default:'';comment:评论正文" json:"body"`
	CreatedAt  time.Time `gorm:"column:created_at;notNull;default:CURRENT_TIMESTAMP;comment:创建时间" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;notNull;default:CURRENT_TIMESTAMP;comment:更新时间" json:"updated_at"`
}

// TableName 指定表名，这是 GORM 约定的方法。
func (*Comment) TableName() string {
	return "comments"
}
