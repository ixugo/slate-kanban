package kanbandb

import (
	"context"

	"github.com/ixugo/goddd/pkg/orm"

	"slate-kanban/internal/core/kanban"
)

var _ kanban.CardChangeStorer = CardChange{}

// CardChange 是卡片变更日志的数据库实现，和 DB 同一个底层结构，通过类型转换得到。
type CardChange DB

// WithTx 返回绑定到事务的副本。
func (d CardChange) WithTx(tx orm.Tx) kanban.CardChangeStorer {
	return CardChange{db: orm.GormDB(tx)}
}

// Create 新增一条变更记录。
func (d CardChange) Create(ctx context.Context, model *kanban.CardChange) error {
	return d.db.WithContext(ctx).Create(model).Error
}

// GetByID 按主键读取。
func (d CardChange) GetByID(ctx context.Context, id int64) (*kanban.CardChange, error) {
	var out kanban.CardChange
	if err := d.db.WithContext(ctx).Where("id = ?", id).Take(&out).Error; err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteByCard 删除某张卡片的全部变更记录。
func (d CardChange) DeleteByCard(ctx context.Context, cardID int64) error {
	return d.db.WithContext(ctx).Where("card_id = ?", cardID).Delete(&kanban.CardChange{}).Error
}

// DeleteByBoard 删除看板的全部变更记录。
func (d CardChange) DeleteByBoard(ctx context.Context, boardID int64) error {
	return d.db.WithContext(ctx).Where("board_id = ?", boardID).Delete(&kanban.CardChange{}).Error
}
