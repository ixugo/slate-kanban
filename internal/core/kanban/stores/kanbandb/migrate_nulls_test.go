package kanbandb_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"gorm.io/gorm"

	"slate-kanban/internal/core/kanban/kanbantest"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
)

// nullableAllowed 是允许为 NULL 的列：用 NULL 表示「没有这个时间」的时间戳，和 deleted_at 一类，
// 其余所有列（字符串、数字、主键引用、JSON 数组）一律非空，用 0 值、空串或 [] 表达没有。
var nullableAllowed = []string{"cards.due_at", "cards.started_at", "cards.finished_at", "cards.claim_expires_at"}

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

// TestFillNullJSONColumns 旧库里 JSON 数组列是 NULL（或字面量 null）的行，同步表结构后要变成空数组，
// 并且列被改成非空；正常有值的行不受影响。
func TestFillNullJSONColumns(t *testing.T) {
	kanbantest.ForEachRawBackend(t, func(t *testing.T, raw *gorm.DB) {
		ctx := context.Background()
		exec := func(sql string) {
			t.Helper()
			if err := raw.Exec(sql).Error; err != nil {
				t.Fatalf("执行 %q 失败: %v", sql, err)
			}
		}
		// 先同步出完整的表，再用「建表即查询」把 cards 换成没有任何 NOT NULL 约束的旧版样子
		kanbandb.NewDB(raw).AutoMigrate(true)
		exec("ALTER TABLE cards RENAME TO cards_keep")
		exec("CREATE TABLE cards AS SELECT * FROM cards_keep")
		exec("DROP TABLE cards_keep")
		insert := func(id int64, waits, tags string) {
			t.Helper()
			exec(fmt.Sprintf("INSERT INTO cards (id, board_id, column_id, number, key, title, description, position, priority, tag_ids, human_only, waits_on, claimed_by, branch, url, version, created_by, updated_by, created_at, updated_at) "+
				"VALUES (%d, 1, 1, %d, 'A-%d', 't', '', 0, 0, %s, FALSE, %s, '', '', '', 1, '', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)", id, id, id, tags, waits))
		}
		insert(1, "NULL", "NULL")
		insert(2, "'null'", "'null'")
		insert(3, "'[7]'", "'[1,2]'")

		db := kanbandb.NewDB(raw).AutoMigrate(true)

		for id, want := range map[int64]struct{ waits, tags []int64 }{1: {}, 2: {}, 3: {[]int64{7}, []int64{1, 2}}} {
			c, err := db.Card().GetByID(ctx, id)
			if err != nil {
				t.Fatalf("读卡片 %d 失败: %v", id, err)
			}
			if !slices.Equal(c.WaitsOn, want.waits) || !slices.Equal(c.TagIDs, want.tags) {
				t.Fatalf("卡片 %d：waits_on=%v tag_ids=%v，期望 %v %v", id, c.WaitsOn, c.TagIDs, want.waits, want.tags)
			}
		}
		var nulls int64
		raw.Table("cards").Where("waits_on IS NULL OR tag_ids IS NULL").Count(&nulls)
		if nulls != 0 {
			t.Fatalf("同步后不应再有 NULL，还剩 %d 行", nulls)
		}
	})
}
