package kanban_test

import (
	"context"
	"slices"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// TestGetBoardListsRules 验证 GetBoard 把规则（包括停用的）按执行顺序列出来，没有规则时是空数组，
// 这样界面能显示并重新启用停用的规则。
func TestGetBoardListsRules(t *testing.T) {
	ctx := context.Background()
	core, _ := newBare(t)
	empty := mustCreateBoard(t, core, kanban.CreateBoardInput{Name: "空板", Template: kanban.TemplateEmpty})
	got, err := core.GetBoard(ctx, empty.ID)
	if err != nil || got.Rules == nil || len(got.Rules) != 0 {
		t.Fatalf("没有规则应该是空数组: %+v %v", got, err)
	}

	off := false
	for _, in := range []kanban.CreateRuleInput{{Name: "先", Enabled: &off}, {Name: "后"}} {
		in.Trigger, in.Actions = kanban.TriggerCardCreated, []kanban.RuleAction{startedAction}
		if _, err := core.CreateRule(ctx, testActor, empty.ID, in); err != nil {
			t.Fatalf("创建规则 %s 失败: %v", in.Name, err)
		}
	}
	got, err = core.GetBoard(ctx, empty.ID)
	if err != nil {
		t.Fatalf("读取看板失败: %v", err)
	}
	if want := []string{"先", "后"}; !slices.Equal(ruleTitles(got.Rules), want) {
		t.Fatalf("顺序应该是 %v，实际 %v", want, ruleTitles(got.Rules))
	}
	if got.Rules[0].Enabled || !got.Rules[1].Enabled {
		t.Fatalf("停用状态不对: %+v %+v", got.Rules[0], got.Rules[1])
	}
}

// ruleTitles 返回一组规则的名称。
func ruleTitles(rules []*kanban.Rule) []string {
	names := make([]string, 0, len(rules))
	for _, r := range rules {
		names = append(names, r.Name)
	}
	return names
}
