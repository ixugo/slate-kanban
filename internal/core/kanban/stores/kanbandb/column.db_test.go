package kanbandb_test

import (
	"context"
	"testing"

	"github.com/ixugo/goddd/pkg/orm"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/kanbantest"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
)

// columnNames 把列的名称按顺序取出来，方便断言。
func columnNames(items []*kanban.Column) []string {
	names := make([]string, 0, len(items))
	for _, c := range items {
		names = append(names, c.Name)
	}
	return names
}

// addColumn 在数据库里建一列并返回。
func addColumn(t *testing.T, store kanban.ColumnStorer, boardID int64, name string, sort int64) *kanban.Column {
	t.Helper()
	c := &kanban.Column{BoardID: boardID, Name: name, Sort: sort}
	if err := store.Create(context.Background(), c); err != nil {
		t.Fatalf("创建列失败: %v", err)
	}
	return c
}

// TestColumnListByBoardOrder 验证列表只包含本看板的列，并且按排序值升序返回，
// 排序值相同时按主键升序，保证顺序稳定。
func TestColumnListByBoardOrder(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Column()
		addColumn(t, store, 1, "完成", 3072)
		addColumn(t, store, 1, "待办", 1024)
		addColumn(t, store, 1, "进行中A", 2048)
		addColumn(t, store, 1, "进行中B", 2048)
		addColumn(t, store, 2, "别的看板", 1)

		items, err := store.ListByBoard(ctx, 1)
		if err != nil {
			t.Fatalf("列表失败: %v", err)
		}
		got := columnNames(items)
		want := []string{"待办", "进行中A", "进行中B", "完成"}
		if len(got) != len(want) {
			t.Fatalf("应该只有本看板的 4 列，实际: %v", got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("顺序不对，期望 %v，实际 %v", want, got)
			}
		}
	})
}

// TestColumnSaveWritesZeroValues 验证 Save 会把 false 和 0 也写回去，
// 否则把领活开关关掉、把下一列清掉的修改会悄悄丢失。
func TestColumnSaveWritesZeroValues(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Column()
		c := &kanban.Column{BoardID: 1, Name: "进行中", AgentPickup: true, IsDone: true, NextColumnID: 9, Sort: 1}
		if err := store.Create(ctx, c); err != nil {
			t.Fatalf("创建列失败: %v", err)
		}

		c.AgentPickup, c.IsDone, c.NextColumnID = false, false, 0
		if err := store.Save(ctx, c); err != nil {
			t.Fatalf("保存失败: %v", err)
		}
		got, err := store.GetByID(ctx, c.ID)
		if err != nil {
			t.Fatalf("读取失败: %v", err)
		}
		if got.AgentPickup || got.IsDone || got.NextColumnID != 0 {
			t.Fatalf("零值没有写回去: %+v", got)
		}
	})
}

// TestColumnSetSort 验证单独更新排序值后，列表顺序随之变化。
func TestColumnSetSort(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Column()
		a := addColumn(t, store, 1, "A", 1024)
		addColumn(t, store, 1, "B", 2048)

		if err := store.SetSort(ctx, a.ID, 4096); err != nil {
			t.Fatalf("更新排序值失败: %v", err)
		}
		items, _ := store.ListByBoard(ctx, 1)
		if got := columnNames(items); got[0] != "B" || got[1] != "A" {
			t.Fatalf("A 应该排到 B 后面，实际: %v", got)
		}
	})
}

// TestColumnClearNextColumn 验证只清除同一看板里指向目标列的引用，
// 指向其他列的引用和别的看板的数据都不受影响。
func TestColumnClearNextColumn(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Column()
		target := addColumn(t, store, 1, "目标", 3)
		other := addColumn(t, store, 1, "其他", 4)
		pointsToTarget := &kanban.Column{BoardID: 1, Name: "指向目标", Sort: 1, NextColumnID: target.ID}
		pointsToOther := &kanban.Column{BoardID: 1, Name: "指向其他", Sort: 2, NextColumnID: other.ID}
		foreign := &kanban.Column{BoardID: 2, Name: "别的看板", Sort: 1, NextColumnID: target.ID}
		for _, c := range []*kanban.Column{pointsToTarget, pointsToOther, foreign} {
			if err := store.Create(ctx, c); err != nil {
				t.Fatalf("创建列失败: %v", err)
			}
		}

		if err := store.ClearNextColumn(ctx, 1, target.ID); err != nil {
			t.Fatalf("清除引用失败: %v", err)
		}
		check := func(id, want int64, what string) {
			t.Helper()
			got, err := store.GetByID(ctx, id)
			if err != nil || got.NextColumnID != want {
				t.Fatalf("%s: 下一列应该是 %d，实际 %+v %v", what, want, got, err)
			}
		}
		check(pointsToTarget.ID, 0, "指向目标的列应该被清除")
		check(pointsToOther.ID, other.ID, "指向其他列的引用应该保留")
		check(foreign.ID, target.ID, "别的看板的数据不应该被改动")
	})
}

// TestColumnDelete 验证删除幂等，以及按看板整体删除只影响本看板。
func TestColumnDelete(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Column()
		c := addColumn(t, store, 1, "A", 1)
		addColumn(t, store, 1, "B", 2)
		keep := addColumn(t, store, 2, "别的看板", 1)

		for range 2 {
			if err := store.Delete(ctx, &kanban.Column{ID: c.ID}); err != nil {
				t.Fatalf("删除应该成功: %v", err)
			}
		}
		if _, err := store.GetByID(ctx, c.ID); !orm.IsErrRecordNotFound(err) {
			t.Fatalf("删除后应该找不到，实际: %v", err)
		}

		if err := store.DeleteByBoard(ctx, 1); err != nil {
			t.Fatalf("按看板删除失败: %v", err)
		}
		if items, _ := store.ListByBoard(ctx, 1); len(items) != 0 {
			t.Fatalf("看板 1 的列应该全部删除，剩余 %d", len(items))
		}
		if _, err := store.GetByID(ctx, keep.ID); err != nil {
			t.Fatalf("别的看板的列不应该被删除: %v", err)
		}
	})
}
