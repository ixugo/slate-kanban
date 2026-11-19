package kanbandb_test

import (
	"bytes"
	"testing"

	"gorm.io/gorm"

	"slate-kanban/internal/core/kanban/kanbantest"
	"slate-kanban/internal/core/kanban/stores/blobfs"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
)

// TestMigrateAttachmentFiles 旧库把图片存在 data 列里：搬迁后文件内容一致、记录有路径、data 列消失，
// 孤儿图片被删除，重复执行没有副作用。
func TestMigrateAttachmentFiles(t *testing.T) {
	kanbantest.ForEachRawBackend(t, func(t *testing.T, raw *gorm.DB) {
		db := kanbandb.NewDB(raw).AutoMigrate(true)
		exec := func(sql string, args ...any) {
			t.Helper()
			if err := raw.Exec(sql, args...).Error; err != nil {
				t.Fatalf("%s: %v", sql, err)
			}
		}
		// 还原旧版结构：加回 NOT NULL 的 data 列
		exec("ALTER TABLE attachments ADD COLUMN data bytea NOT NULL DEFAULT ''")
		exec("INSERT INTO boards (id, name, slug, created_at, updated_at) VALUES (1, 'b', 'SLATE', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)")
		png := []byte("\x89PNG\r\n\x1a\nfake")
		exec("INSERT INTO attachments (id, board_id, token, content_type, size, data, created_at) VALUES (1, 1, 'a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1', 'image/png', ?, ?, CURRENT_TIMESTAMP)", len(png), png)
		exec("INSERT INTO attachments (id, board_id, token, content_type, size, data, created_at) VALUES (2, 99, 'b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2', 'image/png', 1, ?, CURRENT_TIMESTAMP)", []byte{1})

		res := blobfs.New(t.TempDir())
		for i := 0; i < 2; i++ {
			if err := db.MigrateAttachmentFiles(res); err != nil {
				t.Fatalf("第 %d 次搬迁: %v", i+1, err)
			}
		}
		if raw.Migrator().HasColumn("attachments", "data") {
			t.Fatal("data 列应该被删除")
		}
		got, err := db.Attachment().GetByToken(t.Context(), "a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1")
		if err != nil || got.Path != "SLATE/a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1.png" {
			t.Fatalf("搬迁后的记录 %+v, %v", got, err)
		}
		if data, err := res.Read(got.Path); err != nil || !bytes.Equal(data, png) {
			t.Fatalf("文件内容 %v, %v", data, err)
		}
		if _, err := db.Attachment().GetByToken(t.Context(), "b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2"); err == nil {
			t.Fatal("孤儿图片的记录应该被删除")
		}
	})
}
