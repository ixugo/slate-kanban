package kanbandb_test

import (
	"context"
	"testing"

	"github.com/ixugo/goddd/pkg/orm"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/kanbantest"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
)

// addComment 写入一条测试评论并返回。
func addComment(t *testing.T, store kanban.CommentStorer, boardID, cardID int64, body string) *kanban.Comment {
	t.Helper()
	c := &kanban.Comment{BoardID: boardID, CardID: cardID, AuthorKind: "human", Author: "admin", Body: body}
	if err := store.Create(context.Background(), c); err != nil {
		t.Fatalf("创建评论失败: %v", err)
	}
	return c
}

// TestCommentCreateListGet 验证评论按写入先后列出，只列出本卡片的，并且能按主键读回。
func TestCommentCreateListGet(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Comment()
		first := addComment(t, store, 1, 10, "第一条")
		addComment(t, store, 1, 10, "第二条")
		addComment(t, store, 1, 11, "别的卡片")

		items, err := store.ListByCard(ctx, 10)
		if err != nil || len(items) != 2 || items[0].Body != "第一条" || items[1].Body != "第二条" {
			t.Fatalf("应该按先后列出本卡片的 2 条，实际 %+v, %v", items, err)
		}
		got, err := store.GetByID(ctx, first.ID)
		if err != nil || got.Body != "第一条" || got.Author != "admin" {
			t.Fatalf("按主键读取失败: %+v %v", got, err)
		}
		if _, err := store.GetByID(ctx, 9999); !orm.IsErrRecordNotFound(err) {
			t.Fatalf("不存在的主键应该是找不到记录，实际: %v", err)
		}
		if empty, _ := store.ListByCard(ctx, 999); empty == nil || len(empty) != 0 {
			t.Fatalf("没有评论应该返回空数组: %v", empty)
		}
	})
}

// TestCommentListByBoard 验证按看板一次列出全部评论，顺序按卡片再按主键。
func TestCommentListByBoard(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Comment()
		addComment(t, store, 1, 10, "卡10-1")
		addComment(t, store, 1, 10, "卡10-2")
		addComment(t, store, 1, 11, "卡11")
		addComment(t, store, 2, 20, "别的看板")

		items, err := store.ListByBoard(ctx, 1)
		if err != nil || len(items) != 3 || items[0].Body != "卡10-1" || items[2].Body != "卡11" {
			t.Fatalf("ListByBoard 顺序或范围不对: %+v %v", items, err)
		}
		if empty, err := store.ListByBoard(ctx, 999); err != nil || empty == nil || len(empty) != 0 {
			t.Fatalf("没有评论应返回空数组: %+v %v", empty, err)
		}
	})
}

// TestCommentDelete 验证单条删除幂等，按卡片和按看板删除都只影响自己的范围。
func TestCommentDelete(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Comment()
		a := addComment(t, store, 1, 10, "a")
		addComment(t, store, 1, 10, "b")
		addComment(t, store, 1, 11, "c")
		addComment(t, store, 2, 20, "d")

		for range 2 {
			if err := store.Delete(ctx, &kanban.Comment{ID: a.ID}); err != nil {
				t.Fatalf("删除应该成功: %v", err)
			}
		}
		if left, _ := store.ListByCard(ctx, 10); len(left) != 1 {
			t.Fatalf("卡片 10 应该剩 1 条，实际 %d", len(left))
		}
		if err := store.DeleteByCard(ctx, 10); err != nil {
			t.Fatalf("按卡片删除失败: %v", err)
		}
		if left, _ := store.ListByCard(ctx, 10); len(left) != 0 {
			t.Fatalf("卡片 10 应该清空，实际 %d", len(left))
		}
		if left, _ := store.ListByCard(ctx, 11); len(left) != 1 {
			t.Fatalf("卡片 11 不应该被影响，实际 %d", len(left))
		}
		if err := store.DeleteByBoard(ctx, 1); err != nil {
			t.Fatalf("按看板删除失败: %v", err)
		}
		if left, _ := store.ListByCard(ctx, 11); len(left) != 0 {
			t.Fatalf("看板 1 应该清空，实际 %d", len(left))
		}
		if left, _ := store.ListByCard(ctx, 20); len(left) != 1 {
			t.Fatalf("看板 2 不应该被影响，实际 %d", len(left))
		}
	})
}
