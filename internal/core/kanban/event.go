package kanban

import (
	"context"

	"github.com/ixugo/goddd/pkg/orm"
)

// 事件类型。每种类型的 Payload 字段见设计文档 .cursor/scopes/kanban.event.go.md。
const (
	EventBoardCreated = "board.created"
	EventBoardUpdated = "board.updated"
	// EventBoardLabelsUpdated 看板标签库（名称、颜色）有变化；单独成类型，网页的活动流可以用 exclude_types 把它排除。
	EventBoardLabelsUpdated = "board.labels_updated"
	EventColumnCreated      = "column.created"
	EventColumnUpdated      = "column.updated"
	EventColumnDeleted      = "column.deleted"
	EventColumnReordered    = "column.reordered"
	EventCardCreated        = "card.created"
	EventCardUpdated        = "card.updated"
	// EventCardTagsUpdated 卡片上的标签有变化；单独成类型，原因同 EventBoardLabelsUpdated。
	EventCardTagsUpdated = "card.tags_updated"
	EventCardMoved       = "card.moved"
	EventCardReordered   = "card.reordered"
	EventCardDeleted     = "card.deleted"
	EventCardClaimed     = "card.claimed"
	EventCardReleased    = "card.released"
	EventCommentAdded    = "comment.added"
	EventCommentDeleted  = "comment.deleted"
	EventRuleCreated     = "rule.created"
	EventRuleUpdated     = "rule.updated"
	EventRuleDeleted     = "rule.deleted"
	EventAutomationFired = "automation.fired"
)

// EventStorer 是事件日志的数据持久化接口。事件只追加，不修改。
type EventStorer interface {
	// WithTx 返回绑定到事务的副本，事务内的所有读写都必须用它。
	WithTx(orm.Tx) EventStorer
	// Create 追加一条事件，成功后事件的 ID 就是它的序号。
	Create(context.Context, *Event) error
	// ListAfter 返回看板里序号大于 afterSeq 的事件，按序号升序，最多 limit 条。
	ListAfter(ctx context.Context, boardID, afterSeq int64, limit int) ([]*Event, error)
	// ListBefore 返回看板里序号小于 beforeSeq（0 表示不限）且满足筛选条件的事件，按序号降序（最新的在前），最多 limit 条。
	ListBefore(ctx context.Context, boardID, beforeSeq int64, filter ActivityFilter, limit int) ([]*Event, error)
	// Actors 返回看板事件里出现过的不同操作者（类型加名称），按类型、名称排序。
	Actors(ctx context.Context, boardID int64) ([]ActivityActor, error)
	// ListByCard 返回某张卡片的事件，按序号降序（最新的在前），最多 limit 条。
	ListByCard(ctx context.Context, cardID int64, limit int) ([]*Event, error)
	// LatestSeq 返回看板里最大的事件序号，没有事件时返回 0。
	LatestSeq(ctx context.Context, boardID int64) (int64, error)
	// MinSeq 返回看板里还保留着的最小事件序号，没有事件时返回 0。
	MinSeq(ctx context.Context, boardID int64) (int64, error)
	// Prune 只保留看板里最新的 keep 条事件，更早的删除。
	Prune(ctx context.Context, boardID int64, keep int) error
	// DeleteByBoard 删除看板的全部事件，用于删除看板。
	DeleteByBoard(context.Context, int64) error
}

// CursorStorer 是 agent 读取进度的数据持久化接口。
type CursorStorer interface {
	// WithTx 返回绑定到事务的副本。
	WithTx(orm.Tx) CursorStorer
	// Get 读取某个 agent 在某个看板上的进度，没有记录时返回可用 orm.IsErrRecordNotFound 判断的错误。
	Get(ctx context.Context, boardID int64, agent string) (*AgentCursor, error)
	// Advance 把进度推进到 seq 并返回推进后的值。进度只增不减：
	// seq 不大于已有进度时保持不变；还没有记录时新建一条。
	Advance(ctx context.Context, boardID int64, agent string, seq int64) (int64, error)
	// DeleteByBoard 删除看板的全部进度，用于删除看板。
	DeleteByBoard(context.Context, int64) error
}
