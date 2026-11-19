package kanbandb

import (
	"context"

	"github.com/ixugo/goddd/pkg/orm"

	"slate-kanban/internal/core/kanban"
)

var _ kanban.EngineerStorer = Engineer{}

// Engineer 是工程师的数据库实现，和 DB 同一个底层结构，通过类型转换得到。
type Engineer DB

// WithTx 返回绑定到事务的副本。
func (d Engineer) WithTx(tx orm.Tx) kanban.EngineerStorer {
	return Engineer{db: orm.GormDB(tx)}
}

// Create 新增一个工程师。
func (d Engineer) Create(ctx context.Context, model *kanban.Engineer) error {
	return d.db.WithContext(ctx).Create(model).Error
}

// Save 把整个工程师写回数据库。model.ID 必须非零，否则 panic，防止 Save 变成新增或误改。
func (d Engineer) Save(ctx context.Context, model *kanban.Engineer) error {
	if model.ID == 0 {
		panic("kanbandb.Engineer.Save: model.ID is zero, primary key is required")
	}
	return d.db.WithContext(ctx).Save(model).Error
}

// Delete 按主键删除，幂等。model.ID 必须非零，否则 panic。
func (d Engineer) Delete(ctx context.Context, model *kanban.Engineer) error {
	if model.ID == 0 {
		panic("kanbandb.Engineer.Delete: model.ID is zero, primary key is required")
	}
	return d.db.WithContext(ctx).Delete(model).Error
}

// ListByBoard 返回看板的全部工程师，按 Sort、主键升序。没有工程师时返回空切片。
func (d Engineer) ListByBoard(ctx context.Context, boardID int64) ([]*kanban.Engineer, error) {
	items := make([]*kanban.Engineer, 0)
	if err := d.db.WithContext(ctx).Where("board_id = ?", boardID).Order("sort ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// DeleteByBoard 删除看板的全部工程师。
func (d Engineer) DeleteByBoard(ctx context.Context, boardID int64) error {
	return d.db.WithContext(ctx).Where("board_id = ?", boardID).Delete(&kanban.Engineer{}).Error
}
