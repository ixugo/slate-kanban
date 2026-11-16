package kanban

import (
	"context"

	"github.com/ixugo/goddd/pkg/orm"
)

// CommentStorer 是卡片评论的数据持久化接口。评论只能新增和删除，不能修改。
type CommentStorer interface {
	// WithTx 返回绑定到事务的副本，事务内的所有读写都必须用它。
	WithTx(orm.Tx) CommentStorer
	// Create 新增一条评论。
	Create(context.Context, *Comment) error
	// Delete 按主键删除，幂等：评论不存在也返回成功。
	Delete(context.Context, *Comment) error
	// GetByID 按主键读取，不存在时返回可用 orm.IsErrRecordNotFound 判断的错误。
	GetByID(context.Context, int64) (*Comment, error)
	// ListByCard 返回卡片的评论，按时间从早到晚。
	ListByCard(context.Context, int64) ([]*Comment, error)
	// DeleteByCard 删除某张卡片的全部评论，用于删除卡片。
	DeleteByCard(context.Context, int64) error
	// DeleteByBoard 删除看板的全部评论，用于删除看板。
	DeleteByBoard(context.Context, int64) error
}
