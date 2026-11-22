package kanban_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// startedAction 是测试里最常用的动作：写开始时间。
var startedAction = kanban.RuleAction{Type: kanban.ActionSetStartedAt}

// newRuleInput 返回一条合法的创建入参：任务进入某列时写开始时间。
func newRuleInput(name string, column int64) kanban.CreateRuleInput {
	return kanban.CreateRuleInput{
		Name: name, Trigger: kanban.TriggerCardEntered, ColumnID: column,
		Actions: []kanban.RuleAction{startedAction},
	}
}

// mustCreateRule 以人的身份创建规则，失败就终止测试。
func (e *cardEnv) mustCreateRule(t *testing.T, in kanban.CreateRuleInput) *kanban.Rule {
	t.Helper()
	r, err := e.core.CreateRule(context.Background(), testActor, e.board.ID, in)
	if err != nil {
		t.Fatalf("创建规则 %s 失败: %v", in.Name, err)
	}
	return r
}

// storedRule 直接从数据库读出规则的当前状态。
func (e *cardEnv) storedRule(t *testing.T, id int64) *kanban.Rule {
	t.Helper()
	r, err := e.db.Rule().GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("读取规则 %d 失败: %v", id, err)
	}
	return r
}

// ruleNames 返回看板里全部规则的名称，顺序就是执行顺序。
func (e *cardEnv) ruleNames(t *testing.T) []string {
	t.Helper()
	rules, err := e.core.ListRules(context.Background(), e.board.ID)
	if err != nil {
		t.Fatalf("列规则失败: %v", err)
	}
	return ruleTitles(rules)
}

// TestCreateRule 验证创建：名称被整理，默认启用，规则排在已有规则后面，动作被整理成只含自己用到的字段，
// 并追加 rule.created 事件。
func TestCreateRule(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	before := len(e.ruleNames(t))

	r, err := e.core.CreateRule(ctx, testActor, e.board.ID, kanban.CreateRuleInput{
		Name: "  进入进行中  ", Trigger: kanban.TriggerCardEntered, ColumnID: e.col(colDoing), Condition: " -is:started ",
		Actions: []kanban.RuleAction{{Type: kanban.ActionSetStartedAt, Tag: "多余字段会被丢掉"}},
	})
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if r.ID == 0 || r.Name != "进入进行中" || !r.Enabled || r.Condition != "-is:started" || r.BoardID != e.board.ID {
		t.Fatalf("规则内容不对: %+v", r)
	}
	if want := (kanban.RuleAction{Type: kanban.ActionSetStartedAt, Mode: kanban.ModeIfEmpty}); !slices.Equal(r.Actions, []kanban.RuleAction{want}) {
		t.Fatalf("动作应该被整理: %+v", r.Actions)
	}
	if names := e.ruleNames(t); len(names) != before+1 || names[before] != "进入进行中" {
		t.Fatalf("新规则应该排在最后: %v", names)
	}
	want := fmt.Sprintf(`{"name":"进入进行中","rule_id":%d}`, r.ID)
	if ev := e.lastEvent(t); ev.Type != kanban.EventRuleCreated || ev.CardID != 0 || ev.Payload != want {
		t.Fatalf("事件不对: %+v", ev)
	}
}

// TestCreateRuleEnabledFalse 验证显式传 enabled=false 时创建出来的规则是停用的。
func TestCreateRuleEnabledFalse(t *testing.T) {
	e := newCardEnv(t)
	off := false
	in := newRuleInput("停用的", 0)
	in.Enabled = &off
	if r := e.mustCreateRule(t, in); r.Enabled {
		t.Fatalf("应该是停用的: %+v", r)
	}
}

