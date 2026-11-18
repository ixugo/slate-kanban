package kanbandb_test

import (
	"context"
	"slices"
	"testing"

	"github.com/ixugo/goddd/pkg/orm"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/kanbantest"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
)

// addRule 写入一条测试规则并返回。
func addRule(t *testing.T, store kanban.RuleStorer, boardID int64, name string, sort int64) *kanban.Rule {
	t.Helper()
	r := &kanban.Rule{
		BoardID: boardID, Name: name, Enabled: true, TriggerType: kanban.TriggerCardEntered, Sort: sort,
		Actions: []kanban.RuleAction{{Type: kanban.ActionSetStartedAt, Mode: kanban.ModeIfEmpty}},
	}
	if err := store.Create(context.Background(), r); err != nil {
		t.Fatalf("创建规则失败: %v", err)
	}
	return r
}

// TestRuleListByBoardOrder 验证列表只包含本看板的规则（包括停用的），按排序值升序，排序值相同按主键升序，
// 没有规则时返回空数组。
func TestRuleListByBoardOrder(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Rule()
		addRule(t, store, 1, "后", 3072)
		addRule(t, store, 1, "先", 1024)
		disabled := addRule(t, store, 1, "停用的中间", 2048)
		addRule(t, store, 1, "同序后建", 2048)
		addRule(t, store, 2, "别的看板", 1)
		disabled.Enabled = false
		if err := store.Save(ctx, disabled); err != nil {
			t.Fatalf("停用失败: %v", err)
		}

		items, err := store.ListByBoard(ctx, 1)
		if err != nil {
			t.Fatalf("列表失败: %v", err)
		}
		var got []string
		for _, r := range items {
			got = append(got, r.Name)
		}
		if want := []string{"先", "停用的中间", "同序后建", "后"}; !slices.Equal(got, want) {
			t.Fatalf("顺序不对，期望 %v，实际 %v", want, got)
		}
		if empty, err := store.ListByBoard(ctx, 999); err != nil || empty == nil || len(empty) != 0 {
			t.Fatalf("没有规则应该返回空数组: %v %v", empty, err)
		}
	})
}

// TestRuleSaveWritesZeroValuesAndActions 验证 Save 会把 false 写回去，动作列表整体保存并能原样读出，
// 否则停用规则、修改动作的操作会悄悄丢失。
func TestRuleSaveWritesZeroValuesAndActions(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Rule()
		r := addRule(t, store, 1, "规则", 1)

		r.Enabled = false
		r.ColumnID = 0
		r.Actions = []kanban.RuleAction{
			{Type: kanban.ActionAddTag, Tag: "x"},
			{Type: kanban.ActionSetHumanOnly, Flag: true},
			{Type: kanban.ActionMoveToColumn, ColumnID: 7},
		}
		if err := store.Save(ctx, r); err != nil {
			t.Fatalf("保存失败: %v", err)
		}
		got, err := store.GetByID(ctx, r.ID)
		if err != nil {
			t.Fatalf("读取失败: %v", err)
		}
		if got.Enabled || !slices.Equal(got.Actions, r.Actions) || got.TriggerType != kanban.TriggerCardEntered {
			t.Fatalf("保存的内容不对: %+v", got)
		}
		if _, err := store.GetByID(ctx, 9999); !orm.IsErrRecordNotFound(err) {
			t.Fatalf("不存在的主键应该是找不到记录，实际: %v", err)
		}
	})
}

// TestRuleNilActionsStoredAsEmptyArray 验证没有动作的规则读出来是空数组而不是 nil，JSON 里是 []。
func TestRuleNilActionsStoredAsEmptyArray(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Rule()
		r := &kanban.Rule{BoardID: 1, Name: "无动作", TriggerType: kanban.TriggerCardCreated}
		if err := store.Create(ctx, r); err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		got, err := store.GetByID(ctx, r.ID)
		if err != nil || got.Actions == nil || len(got.Actions) != 0 {
			t.Fatalf("动作应该是空数组: %+v %v", got, err)
		}
		items, err := store.ListByBoard(ctx, 1)
		if err != nil || len(items) != 1 || items[0].Actions == nil {
			t.Fatalf("列表里的动作也应该是空数组: %+v %v", items, err)
		}
	})
}

// TestRuleDelete 验证单条删除幂等，按看板删除只影响自己的范围。
func TestRuleDelete(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Rule()
		a := addRule(t, store, 1, "a", 1)
		addRule(t, store, 1, "b", 2)
		addRule(t, store, 2, "c", 1)

		for range 2 {
			if err := store.Delete(ctx, &kanban.Rule{ID: a.ID}); err != nil {
				t.Fatalf("删除应该成功: %v", err)
			}
		}
		if items, _ := store.ListByBoard(ctx, 1); len(items) != 1 || items[0].Name != "b" {
			t.Fatalf("应该只剩 b: %+v", items)
		}
		if err := store.DeleteByBoard(ctx, 1); err != nil {
			t.Fatalf("按看板删除失败: %v", err)
		}
		if items, _ := store.ListByBoard(ctx, 1); len(items) != 0 {
			t.Fatalf("看板 1 的规则应该全部删除: %+v", items)
		}
		if items, _ := store.ListByBoard(ctx, 2); len(items) != 1 {
			t.Fatalf("别的看板的规则不应该受影响: %+v", items)
		}
	})
}

// TestRuleWithTx 验证事务绑定：事务里新增的规则回滚后消失，提交后可见。
func TestRuleWithTx(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()

		tx, err := db.Begin()
		if err != nil {
			t.Fatalf("开启事务失败: %v", err)
		}
		addRule(t, db.Rule().WithTx(tx), 1, "回滚的", 1)
		if err := tx.Rollback(); err != nil {
			t.Fatalf("回滚失败: %v", err)
		}
		if items, _ := db.Rule().ListByBoard(ctx, 1); len(items) != 0 {
			t.Fatalf("回滚后不应该有规则: %+v", items)
		}

		tx, err = db.Begin()
		if err != nil {
			t.Fatalf("开启事务失败: %v", err)
		}
		addRule(t, db.Rule().WithTx(tx), 1, "提交的", 1)
		if err := tx.Commit(); err != nil {
			t.Fatalf("提交失败: %v", err)
		}
		if items, _ := db.Rule().ListByBoard(ctx, 1); len(items) != 1 {
			t.Fatalf("提交后应该有规则: %+v", items)
		}
	})
}
