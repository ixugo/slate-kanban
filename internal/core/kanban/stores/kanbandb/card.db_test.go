package kanbandb_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/ixugo/goddd/pkg/orm"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/kanbantest"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
)

// addCard 写入一张测试卡片并返回。
func addCard(t *testing.T, store kanban.CardStorer, boardID, columnID int64, key string, pos float64) *kanban.Card {
	t.Helper()
	c := &kanban.Card{BoardID: boardID, ColumnID: columnID, Key: key, Title: "卡片 " + key, Position: pos, Version: 1}
	if err := store.Create(context.Background(), c); err != nil {
		t.Fatalf("创建卡片失败: %v", err)
	}
	return c
}

// cardKeys 返回卡片的编号列表。
func cardKeys(items []*kanban.Card) []string {
	keys := make([]string, len(items))
	for i, c := range items {
		keys[i] = c.Key
	}
	return keys
}

// TestCardRoundTrip 验证卡片所有类型的字段都能存取：JSON 数组、时间指针、布尔值；
// 没填的标签和依赖读出来是空数组而不是 nil。
func TestCardRoundTrip(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Card()
		due := time.Date(2026, 5, 6, 7, 8, 9, 0, time.UTC)
		c := &kanban.Card{
			BoardID: 1, ColumnID: 2, Number: 7, Key: "T-7", Title: "标题", Description: "描述", Position: 1536.5,
			Priority: 3, Tags: []string{"后端", "bug"}, HumanOnly: true, WaitsOn: []int64{4, 5}, DueAt: &due,
			Branch: "feat/x", URL: "https://example.com/pr/1", Version: 1, CreatedBy: "admin",
		}
		if err := store.Create(ctx, c); err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		for name, get := range map[string]func() (*kanban.Card, error){
			"按主键": func() (*kanban.Card, error) { return store.GetByID(ctx, c.ID) },
			"按编号": func() (*kanban.Card, error) { return store.GetByKey(ctx, "T-7") },
		} {
			got, err := get()
			if err != nil {
				t.Fatalf("%s读取失败: %v", name, err)
			}
			if got.Title != "标题" || got.Position != 1536.5 || got.Priority != 3 || !got.HumanOnly ||
				!slices.Equal(got.Tags, c.Tags) || !slices.Equal(got.WaitsOn, c.WaitsOn) ||
				got.DueAt == nil || !got.DueAt.Equal(due) || got.StartedAt != nil || got.ClaimExpiresAt != nil {
				t.Fatalf("%s读出的字段不对: %+v", name, got)
			}
		}

		bare := addCard(t, store, 1, 2, "T-8", 1)
		got, _ := store.GetByID(ctx, bare.ID)
		if got.Tags == nil || got.WaitsOn == nil || len(got.Tags) != 0 || len(got.WaitsOn) != 0 {
			t.Fatalf("没填的标签和依赖应该读出空数组: %+v", got)
		}
		if _, err := store.GetByID(ctx, 9999); !orm.IsErrRecordNotFound(err) {
			t.Fatalf("不存在的主键应该是找不到记录，实际: %v", err)
		}
		if _, err := store.GetByKey(ctx, "NOPE-1"); !orm.IsErrRecordNotFound(err) {
			t.Fatalf("不存在的编号应该是找不到记录，实际: %v", err)
		}
	})
}

// TestCardKeyIsUnique 验证编号重复时返回的错误能被识别成唯一键冲突。
func TestCardKeyIsUnique(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		store := db.Card()
		addCard(t, store, 1, 1, "T-1", 1)
		err := store.Create(context.Background(), &kanban.Card{BoardID: 1, ColumnID: 1, Key: "T-1", Title: "重复"})
		if !orm.IsDuplicatedKey(err) {
			t.Fatalf("重复编号应该是唯一键冲突，实际: %v", err)
		}
	})
}

