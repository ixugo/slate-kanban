package kanbandb

import (
	"context"

	"github.com/ixugo/goddd/pkg/orm"

	"slate-kanban/internal/core/kanban"
)

var _ kanban.TagStorer = Tag{}

// Tag 是标签的数据库实现，和 DB 同一个底层结构，通过类型转换得到。
type Tag DB

// WithTx 返回绑定到事务的副本。
func (d Tag) WithTx(tx orm.Tx) kanban.TagStorer {
	return Tag{db: orm.GormDB(tx)}
}

// Create 新增一个标签。
func (d Tag) Create(ctx context.Context, model *kanban.Tag) error {
	return d.db.WithContext(ctx).Create(model).Error
}

// Save 把整个标签写回数据库。model.ID 必须非零，否则 panic，防止 Save 变成新增或误改。
func (d Tag) Save(ctx context.Context, model *kanban.Tag) error {
	if model.ID == 0 {
		panic("kanbandb.Tag.Save: model.ID is zero, primary key is required")
	}
	return d.db.WithContext(ctx).Save(model).Error
}

// Delete 按主键删除，幂等。model.ID 必须非零，否则 panic。
func (d Tag) Delete(ctx context.Context, model *kanban.Tag) error {
	if model.ID == 0 {
		panic("kanbandb.Tag.Delete: model.ID is zero, primary key is required")
	}
	return d.db.WithContext(ctx).Delete(model).Error
}

// ListByBoard 返回看板的全部标签，按 Sort、主键升序。没有标签时返回空切片。
func (d Tag) ListByBoard(ctx context.Context, boardID int64) ([]*kanban.Tag, error) {
	items := make([]*kanban.Tag, 0)
	if err := d.db.WithContext(ctx).Where("board_id = ?", boardID).Order("sort ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// DeleteByBoard 删除看板的全部标签。
func (d Tag) DeleteByBoard(ctx context.Context, boardID int64) error {
	return d.db.WithContext(ctx).Where("board_id = ?", boardID).Delete(&kanban.Tag{}).Error
}
