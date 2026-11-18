package kanban

import (
	"testing"
	"time"
)

// actionNow 是动作测试里固定的"现在"。
var actionNow = time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)

// earlier 是一个比 actionNow 早的时间，表示"已经有值"。
var earlier = time.Date(2026, 11, 1, 9, 30, 0, 0, time.UTC)

// timeAt 返回时间的指针，方便给卡片的时间字段赋值。
func timeAt(t time.Time) *time.Time { return &t }

// TestApplyTimeActions 验证写时间和清时间类动作：if_empty（以及没写模式）只在为空时写，always 会覆盖，
// 清空只在有值时才算变化。
func TestApplyTimeActions(t *testing.T) {
	cases := []struct {
		name        string
		action      RuleAction
		startedAt   *time.Time
		wantChanged bool
		wantStarted *time.Time
	}{
		{"if_empty 且为空，写入", RuleAction{Type: ActionSetStartedAt, Mode: ModeIfEmpty}, nil, true, timeAt(actionNow)},
		{"没写模式按 if_empty，已有值不动", RuleAction{Type: ActionSetStartedAt}, timeAt(earlier), false, timeAt(earlier)},
		{"if_empty 已有值，不动", RuleAction{Type: ActionSetStartedAt, Mode: ModeIfEmpty}, timeAt(earlier), false, timeAt(earlier)},
		{"always 已有值，覆盖", RuleAction{Type: ActionSetStartedAt, Mode: ModeAlways}, timeAt(earlier), true, timeAt(actionNow)},
		{"清空有值的时间", RuleAction{Type: ActionClearStartedAt}, timeAt(earlier), true, nil},
		{"清空本来就空的时间，不算变化", RuleAction{Type: ActionClearStartedAt}, nil, false, nil},
	}
	for _, tc := range cases {
		card := &Card{StartedAt: tc.startedAt}
		changed, skip := applyFieldAction(card, tc.action, actionNow)
		if changed != tc.wantChanged || skip != "" {
			t.Errorf("%s：changed=%v skip=%q，期望 changed=%v", tc.name, changed, skip, tc.wantChanged)
		}
		if !sameTime(card.StartedAt, tc.wantStarted) {
			t.Errorf("%s：开始时间 %v，期望 %v", tc.name, card.StartedAt, tc.wantStarted)
		}
	}
}

// TestApplyFinishedAndDue 验证完成时间和到期时间的动作：完成时间和开始时间对称，
// 到期时间是现在加上指定天数，清空只在有值时才算变化。
func TestApplyFinishedAndDue(t *testing.T) {
	card := &Card{}
	if changed, _ := applyFieldAction(card, RuleAction{Type: ActionSetFinishedAt, Mode: ModeIfEmpty}, actionNow); !changed || !sameTime(card.FinishedAt, timeAt(actionNow)) {
		t.Fatalf("应该写入完成时间: %v", card.FinishedAt)
	}
	if changed, _ := applyFieldAction(card, RuleAction{Type: ActionClearFinishedAt}, actionNow); !changed || card.FinishedAt != nil {
		t.Fatalf("应该清空完成时间: %v", card.FinishedAt)
	}
	card.DueAt = timeAt(earlier)
	if changed, _ := applyFieldAction(card, RuleAction{Type: ActionSetDue, Days: 3}, actionNow); !changed || !sameTime(card.DueAt, timeAt(actionNow.Add(72*time.Hour))) {
		t.Fatalf("到期时间应该是现在加 3 天，并覆盖原来的值: %v", card.DueAt)
	}
	if changed, _ := applyFieldAction(card, RuleAction{Type: ActionClearDue}, actionNow); !changed || card.DueAt != nil {
		t.Fatalf("应该清空到期时间: %v", card.DueAt)
	}
	if changed, _ := applyFieldAction(card, RuleAction{Type: ActionClearDue}, actionNow); changed {
		t.Fatalf("本来就没有到期时间，不算变化")
	}
}

// TestApplyPriorityAndHumanOnly 验证优先级和仅限人处理：值不同才算变化，值相同不算。
func TestApplyPriorityAndHumanOnly(t *testing.T) {
	card := &Card{Priority: 1}
	if changed, _ := applyFieldAction(card, RuleAction{Type: ActionSetPriority, Priority: 3}, actionNow); !changed || card.Priority != 3 {
		t.Fatalf("优先级应该变成 3: %d", card.Priority)
	}
	if changed, _ := applyFieldAction(card, RuleAction{Type: ActionSetPriority, Priority: 3}, actionNow); changed {
		t.Fatalf("优先级没变，不应该算变化")
	}
	if changed, _ := applyFieldAction(card, RuleAction{Type: ActionSetPriority, Priority: 0}, actionNow); !changed || card.Priority != 0 {
		t.Fatalf("优先级 0 表示清除，应该生效: %d", card.Priority)
	}
	if changed, _ := applyFieldAction(card, RuleAction{Type: ActionSetHumanOnly, Flag: true}, actionNow); !changed || !card.HumanOnly {
		t.Fatalf("应该变成仅限人处理")
	}
	if changed, _ := applyFieldAction(card, RuleAction{Type: ActionSetHumanOnly, Flag: true}, actionNow); changed {
		t.Fatalf("没变，不应该算变化")
	}
	if changed, _ := applyFieldAction(card, RuleAction{Type: ActionSetHumanOnly, Flag: false}, actionNow); !changed || card.HumanOnly {
		t.Fatalf("应该取消仅限人处理")
	}
}

// TestApplyUnknownAction 验证不认识的动作类型被跳过并说明原因，卡片不变。
func TestApplyUnknownAction(t *testing.T) {
	card := &Card{Priority: 2}
	changed, skip := applyFieldAction(card, RuleAction{Type: "explode"}, actionNow)
	if changed || skip == "" || card.Priority != 2 {
		t.Fatalf("不认识的动作应该被跳过: changed=%v skip=%q", changed, skip)
	}
}
