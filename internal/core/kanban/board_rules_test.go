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

// TestCreateBoardSeedsDefaultRules 验证 dev 模板创建看板时同时创建三条起步规则：
// 都启用，按开始计时、完成计时、重新打开的顺序，分别绑定进行中和完成列，写时间只在为空时写。
func TestCreateBoardSeedsDefaultRules(t *testing.T) {
	core, _ := newBare(t)
	board := mustCreateBoard(t, core, kanban.CreateBoardInput{Name: "起步", Slug: "SEED", Locale: "zh-CN"})
	rules := board.Rules
	if got, want := ruleTitles(rules), []string{"开始计时", "完成计时", "重新打开"}; !slices.Equal(got, want) {
		t.Fatalf("起步规则名称应该是 %v，实际 %v", want, got)
	}
	doing, done := board.Columns[colDoing].ID, board.Columns[colDone].ID
	want := []struct {
		trigger string
		column  int64
		action  kanban.RuleAction
	}{
		{kanban.TriggerCardEntered, doing, kanban.RuleAction{Type: kanban.ActionSetStartedAt, Mode: kanban.ModeIfEmpty}},
		{kanban.TriggerCardEntered, done, kanban.RuleAction{Type: kanban.ActionSetFinishedAt, Mode: kanban.ModeIfEmpty}},
		{kanban.TriggerCardLeft, done, kanban.RuleAction{Type: kanban.ActionClearFinishedAt}},
	}
	for i, w := range want {
		r := rules[i]
		if !r.Enabled || r.BoardID != board.ID || r.TriggerType != w.trigger || r.ColumnID != w.column ||
			!slices.Equal(r.Actions, []kanban.RuleAction{w.action}) {
			t.Errorf("第 %d 条规则不对: %+v", i, r)
		}
		if i > 0 && r.Sort <= rules[i-1].Sort {
			t.Errorf("规则的排序值应该递增: %d 之后是 %d", rules[i-1].Sort, r.Sort)
		}
	}
}

// TestSeededRulesSurviveReload 验证起步规则落了库：GetBoard 读到的和创建时返回的一致，
// 并且它们能通过完整校验（空修改不报错、不写事件）。
func TestSeededRulesSurviveReload(t *testing.T) {
	ctx := context.Background()
	core, db := newBare(t)
	board := mustCreateBoard(t, core, kanban.CreateBoardInput{Name: "起步", Slug: "SEED", Locale: "zh-CN"})
	got, err := core.GetBoard(ctx, board.ID)
	if err != nil {
		t.Fatalf("读取看板失败: %v", err)
	}
	if !slices.Equal(ruleTitles(got.Rules), ruleTitles(board.Rules)) || len(got.Rules) != 3 {
		t.Fatalf("重新读取的规则和创建时不一致: %v", ruleTitles(got.Rules))
	}
	events := len(boardEvents(t, db, board.ID))
	for _, r := range got.Rules {
		if _, err := core.UpdateRule(ctx, testActor, r.ID, kanban.UpdateRuleInput{}); err != nil {
			t.Errorf("起步规则 %q 应该能通过校验: %v", r.Name, err)
		}
	}
	if len(boardEvents(t, db, board.ID)) != events {
		t.Fatalf("空修改不应该写事件")
	}
}

// TestSeededRulesLocaleAndTemplates 验证起步规则的名称跟随界面语言，simple 模板也有三条且绑定自己的列，
// empty 模板没有规则。
func TestSeededRulesLocaleAndTemplates(t *testing.T) {
	core, _ := newBare(t)
	simple := mustCreateBoard(t, core, kanban.CreateBoardInput{Name: "S1", Template: kanban.TemplateSimple, Locale: "en"})
	if got, want := ruleTitles(simple.Rules), []string{"Start timer", "Finish timer", "Reopen"}; !slices.Equal(got, want) {
		t.Fatalf("英文名称应该是 %v，实际 %v", want, got)
	}
	if simple.Rules[0].ColumnID != simple.Columns[1].ID || simple.Rules[1].ColumnID != simple.Columns[2].ID {
		t.Fatalf("simple 的规则应该绑定自己的进行中和完成列: %+v", simple.Rules)
	}
	empty := mustCreateBoard(t, core, kanban.CreateBoardInput{Name: "E1", Template: kanban.TemplateEmpty})
	if empty.Rules == nil || len(empty.Rules) != 0 {
		t.Fatalf("empty 模板不应该有规则，且是空数组: %+v", empty.Rules)
	}
	got, err := core.GetBoard(context.Background(), empty.ID)
	if err != nil || got.Rules == nil || len(got.Rules) != 0 {
		t.Fatalf("读回的 empty 模板规则也应该是空数组: %+v %v", got, err)
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
