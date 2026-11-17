package kanban

import (
	"context"
	"log/slog"
	"slices"
)

// ruleRun 是一次操作里规则引擎的运行状态。一次操作指创建一张卡片、移动一张卡片或完成一次交接。
// 这期间启用的规则只读一次，同一条规则最多执行一次，规则对卡片的改动都落在内存里的卡片上，
// 由调用方在最后统一保存，所以不管命中几条规则，卡片只写一次、版本只加一次。
type ruleRun struct {
	u       *unit
	card    *Card
	rules   []*Rule        // 启用的规则，第一次触发时才读，之后复用
	loaded  bool           // rules 是否已经读过
	ix      *boardIndex    // 判断条件用的小索引，第一次需要时才建
	fired   map[int64]bool // 本次操作里已经执行过的规则
	changed bool           // 规则是否改动了卡片本身
}

// newRuleRun 为一张卡片的一次操作创建规则引擎的运行状态。
func (u *unit) newRuleRun(card *Card) *ruleRun {
	return &ruleRun{u: u, card: card, fired: make(map[int64]bool)}
}

// fireCreateRules 在新卡片已经写库、card.created 事件已经追加之后触发创建类规则。
// 规则真的改了卡片字段才保存一次，所以创建出来的卡片只有在被规则改过时才是第二版。
func (u *unit) fireCreateRules(ctx context.Context, actor Actor, card *Card) error {
	run := u.newRuleRun(card)
	if err := run.created(ctx); err != nil {
		return err
	}
	if !run.changed {
		return nil
	}
	return u.saveCard(ctx, actor, card)
}

// created 触发创建卡片的规则：先是 card_created，再是进入初始列的 card_entered。
func (r *ruleRun) created(ctx context.Context) error {
	for _, kind := range []string{TriggerCardCreated, TriggerCardEntered} {
		if err := r.trigger(ctx, kind, r.card.ColumnID); err != nil {
			return err
		}
	}
	return nil
}

// moved 触发卡片换列的规则：先是离开原列的 card_left，再是进入新列的 card_entered。
func (r *ruleRun) moved(ctx context.Context, fromID, toID int64) error {
	if err := r.trigger(ctx, TriggerCardLeft, fromID); err != nil {
		return err
	}
	return r.trigger(ctx, TriggerCardEntered, toID)
}

// trigger 按规则的执行顺序，依次执行触发类型和列都匹配、本次操作里还没执行过、条件也满足的规则。
// 复杂度是规则数（最多 maxRulesPerBoard）乘以每条规则的动作数（最多 maxRuleActions）。
func (r *ruleRun) trigger(ctx context.Context, kind string, columnID int64) error {
	rules, err := r.enabledRules(ctx)
	if err != nil {
		return err
	}
	for _, rule := range rules {
		if !listensTo(rule, kind, columnID) || r.fired[rule.ID] {
			continue
		}
		ok, err := r.conditionHolds(ctx, rule)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		r.fired[rule.ID] = true
		if err := r.execute(ctx, rule); err != nil {
			return err
		}
	}
	return nil
}

// listensTo 判断规则是不是在等这个触发：类型一样，并且规则的列是 0（任意列）或者就是发生事件的这一列。
func listensTo(rule *Rule, kind string, columnID int64) bool {
	return rule.TriggerType == kind && (rule.ColumnID == 0 || rule.ColumnID == columnID)
}

// enabledRules 返回看板里启用的规则，按执行顺序。第一次调用时读库，之后复用。
func (r *ruleRun) enabledRules(ctx context.Context) ([]*Rule, error) {
	if r.loaded {
		return r.rules, nil
	}
	all, err := r.u.rule.ListByBoard(ctx, r.card.BoardID)
	if err != nil {
		return nil, wrapDB("读取规则", err)
	}
	r.rules = slices.DeleteFunc(all, func(rule *Rule) bool { return !rule.Enabled })
	r.loaded = true
	return r.rules, nil
}

// index 返回判断条件用的小索引：看板的列，加上这张卡片和它依赖的卡片。第一次需要时才建，
// 索引里放的就是内存里的卡片，所以条件看到的是触发之后、前面的规则改过的状态。
func (r *ruleRun) index(ctx context.Context) (*boardIndex, error) {
	if r.ix == nil {
		ix, err := r.u.claimIndex(ctx, r.card)
		if err != nil {
			return nil, err
		}
		r.ix = ix
	}
	return r.ix, nil
}

// conditionHolds 判断卡片现在是否满足规则的条件，没有条件就是满足。
// 保存时条件已经校验过，所以运行时解析失败只可能是数据被改坏，记一条警告并忽略这条规则，不让用户的操作失败。
func (r *ruleRun) conditionHolds(ctx context.Context, rule *Rule) (bool, error) {
	if rule.Condition == "" {
		return true, nil
	}
	f, err := parseFilter(rule.Condition)
	if err != nil {
		slog.WarnContext(ctx, "规则的条件无法解析，忽略这条规则", "rule_id", rule.ID, "err", err)
		return false, nil
	}
	ix, err := r.index(ctx)
	if err != nil {
		return false, err
	}
	return f.match(r.card, ix), nil
}

// execute 按顺序执行规则的每个动作，并把结果写成一条 automation.fired 事件。
// actions 列出真正生效的动作，skipped 列出因为参数原因无法执行的动作；
// 全是"本来就不需要改动"的空操作时不写事件，免得活动记录被没有意义的条目刷屏。
// 只有数据库错误才会返回错误让整个操作回滚。
func (r *ruleRun) execute(ctx context.Context, rule *Rule) error {
	done, skipped := []string{}, []string{}
	for _, a := range rule.Actions {
		effect, skip, err := r.apply(ctx, rule, a)
		if err != nil {
			return err
		}
		if effect {
			done = append(done, a.Type)
		}
		if skip != "" {
			skipped = append(skipped, a.Type+"："+skip)
		}
	}
	if len(done) == 0 && len(skipped) == 0 {
		return nil
	}
	return r.u.emit(ctx, AutomationActor(rule.Name), newEvent{
		BoardID: r.card.BoardID, CardID: r.card.ID, Type: EventAutomationFired,
		Payload: map[string]any{
			"key": r.card.Key, "rule_id": rule.ID, "rule": rule.Name, "actions": done, "skipped": skipped,
		},
	})
}

// apply 执行一个动作。effect 表示动作生效了，skip 不为空表示动作无法执行，值是原因。
// 评论动作单独处理；其余动作只改内存里的卡片，改动了就标记 changed，等调用方保存。
func (r *ruleRun) apply(ctx context.Context, rule *Rule, a RuleAction) (effect bool, skip string, err error) {
	if a.Type == ActionAddComment {
		_, err := r.u.addComment(ctx, AutomationActor(rule.Name), r.card, a.Text)
		return err == nil, "", err
	}
	changed, skip := applyFieldAction(r.card, a, r.u.now())
	r.changed = r.changed || changed
	return changed, skip, nil
}
