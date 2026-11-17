package kanban

import (
	"context"
	"slices"

	"github.com/ixugo/goddd/pkg/orm"
)

// ListRules 列出看板的全部规则（包括停用的），按执行顺序排列。人和 agent 都可以读，
// agent 靠它了解看板上有哪些自动化，不会重复手工做规则已经做的事。
func (c Core) ListRules(ctx context.Context, boardID int64) ([]*Rule, error) {
	if _, err := c.store.Board().GetByID(ctx, boardID); err != nil {
		return nil, mapNotFound(err, ErrBoardNotFound, "读取看板")
	}
	rules, err := c.store.Rule().ListByBoard(ctx, boardID)
	if err != nil {
		return nil, wrapDB("读取规则", err)
	}
	return rules, nil
}

// CreateRule 在看板上新增一条规则，排在现有规则的最后。规则的增删改只允许人，
// 防止人设计的自动化被 AI 改乱。每个看板最多 maxRulesPerBoard 条，超过返回 ErrRuleLimit。
func (c Core) CreateRule(ctx context.Context, actor Actor, boardID int64, in CreateRuleInput) (*Rule, error) {
	if err := requireHuman(actor); err != nil {
		return nil, err
	}
	var out *Rule
	err := c.run(ctx, boardID, func(u *unit) error {
		cols, err := u.column.ListByBoard(ctx, boardID)
		if err != nil {
			return wrapDB("读取列", err)
		}
		rules, err := u.rule.ListByBoard(ctx, boardID)
		if err != nil {
			return wrapDB("读取规则", err)
		}
		if len(rules) >= maxRulesPerBoard {
			return ErrRuleLimit
		}
		now := u.now()
		r := &Rule{
			BoardID: boardID, Name: in.Name, Enabled: in.Enabled == nil || *in.Enabled, TriggerType: in.Trigger,
			ColumnID: in.ColumnID, Condition: in.Condition, Actions: in.Actions,
			Sort: nextRuleSort(rules), CreatedAt: now, UpdatedAt: now,
		}
		if err := normalizeRule(r, cols); err != nil {
			return err
		}
		if err := u.rule.Create(ctx, r); err != nil {
			return wrapDB("创建规则", err)
		}
		out = r
		return u.emit(ctx, actor, newEvent{BoardID: boardID, Type: EventRuleCreated, Payload: rulePayload(r)})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// nextRuleSort 返回新规则的排序值：现有最大值加一个间距，没有规则时是一个间距。
func nextRuleSort(rules []*Rule) int64 {
	if len(rules) == 0 {
		return ruleSortStep
	}
	return rules[len(rules)-1].Sort + ruleSortStep
}

// rulePayload 是规则事件的内容：规则的主键和名称。
func rulePayload(r *Rule) map[string]any {
	return map[string]any{"rule_id": r.ID, "name": r.Name}
}

// UpdateRule 修改规则，只更新入参里出现的字段，修改后整条规则重新校验。只允许人。
// 没有任何字段真的变化时不写库、不写事件。
func (c Core) UpdateRule(ctx context.Context, actor Actor, ruleID int64, in UpdateRuleInput) (*Rule, error) {
	if err := requireHuman(actor); err != nil {
		return nil, err
	}
	found, err := c.store.Rule().GetByID(ctx, ruleID)
	if err != nil {
		return nil, mapNotFound(err, ErrRuleNotFound, "读取规则")
	}
	var out *Rule
	err = c.run(ctx, found.BoardID, func(u *unit) error {
		r, err := u.rule.GetByID(ctx, ruleID)
		if err != nil {
			return mapNotFound(err, ErrRuleNotFound, "读取规则")
		}
		out = r
		return u.updateRule(ctx, actor, r, in)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// updateRule 在事务里应用修改、重新校验、保存并追加 rule.updated 事件。
func (u *unit) updateRule(ctx context.Context, actor Actor, r *Rule, in UpdateRuleInput) error {
	cols, err := u.column.ListByBoard(ctx, r.BoardID)
	if err != nil {
		return wrapDB("读取列", err)
	}
	before := *r
	in.apply(r)
	if err := normalizeRule(r, cols); err != nil {
		return err
	}
	if sameRule(&before, r) {
		return nil
	}
	r.UpdatedAt = u.now()
	if err := u.rule.Save(ctx, r); err != nil {
		return wrapDB("保存规则", err)
	}
	return u.emit(ctx, actor, newEvent{BoardID: r.BoardID, Type: EventRuleUpdated, Payload: rulePayload(r)})
}

// sameRule 判断两条规则的可配置内容是否完全一样。
func sameRule(a, b *Rule) bool {
	return a.Name == b.Name && a.Enabled == b.Enabled && a.TriggerType == b.TriggerType &&
		a.ColumnID == b.ColumnID && a.Condition == b.Condition && slices.Equal(a.Actions, b.Actions)
}

// DeleteRule 删除规则，只允许人。删除是幂等的：规则已经不存在也返回成功，并且不写事件。
func (c Core) DeleteRule(ctx context.Context, actor Actor, ruleID int64) error {
	if err := requireHuman(actor); err != nil {
		return err
	}
	found, err := c.store.Rule().GetByID(ctx, ruleID)
	if orm.IsErrRecordNotFound(err) {
		return nil
	}
	if err != nil {
		return wrapDB("读取规则", err)
	}
	return c.run(ctx, found.BoardID, func(u *unit) error {
		r, err := u.rule.GetByID(ctx, ruleID)
		if orm.IsErrRecordNotFound(err) {
			return nil // 在读取规则和锁看板之间，别人已经删掉了
		}
		if err != nil {
			return wrapDB("读取规则", err)
		}
		if err := u.rule.Delete(ctx, r); err != nil {
			return wrapDB("删除规则", err)
		}
		return u.emit(ctx, actor, newEvent{BoardID: r.BoardID, Type: EventRuleDeleted, Payload: rulePayload(r)})
	})
}
