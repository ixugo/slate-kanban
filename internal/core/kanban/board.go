package kanban

import (
	"context"

	"github.com/ixugo/goddd/pkg/orm"
)

// BoardStorer 是看板的数据持久化接口。
type BoardStorer interface {
	// WithTx 返回绑定到事务的副本，事务内的所有读写都必须用它。
	WithTx(orm.Tx) BoardStorer
	// Create 新增一个看板。slug 重复时返回可用 orm.IsDuplicatedKey 判断的错误。
	Create(context.Context, *Board) error
	// Save 把整个看板写回数据库，布尔值和 0 值也会写入。
	// 调用方应该已经通过事务执行器锁住了看板行，所以这里不再重复加锁。
	Save(context.Context, *Board) error
	// Delete 按主键删除，幂等：看板不存在也返回成功。
	Delete(context.Context, *Board) error
	// GetByID 按主键读取，不存在时返回可用 orm.IsErrRecordNotFound 判断的错误。
	GetByID(context.Context, int64) (*Board, error)
	// GetBySlug 按 slug 读取，不存在时错误同 GetByID。
	GetBySlug(context.Context, string) (*Board, error)
	// List 返回全部看板，按创建顺序排列。看板数量很少，所以不分页。
	List(context.Context) ([]*Board, error)
	// Lock 锁住看板行并返回最新数据，是同一看板的写事务排队的入口。
	// 只有 PostgreSQL 会真正加行锁；SQLite 只有一个连接，写事务本来就是串行的。
	Lock(context.Context, int64) (*Board, error)
	// IncrCardSeq 把卡片编号计数加一并返回新值，看板不存在时返回找不到记录的错误。
	IncrCardSeq(context.Context, int64) (int64, error)
}
