package kanban

import (
	"fmt"
	"time"
)

// applyFieldAction 把只改卡片自己字段的动作应用到内存里的卡片上，不读写数据库。
// changed 表示卡片真的变了；skip 不为空表示动作因为参数原因无法执行，值是给人看的原因。
// "时间已经有值而模式是 if_empty"这类本来就不需要改动的情况，既不算 changed，也不算 skip。
// 评论、移动列和加减标签这几种需要读写数据库的动作不在这里处理，由规则引擎负责。
func applyFieldAction(card *Card, a RuleAction, now time.Time) (changed bool, skip string) {
	switch a.Type {
	case ActionSetStartedAt:
		return setTime(&card.StartedAt, a.Mode, now), ""
	case ActionSetFinishedAt:
		return setTime(&card.FinishedAt, a.Mode, now), ""
	case ActionClearStartedAt:
		return clearTime(&card.StartedAt), ""
	case ActionClearFinishedAt:
		return clearTime(&card.FinishedAt), ""
	case ActionSetDue:
		due := now.Add(time.Duration(a.Days) * 24 * time.Hour)
		card.DueAt = &due
		return true, ""
	case ActionClearDue:
		return clearTime(&card.DueAt), ""
	case ActionSetPriority:
		changed = card.Priority != a.Priority
		card.Priority = a.Priority
		return changed, ""
	case ActionSetHumanOnly:
		changed = card.HumanOnly != a.Flag
		card.HumanOnly = a.Flag
		return changed, ""
	}
	return false, fmt.Sprintf("不认识的动作类型 %q", a.Type)
}

// setTime 把时间字段设为 now。模式是 always 才覆盖已有的值，其余（包括没写模式）都只在为空时写入，
// 这样人手动修正过的时间不会被规则悄悄改掉。返回字段有没有变化。
func setTime(dst **time.Time, mode string, now time.Time) bool {
	if *dst != nil && mode != ModeAlways {
		return false
	}
	t := now
	*dst = &t
	return true
}

// clearTime 清空时间字段，返回字段有没有变化。
func clearTime(dst **time.Time) bool {
	if *dst == nil {
		return false
	}
	*dst = nil
	return true
}
