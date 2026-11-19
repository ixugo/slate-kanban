package kanban_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"slate-kanban/internal/core/kanban"
)

// handSetTime 是测试里"人手动填的时间"，比固定的 claimNow 早。
const handSetTime = "2026-11-01T09:30:00Z"

// enterRule 创建一条"卡片进入某列"的规则，column 是列的主键，0 表示任意列。
func (e *cardEnv) enterRule(t *testing.T, name string, column int64, actions ...kanban.RuleAction) *kanban.Rule {
	t.Helper()
	return e.mustCreateRule(t, kanban.CreateRuleInput{
		Name: name, Trigger: kanban.TriggerCardEntered, ColumnID: column, Actions: actions,
	})
}

// moveTo 以 actor 的身份把卡片移到第 column 列的末尾，失败就终止测试。
func (e *cardEnv) moveTo(t *testing.T, actor kanban.Actor, id int64, column int) *kanban.Card {
	t.Helper()
	c, err := e.core.MoveCard(context.Background(), actor, id, kanban.MoveCardInput{ColumnID: e.col(column)})
	if err != nil {
		t.Fatalf("移动卡片 %d 失败: %v", id, err)
	}
	return c
}

// fired 返回看板里全部 automation.fired 事件，按发生顺序。
func (e *cardEnv) fired(t *testing.T) []*kanban.Event {
	t.Helper()
	var out []*kanban.Event
	for _, ev := range e.events(t) {
		if ev.Type == kanban.EventAutomationFired {
			out = append(out, ev)
		}
	}
	return out
}

// TestRuleEnterColumnWritesStartTime 验证最典型的用法：卡片进入进行中，规则写开始时间。
// 卡片只保存一次（版本只加一），事件顺序是先 card.moved 再 automation.fired，
// 并且 automation.fired 的操作者是 system、名字带规则名。
func TestRuleEnterColumnWritesStartTime(t *testing.T) {
	e := newClaimEnv(t)
	rule := e.enterRule(t, "开始计时", e.col(colDoing), kanban.RuleAction{Type: kanban.ActionSetStartedAt})
	c := e.card(t, colTodo, "A")
	events := len(e.events(t))

	moved := e.moveTo(t, testActor, c.ID, colDoing)
	if moved.StartedAt == nil || !moved.StartedAt.Equal(claimNow) || moved.Version != 2 {
		t.Fatalf("应该写入开始时间且版本只加一次: %+v", moved)
	}
	if stored := e.stored(t, c.ID); stored.StartedAt == nil || !stored.StartedAt.Equal(claimNow) || stored.Version != 2 {
		t.Fatalf("库里的卡片不对: %+v", stored)
	}
	if got, want := e.eventTypes(t, events), []string{kanban.EventCardMoved, kanban.EventAutomationFired}; !slices.Equal(got, want) {
		t.Fatalf("事件顺序应该是 %v，实际 %v", want, got)
	}
	ev := e.lastEvent(t)
	want := fmt.Sprintf(`{"actions":["set_started_at"],"key":"TST-1","rule":"开始计时","rule_id":%d,"skipped":[]}`, rule.ID)
	if ev.ActorKind != "system" || ev.Actor != "automation:开始计时" || ev.CardID != c.ID || ev.Payload != want {
		t.Fatalf("automation.fired 不对: %+v", ev)
	}
}

// TestRuleIfEmptyKeepsManualTime 验证默认模式不覆盖人手动填的时间，
// 并且规则没有产生任何效果时不写 automation.fired，也不多涨版本。
func TestRuleIfEmptyKeepsManualTime(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	e.enterRule(t, "开始计时", e.col(colDoing), kanban.RuleAction{Type: kanban.ActionSetStartedAt, Mode: kanban.ModeIfEmpty})
	c := e.card(t, colTodo, "A")
	if _, err := e.core.UpdateCard(ctx, testActor, c.ID, kanban.UpdateCardInput{StartedAt: strPtr(handSetTime)}); err != nil {
		t.Fatalf("手动填开始时间失败: %v", err)
	}

	moved := e.moveTo(t, testActor, c.ID, colDoing)
	want, _ := time.Parse(time.RFC3339, handSetTime)
	if moved.StartedAt == nil || !moved.StartedAt.Equal(want) {
		t.Fatalf("手动填的开始时间不应该被覆盖: %v", moved.StartedAt)
	}
	if moved.Version != 3 {
		t.Fatalf("版本应该是创建、改时间、移动各加一次，实际 %d", moved.Version)
	}
	if got := e.fired(t); len(got) != 0 {
		t.Fatalf("规则没有任何效果时不应该写事件: %+v", got)
	}
}

