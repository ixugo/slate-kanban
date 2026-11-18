package kanban

import "time"

// Attachment 是粘贴进 Markdown 的图片。图片字节直接存数据库，随数据库一起备份，
// 不需要额外的文件目录和配置；Token 是随机的公开编号，Markdown 里引用的就是它。
// 图片只能新增，不能修改；整个看板删除时一并删除。
type Attachment struct {
	ID          int64     `gorm:"primaryKey" json:"id"`
	BoardID     int64     `gorm:"column:board_id;notNull;default:0;index:idx_attachments_board;comment:所属看板" json:"board_id"`
	Token       string    `gorm:"column:token;size:32;notNull;default:'';uniqueIndex:uq_attachments_token;comment:随机公开编号，Markdown 里按它引用" json:"token"`
	ContentType string    `gorm:"column:content_type;size:32;notNull;default:'';comment:图片类型，由内容嗅探得出" json:"content_type"`
	Size        int64     `gorm:"column:size;notNull;default:0;comment:字节数" json:"size"`
	Data        []byte    `gorm:"column:data;notNull;comment:图片内容" json:"-"`
	CreatedAt   time.Time `gorm:"column:created_at;notNull;default:CURRENT_TIMESTAMP;comment:创建时间" json:"created_at"`
}

// TableName 指定表名，这是 GORM 约定的方法。
func (*Attachment) TableName() string {
	return "attachments"
}
