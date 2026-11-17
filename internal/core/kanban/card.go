package kanban

import (
	"context"

	"github.com/ixugo/goddd/pkg/orm"
)

// CardStorer 是卡片的数据持久化接口。读出的卡片都已经调用过 Normalize。
// 卡片的修改都发生在已经锁住看板行的事务里，所以更新直接用 Save，不再重复加行锁。
type CardStorer interface {
	// WithTx 返回绑定到事务的副本，事务内的所有读写都必须用它。
	WithTx(orm.Tx) CardStorer
	// Create 新增一张卡片。Key 重复时返回可用 orm.IsDuplicatedKey 判断的错误。
	Create(context.Context, *Card) error
	// Save 把整张卡片写回数据库，布尔值和 0 值也会写入。
	Save(context.Context, *Card) error
	// Delete 按主键删除，幂等：卡片不存在也返回成功。
	Delete(context.Context, *Card) error
	// GetByID 按主键读取，不存在时返回可用 orm.IsErrRecordNotFound 判断的错误。
	GetByID(context.Context, int64) (*Card, error)
	// GetByKey 按卡片编号（例如 SLATE-12）读取，不存在时错误同 GetByID。
	GetByKey(context.Context, string) (*Card, error)
	// ListByBoard 返回看板的全部卡片，按所在列的主键、位置、主键升序。
	ListByBoard(context.Context, int64) ([]*Card, error)
	// ListByColumn 返回某一列的卡片，按位置升序，位置相同按主键升序。
	ListByColumn(context.Context, int64) ([]*Card, error)
	// CountByColumn 返回某一列有几张卡片。
	CountByColumn(context.Context, int64) (int64, error)
	// MaxPosition 返回某一列里最大的位置，空列返回 0。
	MaxPosition(context.Context, int64) (float64, error)
	// SetPosition 只更新位置，用于整列重排。
	SetPosition(ctx context.Context, id int64, position float64) error
	// DeleteByBoard 删除看板的全部卡片，用于删除看板。
	DeleteByBoard(context.Context, int64) error
}