// TestRuleAlwaysOverwritesTime 验证 always 模式会覆盖已有的时间。
func TestRuleAlwaysOverwritesTime(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	e.enterRule(t, "重新计时", e.col(colDoing), kanban.RuleAction{Type: kanban.ActionSetStartedAt, Mode: kanban.ModeAlways})
	c := e.card(t, colTodo, "A")
	if _, err := e.core.UpdateCard(ctx, testActor, c.ID, kanban.UpdateCardInput{StartedAt: strPtr(handSetTime)}); err != nil {
		t.Fatalf("手动填开始时间失败: %v", err)
	}
	if moved := e.moveTo(t, testActor, c.ID, colDoing); moved.StartedAt == nil || !moved.StartedAt.Equal(claimNow) {
		t.Fatalf("always 应该覆盖成现在: %v", moved.StartedAt)
	}
}

// TestRuleLeaveColumnClearsFinishedTime 验证"离开某列"的触发：卡片离开完成列，规则清空完成时间。
func TestRuleLeaveColumnClearsFinishedTime(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	e.mustCreateRule(t, kanban.CreateRuleInput{
		Name: "重新打开", Trigger: kanban.TriggerCardLeft, ColumnID: e.col(colDone),
		Actions: []kanban.RuleAction{{Type: kanban.ActionClearFinishedAt}},
	})
	c := e.card(t, colDone, "A")
	if _, err := e.core.UpdateCard(ctx, testActor, c.ID, kanban.UpdateCardInput{FinishedAt: strPtr(handSetTime)}); err != nil {
		t.Fatalf("手动填完成时间失败: %v", err)
	}

	if moved := e.moveTo(t, testActor, c.ID, colDoing); moved.FinishedAt != nil {
		t.Fatalf("离开完成列应该清空完成时间: %v", moved.FinishedAt)
	}
	if got := e.fired(t); len(got) != 1 || got[0].Actor != "automation:重新打开" {
		t.Fatalf("应该有一条重新打开的 automation.fired: %+v", got)
	}
}

// TestRuleColumnBinding 验证规则绑定的列：只在进入指定的列时执行，列为 0 的规则进入任何列都执行。
func TestRuleColumnBinding(t *testing.T) {
	e := newClaimEnv(t)
	e.enterRule(t, "只管审查", e.col(colReview), kanban.RuleAction{Type: kanban.ActionAddTag, Tag: "审查中"})
	e.enterRule(t, "任意列", 0, kanban.RuleAction{Type: kanban.ActionSetPriority, Priority: 2})
	c := e.card(t, colTodo, "A")

	doing := e.moveTo(t, testActor, c.ID, colDoing)
	if len(doing.TagIDs) != 0 || doing.Priority != 2 {
		t.Fatalf("进入进行中只应该触发任意列的规则: %+v", doing)
	}
	review := e.moveTo(t, testActor, c.ID, colReview)
	if !slices.Equal(e.names(t, review), []string{"审查中"}) {
		t.Fatalf("进入审查应该触发绑定审查的规则: %+v", review)
	}
}