// TestCardSaveWritesZeroValues 验证 Save 会把零值、false、nil 都写进去，
// 否则清除领取、清空完成时间这类修改会悄悄丢失；没带主键直接 panic。
func TestCardSaveWritesZeroValues(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Card()
		now := time.Now().UTC()
		c := addCard(t, store, 1, 1, "T-1", 1)
		c.HumanOnly, c.ClaimedBy, c.FinishedAt, c.ClaimExpiresAt, c.Priority = true, "claude", &now, &now, 2
		c.Tags = []string{"a"}
		if err := store.Save(ctx, c); err != nil {
			t.Fatalf("保存失败: %v", err)
		}

		c.HumanOnly, c.ClaimedBy, c.FinishedAt, c.ClaimExpiresAt, c.Priority, c.Tags = false, "", nil, nil, 0, nil
		if err := store.Save(ctx, c); err != nil {
			t.Fatalf("保存失败: %v", err)
		}
		got, _ := store.GetByID(ctx, c.ID)
		if got.HumanOnly || got.ClaimedBy != "" || got.FinishedAt != nil || got.ClaimExpiresAt != nil ||
			got.Priority != 0 || got.Tags == nil || len(got.Tags) != 0 {
			t.Fatalf("零值应该都写进去了: %+v", got)
		}

		defer func() {
			if recover() == nil {
				t.Fatal("没带主键的保存应该 panic")
			}
		}()
		_ = store.Save(ctx, &kanban.Card{})
	})
}

// TestCardListsAndPositions 验证列表的范围和顺序、列内计数、最大位置，以及空列的最大位置是 0。
func TestCardListsAndPositions(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Card()
		addCard(t, store, 1, 20, "T-3", 3072)
		addCard(t, store, 1, 10, "T-2", 2048)
		addCard(t, store, 1, 10, "T-1", 1024)
		addCard(t, store, 1, 10, "T-4", 2048)
		addCard(t, store, 2, 30, "U-1", 1024)

		byBoard, err := store.ListByBoard(ctx, 1)
		if err != nil {
			t.Fatalf("按看板列出失败: %v", err)
		}
		if got, want := cardKeys(byBoard), []string{"T-1", "T-2", "T-4", "T-3"}; !slices.Equal(got, want) {
			t.Fatalf("按看板应该按列、位置、主键排序，期望 %v，实际 %v", want, got)
		}
		byColumn, _ := store.ListByColumn(ctx, 10)
		if got, want := cardKeys(byColumn), []string{"T-1", "T-2", "T-4"}; !slices.Equal(got, want) {
			t.Fatalf("按列应该按位置升序、位置相同按主键，期望 %v，实际 %v", want, got)
		}
		if n, _ := store.CountByColumn(ctx, 10); n != 3 {
			t.Fatalf("第 10 列应该有 3 张，实际 %d", n)
		}
		if pos, _ := store.MaxPosition(ctx, 10); pos != 2048 {
			t.Fatalf("第 10 列的最大位置应该是 2048，实际 %v", pos)
		}
		if pos, err := store.MaxPosition(ctx, 999); err != nil || pos != 0 {
			t.Fatalf("空列的最大位置应该是 0，实际 %v, %v", pos, err)
		}
		if empty, _ := store.ListByColumn(ctx, 999); empty == nil || len(empty) != 0 {
			t.Fatalf("空列应该返回空数组: %v", empty)
		}
	})
}

// TestCardSetPositionKeepsVersion 验证只改位置，不动版本号：整列重排是内部整理，不能让用户的编辑冲突。
func TestCardSetPositionKeepsVersion(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Card()
		c := addCard(t, store, 1, 1, "T-1", 1)
		if err := store.SetPosition(ctx, c.ID, 4096.25); err != nil {
			t.Fatalf("更新位置失败: %v", err)
		}
		got, _ := store.GetByID(ctx, c.ID)
		if got.Position != 4096.25 || got.Version != 1 {
			t.Fatalf("位置应该更新而版本不变: %+v", got)
		}
	})
}

// TestCardDelete 验证删除幂等，以及按看板删除只影响本看板。
func TestCardDelete(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Card()
		a := addCard(t, store, 1, 1, "T-1", 1)
		addCard(t, store, 1, 1, "T-2", 2)
		addCard(t, store, 2, 5, "U-1", 1)

		for range 2 {
			if err := store.Delete(ctx, &kanban.Card{ID: a.ID}); err != nil {
				t.Fatalf("删除应该成功: %v", err)
			}
		}
		if _, err := store.GetByID(ctx, a.ID); !orm.IsErrRecordNotFound(err) {
			t.Fatalf("删除后应该找不到，实际: %v", err)
		}
		if err := store.DeleteByBoard(ctx, 1); err != nil {
			t.Fatalf("按看板删除失败: %v", err)
		}
		if left, _ := store.ListByBoard(ctx, 1); len(left) != 0 {
			t.Fatalf("看板 1 应该清空，剩余 %d", len(left))
		}
		if left, _ := store.ListByBoard(ctx, 2); len(left) != 1 {
			t.Fatalf("看板 2 不应该被影响，剩余 %d", len(left))
		}
	})
}
