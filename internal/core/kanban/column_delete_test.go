package kanban_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// columnIDs 返回看板当前所有列的主键，按从左到右的顺序。
func (e *cardEnv) columnIDs(t *testing.T) []int64 {
	t.Helper()
	d, err := e.core.GetBoard(context.Background(), e.board.ID)
	if err != nil {
		t.Fatalf("读取看板失败: %v", err)
	}
	ids := make([]int64, len(d.Columns))
	for i, c := range d.Columns {
		ids[i] = c.ID
	}
	return ids
}

// TestDeleteEmptyColumn 验证删除空列：列消失，别的列指向它的"下一列"被清掉，
// 追加 column.deleted 事件，内容里搬走的张数是 0。
func TestDeleteEmptyColumn(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	idea, todo := e.col(colIdea), e.col(colTodo)
	if got := e.board.Columns[colIdea].NextColumnID; got != todo {
		t.Fatalf("前提不成立：收件箱的下一列应该是待办，实际 %d", got)
	}

	if err := e.core.DeleteColumn(ctx, testActor, e.board.ID, todo, 0); err != nil {
		t.Fatalf("删除空列失败: %v", err)
	}
	if slices.Contains(e.columnIDs(t), todo) {
		t.Fatalf("列应该被删除")
	}
	stored, err := e.db.Column().GetByID(ctx, idea)
	if err != nil || stored.NextColumnID != 0 {
		t.Fatalf("指向被删列的下一列应该被清掉: %+v %v", stored, err)
	}
	want := fmt.Sprintf(`{"column_id":%d,"moved_cards":0,"moved_to_id":0,"name":"待办"}`, todo)
	if ev := e.lastEvent(t); ev.Type != kanban.EventColumnDeleted || ev.Payload != want {
		t.Fatalf("事件不对: %+v", ev)
	}
}

// TestDeleteColumnWithCardsNeedsTarget 验证列里有卡片又没有指定移入的列时返回 409，什么都不改。
func TestDeleteColumnWithCardsNeedsTarget(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	c := e.card(t, colTodo, "A")
	events := len(e.events(t))

	err := e.core.DeleteColumn(ctx, testActor, e.board.ID, e.col(colTodo), 0)
	if !errors.Is(err, kanban.ErrColumnNotEmpty) {
		t.Fatalf("应该返回 ErrColumnNotEmpty: %v", err)
	}
	if got := e.stored(t, c.ID); got.ColumnID != e.col(colTodo) || got.Version != 1 {
		t.Fatalf("卡片不应该被动: %+v", got)
	}
	if !slices.Contains(e.columnIDs(t), e.col(colTodo)) || len(e.events(t)) != events {
		t.Fatalf("列和事件都不应该变化")
	}
}

// TestDeleteColumnMovesCards 验证删除有卡片的列：卡片按原顺序追加到目标列末尾，版本各加一，
// 人删列搬走的卡片保留 agent 的领取、不写 card.released，只写一条 column.deleted（不为每张卡片写 card.moved），
// 并且不触发自动化规则。
func TestDeleteColumnMovesCards(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	existing := e.card(t, colReview, "审查里原有的")
	first := e.card(t, colDoing, "先")
	second := e.card(t, colDoing, "后")
	e.hold(t, second.ID, "claude")
	e.enterRule(t, "进入审查提优先级", e.col(colReview), kanban.RuleAction{Type: kanban.ActionSetPriority, Priority: 3})
	events := len(e.events(t))

	if err := e.core.DeleteColumn(ctx, testActor, e.board.ID, e.col(colDoing), e.col(colReview)); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if got := e.titles(t, colReview); got != "审查里原有的,先,后" {
		t.Fatalf("应该按原顺序追加到目标列末尾，实际 %s", got)
	}
	for _, c := range []*kanban.Card{first, second} {
		got := e.stored(t, c.ID)
		wantClaim := ""
		if c.ID == second.ID {
			wantClaim = "claude"
		}
		if got.ColumnID != e.col(colReview) || got.Version != 2 || got.Priority != 0 || got.ClaimedBy != wantClaim {
			t.Fatalf("搬过去的卡片不对（版本加一、不触发规则、领取原样保留）: %+v", got)
		}
	}
	if got := e.stored(t, existing.ID); got.Version != 1 || got.Priority != 0 {
		t.Fatalf("目标列原有的卡片不应该变化: %+v", got)
	}
	want := []string{kanban.EventColumnDeleted}
	if got := e.eventTypes(t, events); !slices.Equal(got, want) {
		t.Fatalf("事件应该是 %v，实际 %v", want, got)
	}
	payload := fmt.Sprintf(`{"column_id":%d,"moved_cards":2,"moved_to_id":%d,"name":"进行中"}`, e.col(colDoing), e.col(colReview))
	if ev := e.lastEvent(t); ev.Payload != payload {
		t.Fatalf("column.deleted 内容不对: %s", ev.Payload)
	}
}

