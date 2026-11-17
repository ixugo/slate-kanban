// Package kanbandb 是看板领域的数据库实现（被动适配器）。
package kanbandb

import (
	"github.com/ixugo/goddd/pkg/orm"
	"gorm.io/gorm"

	"slate-kanban/internal/core/kanban"
)

var _ kanban.Storer = DB{}

// DB 是看板领域各实体存储的统一入口。它只有一个指针字段，
// 各实体存储通过类型转换从它得到（例如 Board(d)），不会产生额外的内存分配。
type DB struct {
	db *gorm.DB
}

// NewDB 用 GORM 连接创建看板领域的数据库存储。
func NewDB(db *gorm.DB) DB {
	return DB{db: db}
}

// Begin 开启一个事务，返回 orm.Tx 句柄。
func (d DB) Begin() (orm.Tx, error) {
	return orm.Begin(d.db)
}

// Board 返回看板存储。
func (d DB) Board() kanban.BoardStorer {
	return Board(d)
}

// AutoMigrate 同步表结构。它在每次启动时都会执行，所以必须可以重复执行：
// 已有的表只会补齐缺失的列和索引，不会删除或改写数据。
func (d DB) AutoMigrate(ok bool) DB {
	if !ok {
		return d
	}
	if err := d.db.AutoMigrate(
		new(kanban.Board), new(kanban.Column), new(kanban.Card), new(kanban.Comment),
		new(kanban.Rule), new(kanban.Event), new(kanban.AgentCursor),
	); err != nil {
		panic(err)
	}
	return d
}