// TestRuleConditionUsesPostTriggerState 验证条件匹配的是触发之后的卡片状态，包括前面的规则刚改的：
// 第一条规则加标签，第二条规则的条件要求有这个标签，所以第二条也会执行；条件不满足的规则不执行。
func TestRuleConditionUsesPostTriggerState(t *testing.T) {
	e := newClaimEnv(t)
	e.enterRule(t, "先加标签", e.col(colDoing), kanban.RuleAction{Type: kanban.ActionAddTag, Tag: "后端"})
	e.mustCreateRule(t, kanban.CreateRuleInput{
		Name: "后端提高优先级", Trigger: kanban.TriggerCardEntered, ColumnID: e.col(colDoing), Condition: "tag:后端",
		Actions: []kanban.RuleAction{{Type: kanban.ActionSetPriority, Priority: 3}},
	})
	e.mustCreateRule(t, kanban.CreateRuleInput{
		Name: "前端才执行", Trigger: kanban.TriggerCardEntered, ColumnID: e.col(colDoing), Condition: "tag:前端",
		Actions: []kanban.RuleAction{{Type: kanban.ActionSetDue, Days: 7}},
	})
	c := e.card(t, colTodo, "A")

	moved := e.moveTo(t, testActor, c.ID, colDoing)
	if moved.Priority != 3 || moved.DueAt != nil {
		t.Fatalf("第二条应该执行、第三条不应该执行: %+v", moved)
	}
	if names := firedRuleNames(e.fired(t)); !slices.Equal(names, []string{"先加标签", "后端提高优先级"}) {
		t.Fatalf("执行过的规则应该是前两条，实际 %v", names)
	}
}

// firedRuleNames 返回 automation.fired 事件对应的规则名，按发生顺序。
func firedRuleNames(events []*kanban.Event) []string {
	var names []string
	for _, ev := range events {
		names = append(names, strings.TrimPrefix(ev.Actor, "automation:"))
	}
	return names
}

// TestRuleDisabledNotFired 验证停用的规则不执行，重新启用后恢复执行。
func TestRuleDisabledNotFired(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	off := false
	r := e.mustCreateRule(t, kanban.CreateRuleInput{
		Name: "停用的", Enabled: &off, Trigger: kanban.TriggerCardEntered,
		Actions: []kanban.RuleAction{{Type: kanban.ActionSetPriority, Priority: 1}},
	})
	a := e.card(t, colTodo, "A")
	if moved := e.moveTo(t, testActor, a.ID, colDoing); moved.Priority != 0 {
		t.Fatalf("停用的规则不应该执行: %+v", moved)
	}
	on := true
	if _, err := e.core.UpdateRule(ctx, testActor, r.ID, kanban.UpdateRuleInput{Enabled: &on}); err != nil {
		t.Fatalf("启用失败: %v", err)
	}
	if moved := e.moveTo(t, testActor, a.ID, colReview); moved.Priority != 1 {
		t.Fatalf("启用后应该执行: %+v", moved)
	}
}

// TestRuleOrderAndSingleSave 验证多条规则按顺序执行、后面的覆盖前面的，
// 并且不管命中几条，卡片只保存一次，版本只加一。
func TestRuleOrderAndSingleSave(t *testing.T) {
	e := newClaimEnv(t)
	e.enterRule(t, "先设低", e.col(colDoing), kanban.RuleAction{Type: kanban.ActionSetPriority, Priority: 1})
	e.enterRule(t, "后设高", e.col(colDoing),
		kanban.RuleAction{Type: kanban.ActionSetPriority, Priority: 3}, kanban.RuleAction{Type: kanban.ActionAddTag, Tag: "急"})
	c := e.card(t, colTodo, "A")

	moved := e.moveTo(t, testActor, c.ID, colDoing)
	if moved.Priority != 3 || !slices.Equal(e.names(t, moved), []string{"急"}) || moved.Version != 2 {
		t.Fatalf("后面的规则应该覆盖前面的，且版本只加一次: %+v", moved)
	}
	if names := firedRuleNames(e.fired(t)); !slices.Equal(names, []string{"先设低", "后设高"}) {
		t.Fatalf("执行顺序不对: %v", names)
	}
}

