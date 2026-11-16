package kanban

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// ruleCols 是规则校验测试用的列。
func ruleCols() []*Column {
	return []*Column{{ID: 10, Name: "待办"}, {ID: 20, Name: "进行中"}}
}

// validRule 返回一条可以通过校验的规则，各个用例在它的基础上改一处。
func validRule() *Rule {
	return &Rule{
		Name: "  开始计时 ", TriggerType: TriggerCardEntered, ColumnID: 20, Condition: " tag:backend ",
		Actions: []RuleAction{{Type: ActionSetStartedAt}},
	}
}

// TestNormalizeRuleAccepts 验证合法的规则：名称和条件去掉空白，写时间动作补上默认模式，
// 每个动作只保留自己用到的字段（别的字段是客户端多传的，直接丢掉）。
func TestNormalizeRuleAccepts(t *testing.T) {
	r := validRule()
	r.Actions = []RuleAction{
		{Type: ActionSetStartedAt, Tag: "多余"},
		{Type: ActionSetFinishedAt, Mode: ModeAlways},
		{Type: ActionClearStartedAt, Days: 3},
		{Type: ActionClearFinishedAt},
		{Type: ActionSetDue, Days: 7, Text: "多余"},
		{Type: ActionClearDue},
		{Type: ActionSetPriority, Priority: 0},
		{Type: ActionAddTag, Tag: " urgent "},
		{Type: ActionRemoveTag, Tag: "old"},
		{Type: ActionSetHumanOnly, Flag: true},
		{Type: ActionAddComment, Text: " 已进入审查 "},
		{Type: ActionMoveToColumn, ColumnID: 10, Days: 1},
	}
	if err := normalizeRule(r, ruleCols()); err != nil {
		t.Fatalf("合法的规则应该通过: %v", err)
	}
	if r.Name != "开始计时" || r.Condition != "tag:backend" {
		t.Fatalf("名称和条件应该去掉空白: %q %q", r.Name, r.Condition)
	}
	want := []RuleAction{
		{Type: ActionSetStartedAt, Mode: ModeIfEmpty},
		{Type: ActionSetFinishedAt, Mode: ModeAlways},
		{Type: ActionClearStartedAt},
		{Type: ActionClearFinishedAt},
		{Type: ActionSetDue, Days: 7},
		{Type: ActionClearDue},
		{Type: ActionSetPriority},
		{Type: ActionAddTag, Tag: "urgent"},
		{Type: ActionRemoveTag, Tag: "old"},
		{Type: ActionSetHumanOnly, Flag: true},
		{Type: ActionAddComment, Text: "已进入审查"},
		{Type: ActionMoveToColumn, ColumnID: 10},
	}
	if !slices.Equal(r.Actions, want) {
		t.Fatalf("动作整理的结果不对:\n实际 %+v\n期望 %+v", r.Actions, want)
	}
}

// TestNormalizeRuleRejects 验证每一处不合法都被拒绝，并且提示里能看出是哪一项。
func TestNormalizeRuleRejects(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Rule)
		must   string
	}{
		{"名称为空", func(r *Rule) { r.Name = "  " }, "规则名称"},
		{"名称过长", func(r *Rule) { r.Name = strings.Repeat("名", maxRuleNameLen+1) }, "规则名称"},
		{"名称含换行", func(r *Rule) { r.Name = "a\nb" }, "规则名称"},
		{"触发类型不认识", func(r *Rule) { r.TriggerType = "card_deleted" }, "card_deleted"},
		{"触发类型为空", func(r *Rule) { r.TriggerType = "" }, "触发类型"},
		{"列不存在", func(r *Rule) { r.ColumnID = 99 }, "列"},
		{"条件语法错", func(r *Rule) { r.Condition = "is:foo" }, "is:foo"},
		{"条件过长", func(r *Rule) { r.Condition = strings.Repeat("a", maxFilterLen+1) }, "条件"},
		{"没有动作", func(r *Rule) { r.Actions = nil }, "至少"},
		{"动作过多", func(r *Rule) { r.Actions = slices.Repeat([]RuleAction{{Type: ActionClearDue}}, maxRuleActions+1) }, "最多"},
		{"动作类型不认识", func(r *Rule) { r.Actions = []RuleAction{{Type: "explode"}} }, "explode"},
		{"模式不认识", func(r *Rule) { r.Actions = []RuleAction{{Type: ActionSetStartedAt, Mode: "never"}} }, "mode"},
		{"天数为零", func(r *Rule) { r.Actions = []RuleAction{{Type: ActionSetDue, Days: 0}} }, "days"},
		{"天数过大", func(r *Rule) { r.Actions = []RuleAction{{Type: ActionSetDue, Days: maxRuleDueDays + 1}} }, "days"},
		{"优先级越界", func(r *Rule) { r.Actions = []RuleAction{{Type: ActionSetPriority, Priority: 4}} }, "priority"},
		{"标签为空", func(r *Rule) { r.Actions = []RuleAction{{Type: ActionAddTag, Tag: " "}} }, "标签"},
		{"标签过长", func(r *Rule) {
			r.Actions = []RuleAction{{Type: ActionRemoveTag, Tag: strings.Repeat("t", maxCardTagLen+1)}}
		}, "标签"},
		{"评论为空", func(r *Rule) { r.Actions = []RuleAction{{Type: ActionAddComment}} }, "评论"},
		{"评论过长", func(r *Rule) {
			r.Actions = []RuleAction{{Type: ActionAddComment, Text: strings.Repeat("a", maxRuleCommentLen+1)}}
		}, "评论"},
		{"目标列不存在", func(r *Rule) { r.Actions = []RuleAction{{Type: ActionMoveToColumn, ColumnID: 99}} }, "目标列"},
		{"第二个动作不合法", func(r *Rule) { r.Actions = []RuleAction{{Type: ActionClearDue}, {Type: ActionSetDue}} }, "第 2 个动作"},
	}
	for _, c := range cases {
		r := validRule()
		c.mutate(r)
		err := normalizeRule(r, ruleCols())
		if !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s：应该得到 ErrInvalidInput，实际 %v", c.name, err)
			continue
		}
		if !strings.Contains(err.Error(), c.must) {
			t.Errorf("%s：提示里应该有 %q，实际 %q", c.name, c.must, err.Error())
		}
	}
}

// TestNormalizeRuleBoundaries 验证数量和长度刚好等于上限时可以通过；列为 0 表示任意列；条件可以为空。
func TestNormalizeRuleBoundaries(t *testing.T) {
	r := validRule()
	r.Name = strings.Repeat("名", maxRuleNameLen)
	r.ColumnID = 0
	r.Condition = ""
	r.Actions = slices.Repeat([]RuleAction{{Type: ActionClearDue}}, maxRuleActions)
	if err := normalizeRule(r, ruleCols()); err != nil {
		t.Fatalf("刚好等于上限应该通过: %v", err)
	}
	r.Actions = []RuleAction{{Type: ActionSetDue, Days: maxRuleDueDays}, {Type: ActionSetPriority, Priority: maxCardPriority}}
	if err := normalizeRule(r, ruleCols()); err != nil {
		t.Fatalf("天数和优先级刚好等于上限应该通过: %v", err)
	}
}
