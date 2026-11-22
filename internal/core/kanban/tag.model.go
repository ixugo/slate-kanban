package kanban

import "time"

// Tag 是看板标签库里的一项：一个颜色加一段文字，各自都可以改。
// 文字可以是空串，表示这个标签只有颜色、没有内容；任务靠主键引用标签（Card.TagIDs），
// 所以改文字、改颜色、重名甚至文字都为空都不会让任务和标签对不上。
// 标签属于一个看板，看板删除时一并删除。
type Tag struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	BoardID   int64     `gorm:"column:board_id;notNull;default:0;index:idx_tags_board_sort,priority:1;comment:所属看板" json:"-"`
	Text      string    `gorm:"column:text;size:30;notNull;default:'';comment:标签文字，可以为空" json:"text"`
	Color     string    `gorm:"column:color;size:16;notNull;default:'';comment:标签颜色，形如 green-2，空表示默认灰色" json:"color"`
	Sort      int64     `gorm:"column:sort;notNull;default:0;index:idx_tags_board_sort,priority:2;comment:在标签库里的顺序，越小越靠前" json:"-"`
	CreatedAt time.Time `gorm:"column:created_at;notNull;default:CURRENT_TIMESTAMP;comment:创建时间" json:"-"`
	UpdatedAt time.Time `gorm:"column:updated_at;notNull;default:CURRENT_TIMESTAMP;comment:更新时间" json:"-"`
}

// TableName 指定表名，这是 GORM 约定的方法。
func (*Tag) TableName() string {
	return "tags"
}

// TagInput 是修改标签库时的一项入参。ID 为 0 表示新增，由服务端分配；
// 非 0 必须是库里已有的主键，文字或颜色不同于原来就是修改。入参的顺序就是标签库的显示顺序。
type TagInput struct {
	ID    int64  `json:"id"`
	Text  string `json:"text"`
	Color string `json:"color"`
}
