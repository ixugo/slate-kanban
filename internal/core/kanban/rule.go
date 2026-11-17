package kanban

import (
	"strings"
)

const (
	// maxRulesPerBoard 每个看板最多几条自动化规则。每次卡片变动都要遍历启用的规则，
	// 不设上限会拖慢所有写操作。
	maxRulesPerBoard = 50
	// maxRuleActions 一条规则最多几个动作。
	maxRuleActions = 20
	// maxRuleNameLen 规则名称的最大字符数，和数据库列长度一致。
	maxRuleNameLen = 100
	// maxRuleCommentLen add_comment 动作里评论文本的最大字符数。自动评论应该是简短的提示，不需要像人写的评论那么长。
	maxRuleCommentLen = 2000
	// maxRuleDueDays set_due 动作最多把到期时间往后推几天。
	maxRuleDueDays = 365
	// ruleSortStep 相邻两条规则排序值的间距，和列、卡片的做法一致。
	ruleSortStep = 1024
)

// 规则的触发类型。
const (
	// TriggerCardCreated 创建卡片时触发，规则的列表示卡片创建在哪一列。
	TriggerCardCreated = "card_created"
	// TriggerCardEntered 卡片进入某一列时触发，创建卡片也算进入初始列。
	TriggerCardEntered = "card_entered"
	// TriggerCardLeft 卡片离开某一列时触发。
	TriggerCardLeft = "card_left"
)

// 规则的动作类型。
const (
	ActionSetStartedAt    = "set_started_at"
	ActionSetFinishedAt   = "set_finished_at"
	ActionClearStartedAt  = "clear_started_at"
	ActionClearFinishedAt = "clear_finished_at"
	ActionSetDue          = "set_due"
	ActionClearDue        = "clear_due"
	ActionSetPriority     = "set_priority"
	ActionAddTag          = "add_tag"
	ActionRemoveTag       = "remove_tag"
	ActionSetHumanOnly    = "set_human_only"
	ActionAddComment      = "add_comment"
	ActionMoveToColumn    = "move_to_column"
)

// 写时间类动作的模式。
const (
	// ModeIfEmpty 只在时间为空时写入，不覆盖人手动修正过的值。这是默认模式。
	ModeIfEmpty = "if_empty"
	// ModeAlways 无论原来有没有值都覆盖。
	ModeAlways = "always"
)

// normalizeRule 整理并校验一条规则：名称和条件去掉首尾空白，动作只保留各自用到的字段，
// 然后检查触发类型、列归属、条件语法和每个动作的参数。cols 是规则所在看板的全部列。
// 保存前必须调用它，这样配置写错的规则在保存时就被拒绝，不会保存了才发现永远不触发。
func normalizeRule(r *Rule, cols []*Column) error {
	r.Name = strings.TrimSpace(r.Name)
	if err := checkLine("规则名称", r.Name, 1, maxRuleNameLen); err != nil {
		return err
	}
	switch r.TriggerType {
	case TriggerCardCreated, TriggerCardEntered, TriggerCardLeft:
	default:
		return invalidf("触发类型 %q 不认识，可选值为 %s、%s、%s", r.TriggerType, TriggerCardCreated, TriggerCardEntered, TriggerCardLeft)
	}
	if r.ColumnID != 0 && columnByID(cols, r.ColumnID) == nil {
		return invalidf("规则指定的列不存在，或者不属于这个看板")
	}
	r.Condition = strings.TrimSpace(r.Condition)
	if _, err := parseFilter(r.Condition); err != nil {
		return invalidf("规则的条件不合法：%s", err.Error())
	}
	return normalizeActions(r, cols)
}

// normalizeActions 校验动作列表的数量，并把每个动作整理成只含自己用到的字段的样子。
func normalizeActions(r *Rule, cols []*Column) error {
	if len(r.Actions) == 0 {
		return invalidf("规则至少要有一个动作")
	}
	if len(r.Actions) > maxRuleActions {
		return invalidf("一条规则最多 %d 个动作", maxRuleActions)
	}
	out := make([]RuleAction, len(r.Actions))
	for i, a := range r.Actions {
		norm, err := normalizeAction(a, cols)
		if err != nil {
			return invalidf("第 %d 个动作不合法：%s", i+1, err.Error())
		}
		out[i] = norm
	}
	r.Actions = out
	return nil
}

// normalizeAction 校验一个动作的类型和参数，返回只保留这种动作用到的字段的副本。
// 没有写模式的写时间动作按默认模式 if_empty 处理。
func normalizeAction(a RuleAction, cols []*Column) (RuleAction, error) {
	switch a.Type {
	case ActionSetStartedAt, ActionSetFinishedAt:
		return normalizeTimeAction(a)
	case ActionClearStartedAt, ActionClearFinishedAt, ActionClearDue:
		return RuleAction{Type: a.Type}, nil
	case ActionSetDue:
		if a.Days < 1 || a.Days > maxRuleDueDays {
			return RuleAction{}, invalidf("set_due 的 days 必须在 1 到 %d 之间", maxRuleDueDays)
		}
		return RuleAction{Type: a.Type, Days: a.Days}, nil
	case ActionSetPriority:
		if a.Priority < 0 || a.Priority > maxCardPriority {
			return RuleAction{}, invalidf("set_priority 的 priority 必须在 0 到 %d 之间", maxCardPriority)
		}
		return RuleAction{Type: a.Type, Priority: a.Priority}, nil
	case ActionAddTag, ActionRemoveTag:
		tag := strings.TrimSpace(a.Tag)
		if err := checkLine("标签", tag, 1, maxCardTagLen); err != nil {
			return RuleAction{}, err
		}
		return RuleAction{Type: a.Type, Tag: tag}, nil
	case ActionSetHumanOnly:
		return RuleAction{Type: a.Type, Flag: a.Flag}, nil
	case ActionAddComment:
		text := strings.TrimSpace(a.Text)
		if err := checkText("评论内容", text, 1, maxRuleCommentLen); err != nil {
			return RuleAction{}, err
		}
		return RuleAction{Type: a.Type, Text: text}, nil
	case ActionMoveToColumn:
		if columnByID(cols, a.ColumnID) == nil {
			return RuleAction{}, invalidf("move_to_column 的目标列不存在，或者不属于这个看板")
		}
		return RuleAction{Type: a.Type, ColumnID: a.ColumnID}, nil
	}
	return RuleAction{}, invalidf("动作类型 %q 不认识", a.Type)
}

// normalizeTimeAction 校验写时间动作的模式，没写按 if_empty。
func normalizeTimeAction(a RuleAction) (RuleAction, error) {
	mode := a.Mode
	if mode == "" {
		mode = ModeIfEmpty
	}
	if mode != ModeIfEmpty && mode != ModeAlways {
		return RuleAction{}, invalidf("%s 的 mode 只能是 %s 或 %s", a.Type, ModeIfEmpty, ModeAlways)
	}
	return RuleAction{Type: a.Type, Mode: mode}, nil
}
