package kanbandb_test

import (
	"slices"
	"strings"
	"testing"

	"gorm.io/gorm"

	"slate-kanban/internal/core/kanban/kanbantest"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
)

// nullableAllowed 是允许为 NULL 的列：用 NULL 表示「没有这个时间」的时间戳，和 deleted_at 一类，
// 其余所有列（字符串、数字、主键引用、JSON 数组）一律非空，用 0 值、空串或 [] 表达没有。
var nullableAllowed = []string{"cards.due_at", "cards.started_at", "cards.finished_at", "cards.claim_expires_at", "cards.archived_at"}

// TestNoNullableColumns 防回归：表结构同步以后，除了白名单里的时间戳列，不允许有可空列。
func TestNoNullableColumns(t *testing.T) {
	kanbantest.ForEachRawBackend(t, func(t *testing.T, raw *gorm.DB) {
		kanbandb.NewDB(raw).AutoMigrate(true)
		tables, err := raw.Migrator().GetTables()
		if err != nil {
			t.Fatal(err)
		}
		for _, table := range tables {
			if strings.HasPrefix(table, "sqlite_") {
				continue
			}
			types, err := raw.Migrator().ColumnTypes(table)
			if err != nil {
				t.Fatal(err)
			}
			for _, col := range types {
				if pk, ok := col.PrimaryKey(); ok && pk {
					continue
				}
				nullable, ok := col.Nullable()
				if ok && nullable && !slices.Contains(nullableAllowed, table+"."+col.Name()) {
					t.Errorf("%s.%s 允许 NULL：除了时间戳白名单，列都应该 notNull 并用 0 值表达没有", table, col.Name())
				}
			}
		}
	})
}
