package kanbandb

import (
	"context"

	"github.com/ixugo/goddd/pkg/orm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"slate-kanban/internal/core/kanban"
)

var _ kanban.BoardStorer = Board{}

// Board 是看板的数据库实现，和 DB 同一个底层结构，通过类型转换得到。
type Board DB

// WithTx 返回绑定到事务的副本。传入的 tx 不是本包产生的会直接 panic，那是编程错误。
func (d Board) WithTx(tx orm.Tx) kanban.BoardStorer {
	return Board{db: orm.GormDB(tx)}
}

// Create 新增一个看板。
func (d Board) Create(ctx context.Context, model *kanban.Board) error {
	return d.db.WithContext(ctx).Create(model).Error
}

// Save 把整个看板写回数据库。model.ID 必须非零，否则 panic，防止 Save 变成新增或误改。
func (d Board) Save(ctx context.Context, model *kanban.Board) error {
	if model.ID == 0 {
		panic("kanbandb.Board.Save: model.ID is zero, primary key is required")
	}
	return d.db.WithContext(ctx).Save(model).Error
}

// Delete 按主键删除，幂等。model.ID 必须非零，否则 panic。
func (d Board) Delete(ctx context.Context, model *kanban.Board) error {
	if model.ID == 0 {
		panic("kanbandb.Board.Delete: model.ID is zero, primary key is required")
	}
	return d.db.WithContext(ctx).Delete(model).Error
}

// GetByID 按主键读取。
func (d Board) GetByID(ctx context.Context, id int64) (*kanban.Board, error) {
	var out kanban.Board
	if err := d.db.WithContext(ctx).Where("id = ?", id).Take(&out).Error; err != nil {
		return nil, err
	}
	return &out, nil
}

// GetBySlug 按 slug 读取。
func (d Board) GetBySlug(ctx context.Context, slug string) (*kanban.Board, error) {
	var out kanban.Board
	if err := d.db.WithContext(ctx).Where("slug = ?", slug).Take(&out).Error; err != nil {
		return nil, err
	}
	return &out, nil
}

// List 返回全部看板，按主键升序，也就是创建顺序。
func (d Board) List(ctx context.Context) ([]*kanban.Board, error) {
	items := make([]*kanban.Board, 0)
	if err := d.db.WithContext(ctx).Order("id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// Lock 锁住看板行并返回最新数据。
func (d Board) Lock(ctx context.Context, id int64) (*kanban.Board, error) {
	var out kanban.Board
	err := d.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).Take(&out).Error
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// IncrCardSeq 用一条 UPDATE 加一，并用 RETURNING 取回新值，
// 不需要先读后写，所以即使调用方忘了先锁行，也不会拿到重复的编号。
func (d Board) IncrCardSeq(ctx context.Context, id int64) (int64, error) {
	var out kanban.Board
	res := d.db.WithContext(ctx).Model(&out).Where("id = ?", id).
		Clauses(clause.Returning{Columns: []clause.Column{{Name: "card_seq"}}}).
		UpdateColumn("card_seq", gorm.Expr("card_seq + 1"))
	if res.Error != nil {
		return 0, res.Error
	}
	if res.RowsAffected == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	return out.CardSeq, nil
}