// TestRuleCreateCardTriggers 验证创建卡片同时触发 card_created（按创建所在的列）和进入初始列的 card_entered，
// 事件顺序是先 card.created 再 automation.fired；规则改了卡片字段时卡片是第二版，没命中规则就是第一版。
func TestRuleCreateCardTriggers(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	e.mustCreateRule(t, kanban.CreateRuleInput{
		Name: "新建在待办", Trigger: kanban.TriggerCardCreated, ColumnID: e.col(colTodo),
		Actions: []kanban.RuleAction{{Type: kanban.ActionAddTag, Tag: "新建"}},
	})
	e.enterRule(t, "进入任意列", 0, kanban.RuleAction{Type: kanban.ActionSetPriority, Priority: 2})
	events := len(e.events(t))

	c := e.card(t, colTodo, "在待办创建")
	if !slices.Equal(e.names(t, c), []string{"新建"}) || c.Priority != 2 || c.Version != 2 {
		t.Fatalf("两条规则都应该执行，卡片是第二版: %+v", c)
	}
	// 规则加的"新建"在标签库里还没有，会先新建标签并写一条标签库事件
	want := []string{kanban.EventCardCreated, kanban.EventBoardLabelsUpdated, kanban.EventAutomationFired, kanban.EventAutomationFired}
	if got := e.eventTypes(t, events); !slices.Equal(got, want) {
		t.Fatalf("事件顺序应该是 %v，实际 %v", want, got)
	}
	idea := e.card(t, colIdea, "在想法池创建")
	if len(idea.TagIDs) != 0 || idea.Priority != 2 {
		t.Fatalf("创建在想法池只应该触发进入任意列的规则: %+v", idea)
	}

	agentCard, err := e.core.CreateCard(ctx, testAgent, e.board.ID, kanban.CreateCardInput{Title: "agent 创建", ColumnID: e.col(colTodo)})
	if err != nil || !slices.Equal(e.names(t, agentCard), []string{"新建"}) {
		t.Fatalf("agent 创建的卡片同样触发规则: %+v %v", agentCard, err)
	}
}

// TestRuleCreateWithoutMatchStaysFirstVersion 验证没有规则命中时，新卡片仍然是第一版，没有多余的保存和事件。
func TestRuleCreateWithoutMatchStaysFirstVersion(t *testing.T) {
	e := newClaimEnv(t)
	e.enterRule(t, "只管审查", e.col(colReview), kanban.RuleAction{Type: kanban.ActionAddTag, Tag: "x"})
	c := e.card(t, colTodo, "A")
	if c.Version != 1 || len(e.fired(t)) != 0 {
		t.Fatalf("没有规则命中时卡片应该是第一版且没有 automation.fired: %+v", c)
	}
}

// TestRuleAddComment 验证评论动作：以 system 身份写评论，作者是 automation 加规则名，
// 评论不动卡片本身，所以版本只因为移动涨一次；事件顺序是移动、评论、规则执行。
func TestRuleAddComment(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	e.enterRule(t, "自动提示", e.col(colDoing), kanban.RuleAction{Type: kanban.ActionAddComment, Text: "已自动开始，请先读描述"})
	c := e.card(t, colTodo, "A")
	events := len(e.events(t))

	moved := e.moveTo(t, testActor, c.ID, colDoing)
	if moved.Version != 2 {
		t.Fatalf("评论不应该让卡片多涨版本: %d", moved.Version)
	}
	d, err := e.core.GetCardDetail(ctx, c.ID)
	if err != nil || len(d.Comments) != 1 {
		t.Fatalf("应该有一条自动评论: %+v %v", d, err)
	}
	if cm := d.Comments[0]; cm.AuthorKind != "system" || cm.Author != "automation:自动提示" || cm.Body != "已自动开始，请先读描述" {
		t.Fatalf("评论内容不对: %+v", cm)
	}
	want := []string{kanban.EventCardMoved, kanban.EventCommentAdded, kanban.EventAutomationFired}
	if got := e.eventTypes(t, events); !slices.Equal(got, want) {
		t.Fatalf("事件顺序应该是 %v，实际 %v", want, got)
	}
}

// TestRuleSkippedActionsAreReported 验证参数原因无法执行的动作不会让用户的操作失败，
// 其余动作照常生效，跳过的原因写在 automation.fired 的 skipped 里。
func TestRuleSkippedActionsAreReported(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	e.enterRule(t, "加标签并提优先级", e.col(colDoing),
		kanban.RuleAction{Type: kanban.ActionAddTag, Tag: "x"}, kanban.RuleAction{Type: kanban.ActionSetPriority, Priority: 2})
	c := e.card(t, colTodo, "A")
	tags := make([]string, 10)
	for i := range tags {
		tags[i] = fmt.Sprintf("t%d", i)
	}
	if _, err := e.core.UpdateCard(ctx, testActor, c.ID, kanban.UpdateCardInput{Tags: &tags}); err != nil {
		t.Fatalf("准备 10 个标签失败: %v", err)
	}

	moved := e.moveTo(t, testActor, c.ID, colDoing)
	if moved.ColumnID != e.col(colDoing) || moved.Priority != 2 || len(moved.TagIDs) != 10 {
		t.Fatalf("移动和能执行的动作应该照常生效: %+v", moved)
	}
	fired := e.fired(t)
	if len(fired) != 1 || !strings.Contains(fired[0].Payload, `"actions":["set_priority"]`) ||
		!strings.Contains(fired[0].Payload, `"skipped":["add_tag：`) {
		t.Fatalf("应该记录生效的动作和跳过的动作: %+v", fired)
	}
}

