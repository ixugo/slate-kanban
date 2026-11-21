package kanban

import "time"

// CardFieldDescription 是变更日志里「描述」字段的名称，和任务 JSON 里的字段名一致。
const CardFieldDescription = "description"

// CardChange 是任务某个字段的一次修改记录：改之前和改之后的完整内容各存一份，
// 网页据此像 git 那样左右对比两个版本。目前只记描述，因为描述是长文本，
// 光写一句「修改了描述」看不出到底改了什么。记录只增不改，随任务或看板一起删除。
type CardChange struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	BoardID   int64     `gorm:"column:board_id;notNull;default:0;index:idx_card_changes_board;comment:所属看板" json:"board_id"`
	CardID    int64     `gorm:"column:card_id;notNull;default:0;index:idx_card_changes_card;comment:所属任务" json:"card_id"`
	Field     string    `gorm:"column:field;size:32;notNull;default:'';comment:被修改的字段名，目前只有 description" json:"field"`
	OldValue  string    `gorm:"column:old_value;type:text;notNull;default:'';comment:修改前的内容" json:"old_value"`
	NewValue  string    `gorm:"column:new_value;type:text;notNull;default:'';comment:修改后的内容" json:"new_value"`
	ActorKind string    `gorm:"column:actor_kind;size:16;notNull;default:'';comment:修改者类型：human、agent、system" json:"actor_kind"`
	Actor     string    `gorm:"column:actor;size:96;notNull;default:'';comment:修改者名称" json:"actor"`
	CreatedAt time.Time `gorm:"column:created_at;notNull;default:CURRENT_TIMESTAMP;comment:修改时间" json:"created_at"`
}

// TableName 指定表名，这是 GORM 约定的方法。
func (*CardChange) TableName() string {
	return "card_changes"
}
