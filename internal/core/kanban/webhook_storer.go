package kanban

import (
	"context"

	"github.com/ixugo/goddd/pkg/orm"
)

// WebhookStorer 是 webhook 配置的数据持久化接口。
type WebhookStorer interface {
	// WithTx 返回绑定到事务的副本，事务内的所有读写都必须用它。
	WithTx(orm.Tx) WebhookStorer
	// Create 新增一条 webhook。
	Create(context.Context, *Webhook) error
	// Save 把整条记录写回数据库。
	Save(context.Context, *Webhook) error
	// Delete 按主键删除，幂等。
	Delete(context.Context, int64) error
	// GetByID 按主键读取，不存在时返回可用 orm.IsErrRecordNotFound 判断的错误。
	GetByID(context.Context, int64) (*Webhook, error)
	// ListByBoard 返回看板的全部 webhook，按主键升序。
	ListByBoard(context.Context, int64) ([]*Webhook, error)
	// DeleteByBoard 删除看板的全部 webhook，用于删除看板。
	DeleteByBoard(context.Context, int64) error
}
