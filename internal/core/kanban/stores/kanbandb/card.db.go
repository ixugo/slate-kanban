package kanbandb

import (
	"context"

	"github.com/ixugo/goddd/pkg/orm"
	"gorm.io/gorm"

	"slate-kanban/internal/core/kanban"
)

var _ kanban.CardStorer = Card{}

// Card 是任务的数据库实现，和 DB 同一个底层结构，通过类型转换得到。
type Card DB

// WithTx 返回绑定到事务的副本。
func (d Card) WithTx(tx orm.Tx) kanban.CardStorer {
	return Card{db: orm.GormDB(tx)}
}

// Create 新增一个任务。写入前先 Normalize，让库里存的是 [] 而不是 null。
func (d Card) Create(ctx context.Context, model *kanban.Card) error {
	model.Normalize()
	return d.db.WithContext(ctx).Create(model).Error
}

// Save 把整个任务写回数据库。model.ID 必须非零，否则 panic，防止 Save 变成新增或误改。
func (d Card) Save(ctx context.Context, model *kanban.Card) error {
	if model.ID == 0 {
		panic("kanbandb.Card.Save: model.ID is zero, primary key is required")
	}
	model.Normalize()
	return d.db.WithContext(ctx).Save(model).Error
}

// Delete 按主键删除，幂等。model.ID 必须非零，否则 panic。
func (d Card) Delete(ctx context.Context, model *kanban.Card) error {
	if model.ID == 0 {
		panic("kanbandb.Card.Delete: model.ID is zero, primary key is required")
	}
	return d.db.WithContext(ctx).Delete(model).Error
}

// GetByID 按主键读取。
func (d Card) GetByID(ctx context.Context, id int64) (*kanban.Card, error) {
	var out kanban.Card
	if err := d.db.WithContext(ctx).Where("id = ?", id).Take(&out).Error; err != nil {
		return nil, err
	}
	out.Normalize()
	return &out, nil
}

// GetByKey 按任务编号读取。
func (d Card) GetByKey(ctx context.Context, key string) (*kanban.Card, error) {
	var out kanban.Card
	if err := d.db.WithContext(ctx).Where("key = ?", key).Take(&out).Error; err != nil {
		return nil, err
	}
	out.Normalize()
	return &out, nil
}

// ListByBoard 返回看板的全部任务。一次查询取回，调用方在内存里按列的顺序和筛选条件处理，
// 单个看板的任务数量预期在几百到几千个，这样比为每种筛选各写一条 SQL 简单可靠。
func (d Card) ListByBoard(ctx context.Context, boardID int64) ([]*kanban.Card, error) {
	items := make([]*kanban.Card, 0)
	err := d.db.WithContext(ctx).Where("board_id = ?", boardID).
		Order("column_id ASC, position ASC, id ASC").Find(&items).Error
	if err != nil {
		return nil, err
	}
	normalizeCards(items)
	return items, nil
}

// ListRecent 用限定的列和条数读取桌面状态栏需要的任务摘要。
func (d Card) ListRecent(ctx context.Context, limit int) ([]kanban.RecentCard, error) {
	var items []kanban.RecentCard
	err := d.db.WithContext(ctx).Model(&kanban.Card{}).
		Select("id", "board_id", "key", "title").
		Where("archived_at IS NULL").
		Order("COALESCE((SELECT MAX(created_at) FROM events WHERE events.card_id = cards.id), cards.updated_at) DESC, cards.id DESC").
		Limit(limit).Find(&items).Error
	return items, err
}

// ListArchivedByBoard 按 (archived_at, id) 倒序读取指定看板的归档任务。
func (d Card) ListArchivedByBoard(ctx context.Context, boardID int64, before *kanban.Card, limit int) ([]*kanban.Card, error) {
	q := d.db.WithContext(ctx).Where("board_id = ? AND archived_at IS NOT NULL", boardID)
	if before != nil {
		q = q.Where("(archived_at < ? OR (archived_at = ? AND id < ?))", before.ArchivedAt, before.ArchivedAt, before.ID)
	}
	items := make([]*kanban.Card, 0)
	if err := q.Order("archived_at DESC, id DESC").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	normalizeCards(items)
	return items, nil
}

// ListByColumn 返回某一列的任务，按位置升序。
func (d Card) ListByColumn(ctx context.Context, columnID int64) ([]*kanban.Card, error) {
	items := make([]*kanban.Card, 0)
	err := d.db.WithContext(ctx).Where("column_id = ? AND archived_at IS NULL", columnID).
		Order("position ASC, id ASC").Find(&items).Error
	if err != nil {
		return nil, err
	}
	normalizeCards(items)
	return items, nil
}

// CountByColumn 返回某一列有几个任务。
func (d Card) CountByColumn(ctx context.Context, columnID int64) (int64, error) {
	var n int64
	err := d.db.WithContext(ctx).Model(&kanban.Card{}).Where("column_id = ? AND archived_at IS NULL", columnID).Count(&n).Error
	return n, err
}

// MaxPosition 返回某一列里最大的位置。COALESCE 让空列得到 0，而不是 NULL。
func (d Card) MaxPosition(ctx context.Context, columnID int64) (float64, error) {
	var pos float64
	err := d.db.WithContext(ctx).Model(&kanban.Card{}).Where("column_id = ? AND archived_at IS NULL", columnID).
		Select("COALESCE(MAX(position), 0)").Scan(&pos).Error
	return pos, err
}

// SetPosition 只更新位置，不改版本号和修改时间：整列重排是内部整理，不是用户的修改。
func (d Card) SetPosition(ctx context.Context, id int64, position float64) error {
	return d.db.WithContext(ctx).Model(&kanban.Card{}).Where("id = ?", id).
		UpdateColumn("position", position).Error
}

// DeleteByBoard 删除看板的全部任务。
func (d Card) DeleteByBoard(ctx context.Context, boardID int64) error {
	return d.db.WithContext(ctx).Where("board_id = ?", boardID).Delete(&kanban.Card{}).Error
}

// IncrUnreadCount 在未读列上加一，留给网页角标用，不算用户编辑。
func (d Card) IncrUnreadCount(ctx context.Context, id int64) error {
	return d.db.WithContext(ctx).Model(&kanban.Card{}).Where("id = ?", id).
		UpdateColumn("unread_count", gorm.Expr("unread_count + ?", 1)).Error
}

// ClearUnreadCount 把未读数归零，打开详情时调用，不算用户编辑。
func (d Card) ClearUnreadCount(ctx context.Context, id int64) error {
	return d.db.WithContext(ctx).Model(&kanban.Card{}).Where("id = ?", id).
		UpdateColumn("unread_count", 0).Error
}

// normalizeCards 对一批任务调用 Normalize。
func normalizeCards(items []*kanban.Card) {
	for _, c := range items {
		c.Normalize()
	}
}
