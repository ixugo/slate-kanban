package kanbandb

import (
	"context"

	"github.com/ixugo/goddd/pkg/orm"

	"slate-kanban/internal/core/kanban"
)

var _ kanban.CommentStorer = Comment{}

// Comment 是卡片评论的数据库实现，和 DB 同一个底层结构，通过类型转换得到。
type Comment DB

// WithTx 返回绑定到事务的副本。
func (d Comment) WithTx(tx orm.Tx) kanban.CommentStorer {
	return Comment{db: orm.GormDB(tx)}
}

// Create 新增一条评论。
func (d Comment) Create(ctx context.Context, model *kanban.Comment) error {
	return d.db.WithContext(ctx).Create(model).Error
}

// Delete 按主键删除，幂等。model.ID 必须非零，否则 panic。
func (d Comment) Delete(ctx context.Context, model *kanban.Comment) error {
	if model.ID == 0 {
		panic("kanbandb.Comment.Delete: model.ID is zero, primary key is required")
	}
	return d.db.WithContext(ctx).Delete(model).Error
}

// GetByID 按主键读取。
func (d Comment) GetByID(ctx context.Context, id int64) (*kanban.Comment, error) {
	var out kanban.Comment
	if err := d.db.WithContext(ctx).Where("id = ?", id).Take(&out).Error; err != nil {
		return nil, err
	}
	return &out, nil
}

// ListByCard 返回卡片的评论，按主键升序，也就是写入的先后。
func (d Comment) ListByCard(ctx context.Context, cardID int64) ([]*kanban.Comment, error) {
	items := make([]*kanban.Comment, 0)
	err := d.db.WithContext(ctx).Where("card_id = ?", cardID).Order("id ASC").Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// ListByBoard 返回看板下全部评论，按卡片、评论主键升序，供整板导出一次加载。
func (d Comment) ListByBoard(ctx context.Context, boardID int64) ([]*kanban.Comment, error) {
	items := make([]*kanban.Comment, 0)
	err := d.db.WithContext(ctx).Where("board_id = ?", boardID).Order("card_id ASC, id ASC").Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// DeleteByCard 删除某张卡片的全部评论。
func (d Comment) DeleteByCard(ctx context.Context, cardID int64) error {
	return d.db.WithContext(ctx).Where("card_id = ?", cardID).Delete(&kanban.Comment{}).Error
}

// DeleteByBoard 删除看板的全部评论。
func (d Comment) DeleteByBoard(ctx context.Context, boardID int64) error {
	return d.db.WithContext(ctx).Where("board_id = ?", boardID).Delete(&kanban.Comment{}).Error
}
