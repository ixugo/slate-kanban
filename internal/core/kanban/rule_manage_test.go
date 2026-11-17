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

// newRuleInput 返回一条合法的创建入参：卡片进入某列时写开始时间。
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

// ruleNames 返回看板里全部规则的名称，顺序就是执行顺序。
func (e *cardEnv) ruleNames(t *testing.T) []string {
	t.Helper()
	rules, err := e.core.ListRules(context.Background(), e.board.ID)
	if err != nil {
		t.Fatalf("列规则失败: %v", err)
	}
	names := make([]string, 0, len(rules))
	for _, r := range rules {
		names = append(names, r.Name)
	}
	return names
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
