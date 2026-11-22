package kanbandb_test

import (
	"testing"

	"gorm.io/gorm"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/kanbantest"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
)

// TestAutoMigrateIsRepeatable 验证表结构迁移可以重复执行。
// 本项目每次启动都会迁移一次，所以第二次、第三次执行必须是无害的空操作。
func TestAutoMigrateIsRepeatable(t *testing.T) {
	kanbantest.ForEachRawBackend(t, func(t *testing.T, raw *gorm.DB) {
		db := kanbandb.NewDB(raw)
		for range 3 {
			db.AutoMigrate(true)
		}
	})
}

// TestAutoMigrateDisabledDoesNothing 验证关闭迁移时不会建表。
func TestAutoMigrateDisabledDoesNothing(t *testing.T) {
	kanbantest.ForEachRawBackend(t, func(t *testing.T, raw *gorm.DB) {
		kanbandb.NewDB(raw).AutoMigrate(false)
		if raw.Migrator().HasTable("boards") {
			t.Fatal("关闭迁移时不应该创建 boards 表")
		}
	})
}

// TestBeginRollback 验证事务可以开启并回滚，回滚后连接被释放，后续还能继续开启新事务。
// 只有一个连接的 SQLite 上，如果回滚没有释放连接，第二次开启事务会永远阻塞。
func TestBeginRollback(t *testing.T) {
	kanbantest.ForEachRawBackend(t, func(t *testing.T, raw *gorm.DB) {
		db := kanbandb.NewDB(raw).AutoMigrate(true)
		for range 2 {
			tx, err := db.Begin()
			if err != nil {
				t.Fatalf("开启事务失败: %v", err)
			}
			if err := tx.Rollback(); err != nil {
				t.Fatalf("回滚失败: %v", err)
			}
		}
	})
}

// TestAutoMigrateWebhookModesPreservesOldEmptyFilters checks both legacy interpretations of [].
func TestAutoMigrateWebhookModesPreservesOldEmptyFilters(t *testing.T) {
	kanbantest.ForEachRawBackend(t, func(t *testing.T, raw *gorm.DB) {
		store := kanbandb.NewDB(raw).AutoMigrate(true)
		ctx := t.Context()
		var hooks [2]kanban.Webhook
		for i := range hooks {
			hook := &hooks[i]
			hook.Enabled = true
			hook.StrictFilters = i != 0
			hook.Normalize()
			if err := store.Webhook().Create(ctx, hook); err != nil {
				t.Fatal(err)
			}
		}
		if err := raw.Exec("ALTER TABLE webhooks DROP COLUMN strict_filter_modes").Error; err != nil {
			t.Fatal(err)
		}
		store.AutoMigrate(true)
		event := &kanban.Event{Type: kanban.EventCardClaimed, ActorKind: "agent"}
		for i, hook := range hooks {
			got, err := store.Webhook().GetByID(ctx, hook.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.StrictFilterModes != (kanban.WebhookFilterModes{}) || kanban.WebhookMatchesEventForTest(got, event) != (i == 0) {
				t.Fatalf("迁移后旧筛选模式变化: %+v", got)
			}
		}
	})
}