// TestRuleFiresOnFinishCard 验证 agent 完成交接换列时规则同样生效，卡片只保存一次，
// 事件里评论、移动在前，规则的执行在最后。
func TestRuleFiresOnFinishCard(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	e.enterRule(t, "进入审查", e.col(colReview), kanban.RuleAction{Type: kanban.ActionAddTag, Tag: "待审查"})
	c := e.card(t, colDoing, "任务")
	if _, err := e.core.ClaimCard(ctx, testAgent, c.ID, kanban.ClaimCardInput{}); err != nil {
		t.Fatalf("领取失败: %v", err)
	}
	events := len(e.events(t))
	before := e.stored(t, c.ID).Version

	d, err := e.core.FinishCard(ctx, testAgent, c.ID, kanban.FinishCardInput{Summary: "做完了"})
	if err != nil {
		t.Fatalf("完成交接失败: %v", err)
	}
	if d.ColumnID != e.col(colReview) || !slices.Equal(e.names(t, d.Card), []string{"待审查"}) || d.Version != before+1 {
		t.Fatalf("交接后应该在审查列、带标签，版本只加一次: %+v", d.Card)
	}
	want := []string{kanban.EventCommentAdded, kanban.EventCardMoved, kanban.EventBoardLabelsUpdated, kanban.EventAutomationFired}
	if got := e.eventTypes(t, events); !slices.Equal(got, want) {
		t.Fatalf("事件顺序应该是 %v，实际 %v", want, got)
	}
}

// TestRuleFiresOnAgentMove 验证规则不区分操作者：agent 移动自己持有的卡片换列，规则同样执行。
func TestRuleFiresOnAgentMove(t *testing.T) {
	e := newClaimEnv(t)
	e.enterRule(t, "进入审查", e.col(colReview), kanban.RuleAction{Type: kanban.ActionSetPriority, Priority: 3})
	c := e.card(t, colDoing, "任务")
	e.hold(t, c.ID, "claude")
	if moved := e.moveTo(t, testAgent, c.ID, colReview); moved.Priority != 3 {
		t.Fatalf("agent 的移动也应该触发规则: %+v", moved)
	}
}

// TestReorderDoesNotTriggerRules 验证在同一列里调整顺序不算进入或离开，不触发规则。
func TestReorderDoesNotTriggerRules(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	a := e.card(t, colTodo, "A")
	b := e.card(t, colTodo, "B")
	e.enterRule(t, "任意进入", 0, kanban.RuleAction{Type: kanban.ActionSetPriority, Priority: 3})

	if _, err := e.core.MoveCard(ctx, testActor, b.ID, kanban.MoveCardInput{BeforeID: a.ID}); err != nil {
		t.Fatalf("同列调整顺序失败: %v", err)
	}
	if got := e.stored(t, b.ID); got.Priority != 0 || len(e.fired(t)) != 0 {
		t.Fatalf("同列调整顺序不应该触发规则: %+v", got)
	}
}

// TestRuleWithBrokenConditionIsIgnored 验证库里被改坏、解析不了条件的规则只是被忽略，用户的操作照常成功。
func TestRuleWithBrokenConditionIsIgnored(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	r := e.enterRule(t, "会被改坏", 0, kanban.RuleAction{Type: kanban.ActionSetPriority, Priority: 3})
	r.Condition = "nosuchterm:x"
	if err := e.db.Rule().Save(ctx, r); err != nil {
		t.Fatalf("写入被改坏的规则失败: %v", err)
	}
	c := e.card(t, colTodo, "A")
	if moved := e.moveTo(t, testActor, c.ID, colDoing); moved.ColumnID != e.col(colDoing) || moved.Priority != 0 {
		t.Fatalf("坏规则应该被忽略，移动照常成功: %+v", moved)
	}
}
