package kanbandb_test

import (
	"testing"

	"slate-kanban/internal/core/kanban/kanbantest"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
)

// TestAutoMigrateIsRepeatable 验证表结构迁移可以重复执行。
// 本项目每次启动都会迁移一次，所以第二次、第三次执行必须是无害的空操作。
func TestAutoMigrateIsRepeatable(t *testing.T) {
	db := kanbandb.NewDB(kanbantest.NewRawDB(t))
	for range 3 {
		db.AutoMigrate(true)
	}
}

// TestAutoMigrateDisabledDoesNothing 验证关闭迁移时不会建表。
func TestAutoMigrateDisabledDoesNothing(t *testing.T) {
	raw := kanbantest.NewRawDB(t)
	kanbandb.NewDB(raw).AutoMigrate(false)
	if raw.Migrator().HasTable("boards") {
		t.Fatal("关闭迁移时不应该创建 boards 表")
	}
}

// TestBeginRollback 验证事务可以开启并回滚，回滚后连接被释放，后续还能继续开启新事务。
// 只有一个连接的 SQLite 上，如果回滚没有释放连接，第二次开启事务会永远阻塞。
func TestBeginRollback(t *testing.T) {
	db := kanbandb.NewDB(kanbantest.NewRawDB(t))
	for range 2 {
		tx, err := db.Begin()
		if err != nil {
			t.Fatalf("开启事务失败: %v", err)
		}
		if err := tx.Rollback(); err != nil {
			t.Fatalf("回滚失败: %v", err)
		}
	}
}