// TestDeleteColumnInvalidTarget 验证移入的列不存在、是别的看板的列、或者就是要删的列时返回 400，什么都不改。
func TestDeleteColumnInvalidTarget(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	other := mustCreateBoard(t, e.core, kanban.CreateBoardInput{Name: "别的板", Slug: "OTH", Locale: "zh-CN"})
	c := e.card(t, colTodo, "A")

	for name, target := range map[string]int64{
		"不存在": 9999, "别的看板的列": other.Columns[0].ID, "要删的列本身": e.col(colTodo),
	} {
		err := e.core.DeleteColumn(ctx, testActor, e.board.ID, e.col(colTodo), target)
		if !errors.Is(err, kanban.ErrInvalidInput) {
			t.Errorf("%s：应该返回 ErrInvalidInput，实际 %v", name, err)
		}
	}
	if got := e.stored(t, c.ID); got.ColumnID != e.col(colTodo) || !slices.Contains(e.columnIDs(t), e.col(colTodo)) {
		t.Fatalf("被拒绝的删除不应该改动任何东西: %+v", got)
	}
}

// TestDeleteColumnDisablesRules 验证删列时停用引用它的启用中的规则：触发所在的列、移动列动作的目标列都算引用，
// 每条追加 rule.updated 事件，并且排在 column.deleted 之前；没引用的和本来就停用的规则不动。
func TestDeleteColumnDisablesRules(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	review := e.col(colReview)
	byTrigger := e.enterRule(t, "按触发列引用", review, kanban.RuleAction{Type: kanban.ActionSetPriority, Priority: 1})
	byAction := e.enterRule(t, "按目标列引用", e.col(colDoing), kanban.RuleAction{Type: kanban.ActionMoveToColumn, ColumnID: review})
	unrelated := e.enterRule(t, "无关的", e.col(colDoing), kanban.RuleAction{Type: kanban.ActionSetPriority, Priority: 2})
	off := false
	alreadyOff := e.mustCreateRule(t, kanban.CreateRuleInput{
		Name: "本来就停用", Enabled: &off, Trigger: kanban.TriggerCardEntered, ColumnID: review,
		Actions: []kanban.RuleAction{{Type: kanban.ActionSetPriority, Priority: 1}},
	})
	events := len(e.events(t))

	if err := e.core.DeleteColumn(ctx, testActor, e.board.ID, review, 0); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	for r, wantEnabled := range map[*kanban.Rule]bool{byTrigger: false, byAction: false, unrelated: true, alreadyOff: false} {
		if got := e.storedRule(t, r.ID); got.Enabled != wantEnabled {
			t.Errorf("规则 %s 的启用状态应该是 %v", r.Name, wantEnabled)
		}
	}
	want := []string{kanban.EventRuleUpdated, kanban.EventRuleUpdated, kanban.EventColumnDeleted}
	if got := e.eventTypes(t, events); !slices.Equal(got, want) {
		t.Fatalf("事件应该是 %v，实际 %v", want, got)
	}
}

// TestDeleteColumnIdempotentAndDenied 验证列已经不存在时返回成功且不写事件；
// agent、自动化、身份无效不能删，看板不存在返回 404。
func TestDeleteColumnIdempotentAndDenied(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	idea := e.col(colIdea)
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
		if err := e.core.DeleteColumn(ctx, tc.actor, tc.board, idea, 0); !errors.Is(err, tc.want) {
			t.Errorf("%s：期望 %v，实际 %v", name, tc.want, err)
		}
	}
	if !slices.Contains(e.columnIDs(t), idea) {
		t.Fatalf("被拒绝的删除不应该删列")
	}

	if err := e.core.DeleteColumn(ctx, testActor, e.board.ID, idea, 0); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	events := len(e.events(t))
	if err := e.core.DeleteColumn(ctx, testActor, e.board.ID, idea, 0); err != nil {
		t.Fatalf("重复删除应该成功: %v", err)
	}
	if len(e.events(t)) != events {
		t.Fatalf("重复删除不应该写事件")
	}
}
