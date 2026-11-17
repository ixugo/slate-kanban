// Package accesstest 提供访问令牌领域测试用的构造函数：内存 SQLite、迁移好的表、完整的 Core。
// 它只被测试使用，生产代码不应该引用。
package accesstest

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/ixugo/goddd/pkg/orm"

	"slate-kanban/internal/core/access"
	"slate-kanban/internal/core/access/stores/accessdb"
)

// NewDB 创建一个只活在当前测试里的内存 SQLite 并迁移好令牌表，返回数据库存储。
// 连接池和线上默认的 SQLite 一致：只有一个连接。orm.New 同时打开了 TranslateError，
// 唯一键冲突会被翻译成 orm.ErrDuplicatedKey，和线上行为相同。
func NewDB(t testing.TB) accessdb.DB {
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
	return accessdb.NewDB(db).AutoMigrate(true)
}

// NewCore 创建一个接好内存数据库的 Core，供业务测试使用。
func NewCore(t testing.TB, adminToken string, opts ...access.Option) access.Core {
	t.Helper()
	return access.NewCore(NewDB(t), adminToken, opts...)
}
