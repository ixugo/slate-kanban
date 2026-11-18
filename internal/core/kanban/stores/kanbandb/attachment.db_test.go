package kanbandb_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/ixugo/goddd/pkg/orm"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/kanbantest"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
)

// TestAttachmentStore 验证图片字节原样存取（含 0 字节和高位字节），按看板删除只影响该看板。
func TestAttachmentStore(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Attachment()
		data := []byte{0x89, 'P', 'N', 'G', 0, 1, 2, 0xff, 0xfe}
		a := &kanban.Attachment{BoardID: 1, Token: "a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1", ContentType: "image/png", Size: int64(len(data)), Data: data}
		b := &kanban.Attachment{BoardID: 2, Token: "b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2", ContentType: "image/png", Size: 1, Data: []byte{1}}
		for _, m := range []*kanban.Attachment{a, b} {
			if err := store.Create(ctx, m); err != nil {
				t.Fatalf("创建失败: %v", err)
			}
		}
		got, err := store.GetByToken(ctx, a.Token)
		if err != nil || !bytes.Equal(got.Data, data) || got.BoardID != 1 {
			t.Fatalf("读回不对: %+v %v", got, err)
		}
		if err := store.Create(ctx, &kanban.Attachment{BoardID: 1, Token: a.Token, Data: []byte{1}}); err == nil {
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
