package kanban

import (
	"context"

	"github.com/ixugo/goddd/pkg/orm"
)

// columnSortStep 相邻两列排序值的间距。用间距而不是连续整数，
// 是为了新增列时直接取"最大值加一个间距"，不用改动其他列。
const columnSortStep = 1024

// ColumnStorer 是看板列的数据持久化接口。
// 列的修改都发生在已经锁住看板行的事务里，所以更新直接用 Save，不再重复加行锁。
type ColumnStorer interface {
	// WithTx 返回绑定到事务的副本，事务内的所有读写都必须用它。
	WithTx(orm.Tx) ColumnStorer
	// Create 新增一列。
	Create(context.Context, *Column) error
	// Save 把整列写回数据库，布尔值和 0 值也会写入，用于修改和回填下一列。
	Save(context.Context, *Column) error
	// Delete 按主键删除，幂等：列不存在也返回成功。
	Delete(context.Context, *Column) error
	// GetByID 按主键读取，不存在时返回可用 orm.IsErrRecordNotFound 判断的错误。
	GetByID(context.Context, int64) (*Column, error)
	// ListByBoard 返回看板的全部列，按 Sort 升序，Sort 相同按主键升序。
	ListByBoard(context.Context, int64) ([]*Column, error)
	// SetSort 只更新排序值，用于调整列顺序。
	SetSort(ctx context.Context, id, sort int64) error
	// ClearNextColumn 把同一看板里所有指向 columnID 的下一列引用清成 0。
	ClearNextColumn(ctx context.Context, boardID, columnID int64) error
	// DeleteByBoard 删除看板的全部列，用于删除看板。
	DeleteByBoard(context.Context, int64) error
}
