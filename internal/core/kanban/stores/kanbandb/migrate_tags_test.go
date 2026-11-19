package kanbandb_test

import (
	"context"
	"slices"
	"testing"

	"gorm.io/gorm"

	"slate-kanban/internal/core/kanban/kanbantest"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
)

// TestMigrateLegacyTags 验证旧版数据的搬迁：boards.labels 里的标签按原顺序变成 tags 行，
// 卡片上 cards.tags 的文字（不区分大小写）换成主键，库外的文字补成没有颜色的标签；
// 搬完旧列被清空，重复迁移不会重复搬，没有旧数据的看板不受影响。
func TestMigrateLegacyTags(t *testing.T) {
	kanbantest.ForEachRawBackend(t, func(t *testing.T, raw *gorm.DB) {
		ctx := context.Background()
		db := kanbandb.NewDB(raw).AutoMigrate(true)
		exec := func(sql string, args ...any) {
			t.Helper()
			if err := raw.Exec(sql, args...).Error; err != nil {
				t.Fatalf("执行 %q 失败: %v", sql, err)
			}
		}
		exec("ALTER TABLE boards ADD COLUMN labels text")
		exec("ALTER TABLE cards ADD COLUMN tags text")
		exec(`INSERT INTO boards (id, name, slug, labels) VALUES (1, 'old', 'OLD', '[{"id":"0123abcd","name":"bug","color":"red-2"},{"id":"89abcdef","name":"后端","color":""}]')`)
		exec("INSERT INTO boards (id, name, slug, labels) VALUES (2, 'plain', 'PLN', '[]')")
		exec(`INSERT INTO cards (id, board_id, column_id, number, key, title, tags) VALUES (10, 1, 1, 1, 'OLD-1', 'a', '["Bug","库外","BUG","后端"]')`)
		exec(`INSERT INTO cards (id, board_id, column_id, number, key, title, tags) VALUES (11, 1, 1, 2, 'OLD-2', 'b', '[]')`)
		exec(`INSERT INTO cards (id, board_id, column_id, number, key, title, tags) VALUES (12, 2, 1, 1, 'PLN-1', 'c', '["自由文字"]')`)

		for range 2 {
			db.AutoMigrate(true)
		}

		old, _ := db.Tag().ListByBoard(ctx, 1)
		if len(old) != 3 || old[0].Text != "bug" || old[0].Color != "red-2" || old[1].Text != "后端" || old[2].Text != "库外" || old[2].Color != "" {
			t.Fatalf("看板 1 的标签库不对: %+v", old)
		}
		a, err := db.Card().GetByID(ctx, 10)
		if err != nil {
			t.Fatalf("读卡片失败: %v", err)
		}
		if want := []int64{old[0].ID, old[2].ID, old[1].ID}; !slices.Equal(a.TagIDs, want) {
			t.Fatalf("卡片 10 的标签应该是 %v，实际 %v", want, a.TagIDs)
		}
		if b, _ := db.Card().GetByID(ctx, 11); len(b.TagIDs) != 0 {
			t.Fatalf("没有标签的卡片不应该有 tag_ids: %v", b.TagIDs)
		}
		plain, _ := db.Tag().ListByBoard(ctx, 2)
		if len(plain) != 1 || plain[0].Text != "自由文字" {
			t.Fatalf("没有标签库、只有卡片文字的看板应该补出标签: %+v", plain)
		}
		if raw.Migrator().HasColumn("boards", "labels") || raw.Migrator().HasColumn("cards", "tags") {
			t.Fatal("搬完以后旧的 boards.labels 和 cards.tags 两列应该被删掉")
		}
	})
}