// TestCreateRuleOrder 验证连续创建的规则排序值严格递增，列表顺序就是创建顺序。
func TestCreateRuleOrder(t *testing.T) {
	e := newCardEnv(t)
	base := len(e.ruleNames(t))
	var last int64
	for i, name := range []string{"一", "二", "三"} {
		r := e.mustCreateRule(t, newRuleInput(name, 0))
		if i > 0 && r.Sort <= last {
			t.Fatalf("排序值应该递增: %d 之后是 %d", last, r.Sort)
		}
		last = r.Sort
	}
	if names := e.ruleNames(t); !slices.Equal(names[base:], []string{"一", "二", "三"}) {
		t.Fatalf("顺序不对: %v", names)
	}
}

// TestCreateRuleDenied 验证只有人能创建规则：agent、自动化、身份无效、看板不存在都被拒绝，且不写事件。
func TestCreateRuleDenied(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	events := len(e.events(t))

	for name, tc := range map[string]struct {
		actor kanban.Actor
		board int64
		want  error
	}{
		"agent": {testAgent, e.board.ID, kanban.ErrHumanOnly},
		"自动化":   {kanban.AutomationActor("x"), e.board.ID, kanban.ErrHumanOnly},
		"身份无效":  {kanban.Actor{}, e.board.ID, kanban.ErrInvalidActor},
		"看板不存在": {testActor, 9999, kanban.ErrBoardNotFound},
	} {
		if _, err := e.core.CreateRule(ctx, tc.actor, tc.board, newRuleInput("x", 0)); !errors.Is(err, tc.want) {
			t.Errorf("%s：期望 %v，实际 %v", name, tc.want, err)
		}
	}
	if len(e.events(t)) != events {
		t.Fatalf("被拒绝的创建不应该写事件")
	}
}

// TestCreateRuleInvalid 验证内容不合法的规则被拒绝，并且没有残留。
func TestCreateRuleInvalid(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	before := len(e.ruleNames(t))

	bad := newRuleInput("", 0)
	if _, err := e.core.CreateRule(ctx, testActor, e.board.ID, bad); !errors.Is(err, kanban.ErrInvalidInput) {
		t.Fatalf("空名称应该被拒绝: %v", err)
	}
	bad = newRuleInput("x", 0)
	bad.Trigger = "card_exploded"
	if _, err := e.core.CreateRule(ctx, testActor, e.board.ID, bad); !errors.Is(err, kanban.ErrInvalidInput) {
		t.Fatalf("未知触发类型应该被拒绝: %v", err)
	}
	bad = newRuleInput("x", 9999)
	if _, err := e.core.CreateRule(ctx, testActor, e.board.ID, bad); !errors.Is(err, kanban.ErrInvalidInput) {
		t.Fatalf("不存在的列应该被拒绝: %v", err)
	}
	bad = newRuleInput("x", 0)
	bad.Actions = nil
	if _, err := e.core.CreateRule(ctx, testActor, e.board.ID, bad); !errors.Is(err, kanban.ErrInvalidInput) {
		t.Fatalf("没有动作应该被拒绝: %v", err)
	}
	if got := len(e.ruleNames(t)); got != before {
		t.Fatalf("被拒绝的创建不应该留下规则: %d -> %d", before, got)
	}
}

// TestCreateRuleLimit 验证每个看板最多 50 条规则，第 51 条被拒绝，别的看板不受影响。
func TestCreateRuleLimit(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	for len(e.ruleNames(t)) < 50 {
		e.mustCreateRule(t, newRuleInput(fmt.Sprintf("规则 %d", len(e.ruleNames(t))), 0))
	}
	if _, err := e.core.CreateRule(ctx, testActor, e.board.ID, newRuleInput("第 51 条", 0)); !errors.Is(err, kanban.ErrRuleLimit) {
		t.Fatalf("超过上限应该返回 ErrRuleLimit: %v", err)
	}
	other := mustCreateBoard(t, e.core, kanban.CreateBoardInput{Name: "另一个", Slug: "OTH", Locale: "zh-CN", Template: "empty"})
	if _, err := e.core.CreateRule(ctx, testActor, other.ID, kanban.CreateRuleInput{
		Name: "别的看板", Trigger: kanban.TriggerCardCreated, Actions: []kanban.RuleAction{startedAction},
	}); err != nil {
		t.Fatalf("别的看板不受上限影响: %v", err)
	}
}

