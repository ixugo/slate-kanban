package kanban_test

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// moveRule 创建一条"卡片进入 from 列就移到 to 列"的规则，from 和 to 是列的下标。
func (e *cardEnv) moveRule(t *testing.T, name string, from, to int) *kanban.Rule {
	t.Helper()
	return e.enterRule(t, name, e.col(from), kanban.RuleAction{Type: kanban.ActionMoveToColumn, ColumnID: e.col(to)})
}

// TestRuleMoveToColumn 验证移动列动作：卡片被移到目标列末尾，卡片只保存一次（版本只加一），
// 事件顺序是人的移动、规则的移动（操作者是 system）、规则的执行记录。
func TestRuleMoveToColumn(t *testing.T) {
	e := newClaimEnv(t)
	e.moveRule(t, "进行中自动送审", colDoing, colReview)
	e.card(t, colReview, "审查里已有的卡")
	c := e.card(t, colTodo, "A")
	events := len(e.events(t))

	moved := e.moveTo(t, testActor, c.ID, colDoing)
	if moved.ColumnID != e.col(colReview) || moved.Position != 2048 || moved.Version != 2 {
		t.Fatalf("应该在审查列末尾，版本只加一次: %+v", moved)
	}
	if stored := e.stored(t, c.ID); stored.ColumnID != e.col(colReview) || stored.Position != 2048 {
		t.Fatalf("库里的卡片不对: %+v", stored)
	}
	want := []string{kanban.EventCardMoved, kanban.EventCardMoved, kanban.EventAutomationFired}
	if got := e.eventTypes(t, events); !slices.Equal(got, want) {
		t.Fatalf("事件顺序应该是 %v，实际 %v", want, got)
	}
	evs := e.events(t)[events:]
	if evs[0].ActorKind != "human" || evs[1].ActorKind != "system" || evs[1].Actor != "automation:进行中自动送审" {
		t.Fatalf("两次移动的操作者不对: %+v %+v", evs[0], evs[1])
	}
	wantMoved := `{"from":"进行中","from_id":3,"key":"TST-2","title":"A","to":"审查","to_id":4}`
	if evs[1].Payload != wantMoved {
		t.Fatalf("规则的移动事件内容不对: %s", evs[1].Payload)
	}
}

// TestRuleMoveChainsToOtherRules 验证规则的移动会继续触发规则：
// 离开进行中、进入审查的规则都会执行，执行记录里先是被连锁触发的，最后才是引发连锁的那条。
func TestRuleMoveChainsToOtherRules(t *testing.T) {
	e := newClaimEnv(t)
	e.moveRule(t, "自动送审", colDoing, colReview)
	e.enterRule(t, "审查加标签", e.col(colReview), kanban.RuleAction{Type: kanban.ActionAddTag, Tag: "待审查"})
	e.mustCreateRule(t, kanban.CreateRuleInput{
		Name: "离开进行中提优先级", Trigger: kanban.TriggerCardLeft, ColumnID: e.col(colDoing),
		Actions: []kanban.RuleAction{{Type: kanban.ActionSetPriority, Priority: 2}},
	})
	c := e.card(t, colTodo, "A")

	moved := e.moveTo(t, testActor, c.ID, colDoing)
	if moved.ColumnID != e.col(colReview) || !slices.Equal(moved.Tags, []string{"待审查"}) || moved.Priority != 2 || moved.Version != 2 {
		t.Fatalf("连锁的效果都应该生效，版本只加一次: %+v", moved)
	}
	if names := firedRuleNames(e.fired(t)); !slices.Equal(names, []string{"离开进行中提优先级", "审查加标签", "自动送审"}) {
		t.Fatalf("执行记录的顺序不对: %v", names)
	}
}

// TestRuleLoopProtection 验证规则互相把卡片移来移去不会死循环：同一条规则一次操作里只执行一次，
// 所以进入进行中移去审查、进入审查又移回进行中，到这里就停了，两条规则各执行一次。
func TestRuleLoopProtection(t *testing.T) {
	e := newClaimEnv(t)
	e.moveRule(t, "进行中去审查", colDoing, colReview)
	e.moveRule(t, "审查回进行中", colReview, colDoing)
	c := e.card(t, colTodo, "A")

	moved := e.moveTo(t, testActor, c.ID, colDoing)
	if moved.ColumnID != e.col(colDoing) || moved.Version != 2 {
		t.Fatalf("应该绕一圈回到进行中就停下: %+v", moved)
	}
	if names := firedRuleNames(e.fired(t)); !slices.Equal(names, []string{"审查回进行中", "进行中去审查"}) {
		t.Fatalf("两条规则应该各执行一次: %v", names)
	}
}

