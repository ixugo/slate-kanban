package kanbandb_test

import (
	"context"
	"testing"

	"github.com/ixugo/goddd/pkg/orm"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/kanbantest"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
)

// newBoard 在数据库里建一个测试看板并返回，slug 由调用方指定。
func newBoard(t *testing.T, store kanban.BoardStorer, slug string) *kanban.Board {
	t.Helper()
	b := &kanban.Board{Name: "看板 " + slug, Slug: slug}
	if err := store.Create(context.Background(), b); err != nil {
		t.Fatalf("创建看板失败: %v", err)
	}
	return b
}

// TestBoardCreateAndGet 验证看板能按主键和 slug 读回，不存在时错误能被识别成找不到。
func TestBoardCreateAndGet(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Board()
		b := newBoard(t, store, "SLATE")

		byID, err := store.GetByID(ctx, b.ID)
		if err != nil || byID.Slug != "SLATE" {
			t.Fatalf("按主键读取失败: %+v %v", byID, err)
		}
		bySlug, err := store.GetBySlug(ctx, "SLATE")
		if err != nil || bySlug.ID != b.ID {
			t.Fatalf("按 slug 读取失败: %+v %v", bySlug, err)
		}
		if _, err := store.GetByID(ctx, 9999); !orm.IsErrRecordNotFound(err) {
			t.Fatalf("不存在的主键应该是找不到记录，实际: %v", err)
		}
		if _, err := store.GetBySlug(ctx, "NOPE"); !orm.IsErrRecordNotFound(err) {
			t.Fatalf("不存在的 slug 应该是找不到记录，实际: %v", err)
		}
	})
}

// TestBoardSlugIsUnique 验证 slug 重复时返回的错误能被识别成唯一键冲突，
// 看板创建流程靠它把并发抢同一个 slug 的失败方翻译成 409。
func TestBoardSlugIsUnique(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		store := db.Board()
		newBoard(t, store, "SLATE")
		err := store.Create(context.Background(), &kanban.Board{Name: "另一个", Slug: "SLATE"})
		if !orm.IsDuplicatedKey(err) {
			t.Fatalf("重复 slug 应该是唯一键冲突，实际: %v", err)
		}
	})
}

// TestBoardListKeepsCreationOrder 验证列表按创建顺序返回。
func TestBoardListKeepsCreationOrder(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		store := db.Board()
		for _, slug := range []string{"CC", "AA", "BB"} {
			newBoard(t, store, slug)
		}
		items, err := store.List(context.Background())
		if err != nil {
			t.Fatalf("列表失败: %v", err)
		}
		var got []string
		for _, b := range items {
			got = append(got, b.Slug)
		}
		if len(got) != 3 || got[0] != "CC" || got[1] != "AA" || got[2] != "BB" {
			t.Fatalf("应该按创建顺序返回，实际: %v", got)
		}
	})
}

// TestBoardSave 验证 Save 把整个看板写回，包括改成空字符串的字段，读出来的是最新值。
func TestBoardSave(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Board()
		b := newBoard(t, store, "SLATE")

		b.Name, b.Description, b.ProjectPath = "新名字", "说明", "/code"
		if err := store.Save(ctx, b); err != nil {
			t.Fatalf("保存失败: %v", err)
		}
		got, _ := store.GetByID(ctx, b.ID)
		if got.Name != "新名字" || got.Description != "说明" || got.ProjectPath != "/code" {
			t.Fatalf("字段应该已更新，实际: %+v", got)
		}

		b.ProjectPath = ""
		if err := store.Save(ctx, b); err != nil {
			t.Fatalf("保存失败: %v", err)
		}
		if got, _ = store.GetByID(ctx, b.ID); got.ProjectPath != "" {
			t.Fatalf("清空的字段也应该写入，实际: %q", got.ProjectPath)
		}
	})
}

// TestBoardSaveRequiresPrimaryKey 验证没带主键的保存直接 panic，防止变成新增或误改。
func TestBoardSaveRequiresPrimaryKey(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		store := db.Board()
		defer func() {
			if recover() == nil {
				t.Fatal("没带主键的保存应该 panic")
			}
		}()
		_ = store.Save(context.Background(), &kanban.Board{})
	})
}

// TestBoardDeleteIsIdempotent 验证删除幂等：删两次都成功，之后读取是找不到。
func TestBoardDeleteIsIdempotent(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Board()
		b := newBoard(t, store, "SLATE")
		for range 2 {
			if err := store.Delete(ctx, &kanban.Board{ID: b.ID}); err != nil {
				t.Fatalf("删除应该成功: %v", err)
			}
		}
		if _, err := store.GetByID(ctx, b.ID); !orm.IsErrRecordNotFound(err) {
			t.Fatalf("删除后应该找不到，实际: %v", err)
		}
	})
}

// TestBoardIncrCardSeq 验证卡片编号计数从 1 开始连续递增，看板不存在时返回找不到。
func TestBoardIncrCardSeq(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Board()
		b := newBoard(t, store, "SLATE")
		for want := int64(1); want <= 3; want++ {
			got, err := store.IncrCardSeq(ctx, b.ID)
			if err != nil || got != want {
				t.Fatalf("第 %d 次应该返回 %d，实际 %d, %v", want, want, got, err)
			}
		}
		if _, err := store.IncrCardSeq(ctx, 9999); !orm.IsErrRecordNotFound(err) {
			t.Fatalf("不存在的看板应该是找不到记录，实际: %v", err)
		}
	})
}

// TestBoardTxLockAndRollback 验证事务内通过 WithTx 副本锁行、改数据，回滚后改动消失，
// 并且回滚释放了唯一的连接，后面的普通读取不会卡死。
func TestBoardTxLockAndRollback(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		b := newBoard(t, db.Board(), "SLATE")

		tx, err := db.Begin()
		if err != nil {
			t.Fatalf("开启事务失败: %v", err)
		}
		inTx := db.Board().WithTx(tx)
		locked, err := inTx.Lock(ctx, b.ID)
		if err != nil || locked.ID != b.ID {
			t.Fatalf("事务内锁行失败: %+v %v", locked, err)
		}
		if _, err := inTx.IncrCardSeq(ctx, b.ID); err != nil {
			t.Fatalf("事务内自增失败: %v", err)
		}
		if err := tx.Rollback(); err != nil {
			t.Fatalf("回滚失败: %v", err)
		}

		got, err := db.Board().GetByID(ctx, b.ID)
		if err != nil {
			t.Fatalf("回滚后读取失败: %v", err)
		}
		if got.CardSeq != 0 {
			t.Fatalf("回滚后计数应该还是 0，实际 %d", got.CardSeq)
		}
	})
}
