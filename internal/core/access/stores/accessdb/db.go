// Package accessdb 是访问令牌领域的数据库实现（被动适配器）。
package accessdb

import (
	"gorm.io/gorm"

	"slate-kanban/internal/core/access"
)

var _ access.Storer = DB{}

// DB 是访问令牌领域各实体存储的统一入口，只有一个指针字段，
// 各实体存储通过类型转换从它得到，不会产生额外的内存分配。
type DB struct {
	db *gorm.DB
}

// NewDB 用 GORM 连接创建访问令牌领域的数据库存储。
func NewDB(db *gorm.DB) DB {
	return DB{db: db}
}

// Token 返回令牌存储。
func (d DB) Token() access.TokenStorer {
	return Token(d)
}

// AutoMigrate 同步表结构。它在每次启动时都会执行，所以必须可以重复执行：
// 已有的表只会补齐缺失的列和索引，不会删除或改写数据。
func (d DB) AutoMigrate(ok bool) DB {
	if !ok {
		return d
	}
	if err := d.db.AutoMigrate(new(access.Token)); err != nil {
		panic(err)
	}
	return d
}
