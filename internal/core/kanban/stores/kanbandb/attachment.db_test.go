package kanbandb_test

import (
	"context"
	"testing"

	"github.com/ixugo/goddd/pkg/orm"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/kanbantest"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
)

// TestAttachmentStore 验证图片记录（含文件路径）原样存取，按看板删除只影响该看板。
func TestAttachmentStore(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Attachment()
		a := &kanban.Attachment{BoardID: 1, Token: "a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1", ContentType: "image/png", Size: 9, Path: "A/a.png"}
		b := &kanban.Attachment{BoardID: 2, Token: "b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2", ContentType: "image/png", Size: 1, Path: "B/b.png"}
		for _, m := range []*kanban.Attachment{a, b} {
			if err := store.Create(ctx, m); err != nil {
				t.Fatalf("创建失败: %v", err)
			}
		}
		got, err := store.GetByToken(ctx, a.Token)
		if err != nil || got.Path != "A/a.png" || got.BoardID != 1 {
			t.Fatalf("读回不对: %+v %v", got, err)
		}
		if err := store.Create(ctx, &kanban.Attachment{BoardID: 1, Token: a.Token, Path: "B/b.png"}); err == nil {
			t.Fatal("编号重复应该报错")
		}
		if err := store.DeleteByBoard(ctx, 1); err != nil {
			t.Fatalf("删除失败: %v", err)
		}
		if _, err := store.GetByToken(ctx, a.Token); !orm.IsErrRecordNotFound(err) {
			t.Fatalf("应该已删除: %v", err)
		}
		if _, err := store.GetByToken(ctx, b.Token); err != nil {
			t.Fatalf("别的看板的图片不应该被删: %v", err)
		}
	})
}
