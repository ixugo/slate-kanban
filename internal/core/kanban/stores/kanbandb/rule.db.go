package kanbandb

import (
	"context"

	"github.com/ixugo/goddd/pkg/orm"

	"slate-kanban/internal/core/kanban"
)

var _ kanban.RuleStorer = Rule{}

// Rule 是自动化规则的数据库实现，和 DB 同一个底层结构，通过类型转换得到。
type Rule DB

// WithTx 返回绑定到事务的副本。
func (d Rule) WithTx(tx orm.Tx) kanban.RuleStorer {
	return Rule{db: orm.GormDB(tx)}
}

// Create 新增一条规则。写入前先 Normalize，让库里存的是 [] 而不是 null。
func (d Rule) Create(ctx context.Context, model *kanban.Rule) error {
	model.Normalize()
	return d.db.WithContext(ctx).Create(model).Error
}

// Save 把整条规则写回数据库。model.ID 必须非零，否则 panic，防止 Save 变成新增或误改。
func (d Rule) Save(ctx context.Context, model *kanban.Rule) error {
	if model.ID == 0 {
		panic("kanbandb.Rule.Save: model.ID is zero, primary key is required")
	}
	model.Normalize()
	return d.db.WithContext(ctx).Save(model).Error
}

// Delete 按主键删除，幂等。model.ID 必须非零，否则 panic。
func (d Rule) Delete(ctx context.Context, model *kanban.Rule) error {
	if model.ID == 0 {
		panic("kanbandb.Rule.Delete: model.ID is zero, primary key is required")
	}
	return d.db.WithContext(ctx).Delete(model).Error
}

// GetByID 按主键读取。
func (d Rule) GetByID(ctx context.Context, id int64) (*kanban.Rule, error) {
	var out kanban.Rule
	if err := d.db.WithContext(ctx).Where("id = ?", id).Take(&out).Error; err != nil {
		return nil, err
	}
	out.Normalize()
	return &out, nil
}

// ListByBoard 返回看板的全部规则，按排序值升序；排序值相同时按主键升序，保证执行顺序稳定。
func (d Rule) ListByBoard(ctx context.Context, boardID int64) ([]*kanban.Rule, error) {
	items := make([]*kanban.Rule, 0)
	err := d.db.WithContext(ctx).Where("board_id = ?", boardID).Order("sort ASC, id ASC").Find(&items).Error
	if err != nil {
		return nil, err
	}
	for _, r := range items {
		r.Normalize()
	}
	return items, nil
}

// DeleteByBoard 删除看板的全部规则。
func (d Rule) DeleteByBoard(ctx context.Context, boardID int64) error {
	return d.db.WithContext(ctx).Where("board_id = ?", boardID).Delete(&kanban.Rule{}).Error
}