// TestListRules 验证读取：agent 可以读，看板不存在返回 ErrBoardNotFound。
func TestListRules(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	e.mustCreateRule(t, newRuleInput("读我", 0))
	rules, err := e.core.ListRules(ctx, e.board.ID)
	if err != nil || len(rules) == 0 || rules[len(rules)-1].Name != "读我" {
		t.Fatalf("应该能读到规则: %+v %v", rules, err)
	}
	if _, err := e.core.ListRules(ctx, 9999); !errors.Is(err, kanban.ErrBoardNotFound) {
		t.Fatalf("看板不存在应该返回 ErrBoardNotFound: %v", err)
	}
}

// TestUpdateRulePartial 验证只改出现的字段：改名、停用、换动作各自独立生效，其余字段保持不变，
// 每次真的有变化都追加 rule.updated 事件。
func TestUpdateRulePartial(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	r := e.mustCreateRule(t, newRuleInput("原名", e.col(colDoing)))

	name, off := "新名", false
	got, err := e.core.UpdateRule(ctx, testActor, r.ID, kanban.UpdateRuleInput{Name: &name, Enabled: &off})
	if err != nil || got.Name != "新名" || got.Enabled || got.ColumnID != e.col(colDoing) || got.TriggerType != kanban.TriggerCardEntered {
		t.Fatalf("改名和停用不对: %+v %v", got, err)
	}
	actions := []kanban.RuleAction{{Type: kanban.ActionSetPriority, Priority: 3}, {Type: kanban.ActionAddTag, Tag: "急"}}
	trigger, anyColumn := kanban.TriggerCardLeft, int64(0)
	got, err = e.core.UpdateRule(ctx, testActor, r.ID, kanban.UpdateRuleInput{Actions: &actions, Trigger: &trigger, ColumnID: &anyColumn})
	if err != nil || !slices.Equal(got.Actions, actions) || got.TriggerType != kanban.TriggerCardLeft || got.ColumnID != 0 || got.Name != "新名" {
		t.Fatalf("换动作和触发不对: %+v %v", got, err)
	}
	stored := e.storedRule(t, r.ID)
	if stored.Name != "新名" || stored.Enabled || !slices.Equal(stored.Actions, actions) {
		t.Fatalf("库里的内容不对: %+v", stored)
	}
	want := fmt.Sprintf(`{"name":"新名","rule_id":%d}`, r.ID)
	if ev := e.lastEvent(t); ev.Type != kanban.EventRuleUpdated || ev.Payload != want {
		t.Fatalf("事件不对: %+v", ev)
	}
}

// TestUpdateRuleNoChange 验证没有任何字段真的变化时不写库、不写事件，更新时间也不变。
func TestUpdateRuleNoChange(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	r := e.mustCreateRule(t, newRuleInput("原名", 0))
	first := e.storedRule(t, r.ID)
	events := len(e.events(t))
	same := "  原名 "

	got, err := e.core.UpdateRule(ctx, testActor, r.ID, kanban.UpdateRuleInput{Name: &same})
	if err != nil || got.Name != "原名" {
		t.Fatalf("没变化应该成功: %+v %v", got, err)
	}
	if _, err := e.core.UpdateRule(ctx, testActor, r.ID, kanban.UpdateRuleInput{}); err != nil {
		t.Fatalf("空修改应该成功: %v", err)
	}
	if len(e.events(t)) != events {
		t.Fatalf("没有变化不应该写事件")
	}
	if stored := e.storedRule(t, r.ID); !stored.UpdatedAt.Equal(first.UpdatedAt) {
		t.Fatalf("没有变化不应该动更新时间: %v -> %v", first.UpdatedAt, stored.UpdatedAt)
	}
}

