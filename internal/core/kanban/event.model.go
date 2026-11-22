package kanban

import "time"

// Event 事件日志，只追加不修改。ID 是全局递增的序号，也就是 agent 游标和网页实时流用的 seq。
// Payload 是一段 JSON，只放展示和判断需要的字段，不放任何自然语言，
// 网页按 Type 和 Payload 在前端拼出各语言的文案。
type Event struct {
	ID        int64     `gorm:"primaryKey;index:idx_events_board_seq,priority:2" json:"id"`
	BoardID   int64     `gorm:"column:board_id;notNull;default:0;index:idx_events_board_seq,priority:1;comment:所属看板" json:"board_id"`
	CardID    int64     `gorm:"column:card_id;notNull;default:0;index:idx_events_card;comment:关联的任务，0 表示与任务无关" json:"card_id"`
	Type      string    `gorm:"column:type;size:48;notNull;default:'';comment:事件类型，例如 card.moved" json:"type"`
	ActorKind string    `gorm:"column:actor_kind;size:16;notNull;default:'';comment:操作者类型：human、agent、system" json:"actor_kind"`
	Actor     string    `gorm:"column:actor;size:96;notNull;default:'';comment:操作者名称" json:"actor"`
	Payload   string    `gorm:"column:payload;type:text;notNull;default:'';comment:事件内容，JSON 对象" json:"payload"`
	CreatedAt time.Time `gorm:"column:created_at;notNull;default:CURRENT_TIMESTAMP;comment:创建时间" json:"created_at"`
}

// TableName 指定表名，这是 GORM 约定的方法。
func (*Event) TableName() string {
	return "events"
}

// AgentCursor 记录某个 agent 在某个看板上读到了哪个事件序号。
// 同一个看板、同一个 agent 名只有一行，LastSeq 只增不减。
type AgentCursor struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	BoardID   int64     `gorm:"column:board_id;notNull;default:0;uniqueIndex:uq_agent_cursors_board_agent,priority:1;comment:所属看板" json:"board_id"`
	Agent     string    `gorm:"column:agent;size:64;notNull;default:'';uniqueIndex:uq_agent_cursors_board_agent,priority:2;comment:agent 名称" json:"agent"`
	LastSeq   int64     `gorm:"column:last_seq;notNull;default:0;comment:已经确认读取到的最后一个事件序号" json:"last_seq"`
	CreatedAt time.Time `gorm:"column:created_at;notNull;default:CURRENT_TIMESTAMP;comment:创建时间" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;notNull;default:CURRENT_TIMESTAMP;comment:更新时间" json:"updated_at"`
}

// TableName 指定表名，这是 GORM 约定的方法。
func (*AgentCursor) TableName() string {
	return "agent_cursors"
}
