// Package kanbantest 提供看板领域测试用的构造函数：内存 SQLite、迁移好的表、完整的 Core。
// 它只被测试使用，生产代码不应该引用。
package kanbantest

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/ixugo/goddd/pkg/orm"
	"gorm.io/gorm"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
)

// NewRawDB 创建一个只活在当前测试里的内存 SQLite，不建任何表。
// 连接池和线上默认的 SQLite 一致：只有一个连接，所以事务里漏用事务连接的写法，
// 在测试里会直接卡死暴露出来。orm.New 同时打开了 TranslateError，
// 唯一键冲突会被翻译成 orm.ErrDuplicatedKey，和线上行为相同。
func NewRawDB(t testing.TB) *gorm.DB {
	t.Helper()
	db, err := orm.New(sqlite.Open(":memory:"), orm.Config{MaxIdleConns: 1, MaxOpenConns: 1})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// NewDB 创建内存 SQLite 并迁移好全部表，返回数据库存储。
func NewDB(t testing.TB) kanbandb.DB {
	t.Helper()
	return kanbandb.NewDB(NewRawDB(t)).AutoMigrate(true)
}

// NewCore 创建一个接好内存数据库的 Core，供业务测试使用。
func NewCore(t testing.TB, opts ...kanban.Option) kanban.Core {
	t.Helper()
	return kanban.NewCore(NewDB(t), opts...)
}
