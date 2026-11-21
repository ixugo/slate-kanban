package kanbandb

import (
	"context"

	"github.com/ixugo/goddd/pkg/orm"

	"slate-kanban/internal/core/kanban"
)

var _ kanban.WebhookStorer = Webhook{}

// Webhook 是 webhook 配置的数据库实现。
type Webhook DB

// WithTx 返回绑定到事务的副本。
func (d Webhook) WithTx(tx orm.Tx) kanban.WebhookStorer {
	return Webhook{db: orm.GormDB(tx)}
}

// Create 新增一条 webhook。
func (d Webhook) Create(ctx context.Context, model *kanban.Webhook) error {
	return d.db.WithContext(ctx).Create(model).Error
}

// Save 把整条记录写回数据库。
func (d Webhook) Save(ctx context.Context, model *kanban.Webhook) error {
	return d.db.WithContext(ctx).Save(model).Error
}

// Delete 按主键删除，幂等。
func (d Webhook) Delete(ctx context.Context, id int64) error {
	res := d.db.WithContext(ctx).Delete(&kanban.Webhook{}, id)
	return res.Error
}

// GetByID 按主键读取。
func (d Webhook) GetByID(ctx context.Context, id int64) (*kanban.Webhook, error) {
	var out kanban.Webhook
	if err := d.db.WithContext(ctx).Where("id = ?", id).Take(&out).Error; err != nil {
		return nil, err
	}
	out.Normalize()
	return &out, nil
}

// ListByBoard 返回看板的全部 webhook。
func (d Webhook) ListByBoard(ctx context.Context, boardID int64) ([]*kanban.Webhook, error) {
	items := make([]*kanban.Webhook, 0)
	err := d.db.WithContext(ctx).Where("board_id = ?", boardID).Order("id ASC").Find(&items).Error
	for _, w := range items {
		w.Normalize()
	}
	return items, err
}

// DeleteByBoard 删除看板的全部 webhook。
func (d Webhook) DeleteByBoard(ctx context.Context, boardID int64) error {
	return d.db.WithContext(ctx).Where("board_id = ?", boardID).Delete(&kanban.Webhook{}).Error
}
