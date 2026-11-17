package kanbandb

import (
	"context"

	"github.com/ixugo/goddd/pkg/orm"

	"slate-kanban/internal/core/kanban"
)

var _ kanban.ColumnStorer = Column{}

// Column 是看板列的数据库实现，和 DB 同一个底层结构，通过类型转换得到。
type Column DB

// WithTx 返回绑定到事务的副本。
func (d Column) WithTx(tx orm.Tx) kanban.ColumnStorer {
	return Column{db: orm.GormDB(tx)}
}

// Create 新增一列。
func (d Column) Create(ctx context.Context, model *kanban.Column) error {
	return d.db.WithContext(ctx).Create(model).Error
}

// Save 把整列写回数据库。model.ID 必须非零，否则 panic，防止 Save 变成新增或误改。
func (d Column) Save(ctx context.Context, model *kanban.Column) error {
	if model.ID == 0 {
		panic("kanbandb.Column.Save: model.ID is zero, primary key is required")
	}
	return d.db.WithContext(ctx).Save(model).Error
}

// Delete 按主键删除，幂等。model.ID 必须非零，否则 panic。
func (d Column) Delete(ctx context.Context, model *kanban.Column) error {
	if model.ID == 0 {
		panic("kanbandb.Column.Delete: model.ID is zero, primary key is required")
	}
	return d.db.WithContext(ctx).Delete(model).Error
}

// GetByID 按主键读取。
func (d Column) GetByID(ctx context.Context, id int64) (*kanban.Column, error) {
	var out kanban.Column
	if err := d.db.WithContext(ctx).Where("id = ?", id).Take(&out).Error; err != nil {
		return nil, err
	}
	return &out, nil
}

// ListByBoard 返回看板的全部列，按排序值升序；排序值相同时按主键升序，保证顺序稳定。
func (d Column) ListByBoard(ctx context.Context, boardID int64) ([]*kanban.Column, error) {
	items := make([]*kanban.Column, 0)
	err := d.db.WithContext(ctx).Where("board_id = ?", boardID).Order("sort ASC, id ASC").Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// SetSort 只更新排序值。用 UpdateColumn 是为了不改动 updated_at：
// 调整顺序不算列内容的修改。
func (d Column) SetSort(ctx context.Context, id, sort int64) error {
	return d.db.WithContext(ctx).Model(&kanban.Column{}).Where("id = ?", id).
		UpdateColumn("sort", sort).Error
}

// ClearNextColumn 把同一看板里指向 columnID 的下一列引用清成 0。
func (d Column) ClearNextColumn(ctx context.Context, boardID, columnID int64) error {
	return d.db.WithContext(ctx).Model(&kanban.Column{}).
		Where("board_id = ? AND next_column_id = ?", boardID, columnID).
		UpdateColumn("next_column_id", 0).Error
}

// DeleteByBoard 删除看板的全部列。
func (d Column) DeleteByBoard(ctx context.Context, boardID int64) error {
	return d.db.WithContext(ctx).Where("board_id = ?", boardID).Delete(&kanban.Column{}).Error
}