// TestRuleDepthLimit 验证连锁最多 3 层：待办→进行中→审查→完成→收件箱一共四次规则移动，
// 第四次移动已经发生，但它引发的"进入收件箱"规则不再触发。
func TestRuleDepthLimit(t *testing.T) {
	e := newClaimEnv(t)
	c := e.card(t, colIdea, "A")
	e.moveRule(t, "一", colTodo, colDoing)
	e.moveRule(t, "二", colDoing, colReview)
	e.moveRule(t, "三", colReview, colDone)
	e.moveRule(t, "四", colDone, colIdea)
	e.enterRule(t, "五", e.col(colIdea), kanban.RuleAction{Type: kanban.ActionSetPriority, Priority: 3})

	moved := e.moveTo(t, testActor, c.ID, colTodo)
	if moved.ColumnID != e.col(colIdea) || moved.Priority != 0 {
		t.Fatalf("第四次移动应该发生，第五条规则不应该触发: %+v", moved)
	}
	if names := firedRuleNames(e.fired(t)); !slices.Equal(names, []string{"四", "三", "二", "一"}) {
		t.Fatalf("应该只有前四条规则执行: %v", names)
	}
}

// TestRuleMoveOnCreate 验证创建卡片时规则可以把它移走：创建出来就在目标列，
// 卡片只保存一次（第二版），事件是创建、规则的移动、规则的执行记录。
func TestRuleMoveOnCreate(t *testing.T) {
	e := newClaimEnv(t)
	e.mustCreateRule(t, kanban.CreateRuleInput{
		Name: "新想法进待办", Trigger: kanban.TriggerCardCreated, ColumnID: e.col(colIdea),
		Actions: []kanban.RuleAction{{Type: kanban.ActionMoveToColumn, ColumnID: e.col(colTodo)}},
	})
	events := len(e.events(t))

	c := e.card(t, colIdea, "A")
	if c.ColumnID != e.col(colTodo) || c.Version != 2 || e.stored(t, c.ID).ColumnID != e.col(colTodo) {
		t.Fatalf("创建出来就应该在待办列，且是第二版: %+v", c)
	}
	want := []string{kanban.EventCardCreated, kanban.EventCardMoved, kanban.EventAutomationFired}
	if got := e.eventTypes(t, events); !slices.Equal(got, want) {
		t.Fatalf("事件顺序应该是 %v，实际 %v", want, got)
	}
}

// TestRuleMoveToSameColumnIsNoop 验证目标列就是卡片当前所在的列时什么都不做：没有事件，版本只因为人的移动涨一次。
func TestRuleMoveToSameColumnIsNoop(t *testing.T) {
	e := newClaimEnv(t)
	e.moveRule(t, "原地不动", colDoing, colDoing)
	c := e.card(t, colTodo, "A")

	moved := e.moveTo(t, testActor, c.ID, colDoing)
	if moved.ColumnID != e.col(colDoing) || moved.Version != 2 || len(e.fired(t)) != 0 {
		t.Fatalf("目标列就是当前列应该什么都不做: %+v", moved)
	}
}

// TestRuleMoveToMissingColumnIsSkipped 验证目标列已经不存在时，动作被跳过并记录原因，
// 用户的移动照常成功，其余动作照常执行。
func TestRuleMoveToMissingColumnIsSkipped(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	r := e.enterRule(t, "目标列没了", e.col(colDoing),
		kanban.RuleAction{Type: kanban.ActionMoveToColumn, ColumnID: e.col(colReview)}, kanban.RuleAction{Type: kanban.ActionSetPriority, Priority: 1})
	r.Actions[0].ColumnID = 9999 // 模拟目标列在规则保存之后被删掉
	if err := e.db.Rule().Save(ctx, r); err != nil {
		t.Fatalf("写入规则失败: %v", err)
	}
	c := e.card(t, colTodo, "A")

	moved := e.moveTo(t, testActor, c.ID, colDoing)
	if moved.ColumnID != e.col(colDoing) || moved.Priority != 1 {
		t.Fatalf("移动成功、其余动作照常执行: %+v", moved)
	}
	fired := e.fired(t)
	want := fmt.Sprintf(`{"actions":["set_priority"],"key":"TST-1","rule":"目标列没了","rule_id":%d,"skipped":["move_to_column：目标列已经不存在"]}`, r.ID)
	if len(fired) != 1 || fired[0].Payload != want {
		t.Fatalf("执行记录不对: %+v", fired)
	}
}