// TestUpdateRuleRejected 验证被拒绝的修改不留任何痕迹：非法内容、非人身份、规则不存在。
func TestUpdateRuleRejected(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	r := e.mustCreateRule(t, newRuleInput("原名", 0))
	events := len(e.events(t))

	empty, none := "", []kanban.RuleAction{}
	for name, in := range map[string]kanban.UpdateRuleInput{
		"空名称":  {Name: &empty},
		"空动作":  {Actions: &none},
		"错误条件": {Condition: strPtr("nosuch:x")},
	} {
		if _, err := e.core.UpdateRule(ctx, testActor, r.ID, in); !errors.Is(err, kanban.ErrInvalidInput) {
			t.Errorf("%s：应该返回 ErrInvalidInput，实际 %v", name, err)
		}
	}
	newName := "改名"
	for name, tc := range map[string]struct {
		actor kanban.Actor
		id    int64
		want  error
	}{
		"agent": {testAgent, r.ID, kanban.ErrHumanOnly},
		"自动化":   {kanban.AutomationActor("x"), r.ID, kanban.ErrHumanOnly},
		"规则不存在": {testActor, 9999, kanban.ErrRuleNotFound},
	} {
		if _, err := e.core.UpdateRule(ctx, tc.actor, tc.id, kanban.UpdateRuleInput{Name: &newName}); !errors.Is(err, tc.want) {
			t.Errorf("%s：期望 %v，实际 %v", name, tc.want, err)
		}
	}
	if stored := e.storedRule(t, r.ID); stored.Name != "原名" || len(e.events(t)) != events {
		t.Fatalf("被拒绝的修改不应该有任何痕迹: %+v", stored)
	}
}

// TestDeleteRule 验证删除：规则消失并追加 rule.deleted 事件，重复删除幂等且不再写事件；
// agent 和自动化不能删，规则保留。
func TestDeleteRule(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	keep := e.mustCreateRule(t, newRuleInput("保留", 0))
	gone := e.mustCreateRule(t, newRuleInput("删掉", 0))

	for name, actor := range map[string]kanban.Actor{"agent": testAgent, "自动化": kanban.AutomationActor("x")} {
		if err := e.core.DeleteRule(ctx, actor, gone.ID); !errors.Is(err, kanban.ErrHumanOnly) {
			t.Errorf("%s 不能删规则，实际: %v", name, err)
		}
	}
	if !slices.Contains(e.ruleNames(t), "删掉") {
		t.Fatalf("被拒绝的删除不应该删规则")
	}

	if err := e.core.DeleteRule(ctx, testActor, gone.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	want := fmt.Sprintf(`{"name":"删掉","rule_id":%d}`, gone.ID)
	if ev := e.lastEvent(t); ev.Type != kanban.EventRuleDeleted || ev.Payload != want {
		t.Fatalf("事件不对: %+v", ev)
	}
	events := len(e.events(t))
	if err := e.core.DeleteRule(ctx, testActor, gone.ID); err != nil {
		t.Fatalf("重复删除应该成功: %v", err)
	}
	if len(e.events(t)) != events {
		t.Fatalf("重复删除不应该写事件")
	}
	if names := e.ruleNames(t); slices.Contains(names, "删掉") || !slices.Contains(names, keep.Name) {
		t.Fatalf("只应该删掉目标规则: %v", names)
	}
}

// TestRejectNewDeadlineRule 确认 REST/MCP 共用的规则入口拒绝新增截止时间动作。
func TestRejectNewDeadlineRule(t *testing.T) {
	e := newClaimEnv(t)
	for _, action := range []kanban.RuleAction{{Type: kanban.ActionSetDue, Days: 1}, {Type: kanban.ActionClearDue}} {
		t.Run(action.Type, func(t *testing.T) {
			input := newRuleInput("旧截止时间动作", e.col(colDoing))
			input.Actions = []kanban.RuleAction{action}
			if _, err := e.core.CreateRule(t.Context(), testActor, e.board.ID, input); !errors.Is(err, kanban.ErrInvalidInput) {
				t.Fatalf("应拒绝新增截止时间动作，实际 %v", err)
			}
		})
	}
}
