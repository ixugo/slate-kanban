package kanban

import (
	"context"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	// defaultActivityLimit 活动流一页返回的事件数，没指定时的默认值。
	defaultActivityLimit = 50
	// maxActivityLimit 活动流一页最多返回的事件数。
	maxActivityLimit = 200
)

// knownEventTypes 是活动流允许按类型筛选的全部事件类型。
// 只放行这些值，免得把任意字符串带进查询条件，也让拼错的类型直接得到 400 而不是一个永远为空的列表。
var knownEventTypes = []string{
	EventBoardCreated, EventBoardUpdated, EventBoardLabelsUpdated,
	EventColumnCreated, EventColumnUpdated, EventColumnDeleted, EventColumnReordered,
	EventCardCreated, EventCardUpdated, EventCardTagsUpdated, EventCardMoved, EventCardReordered, EventCardDeleted,
	EventCardClaimed, EventCardReleased,
	EventCommentAdded, EventCommentDeleted,
	EventRuleCreated, EventRuleUpdated, EventRuleDeleted,
	EventAutomationFired,
}

// ActivityFilter 是活动流的筛选条件，每一项为空都表示不限。
type ActivityFilter struct {
	// Types 只看这些类型的事件，取值必须在 knownEventTypes 里。
	Types []string
	// ExcludeTypes 不要这些类型的事件，取值必须在 knownEventTypes 里；和 Types 同时出现时以排除为准。
	ExcludeTypes []string
	// ActorKind 只看这一类操作者（人、agent、系统）的事件。
	ActorKind ActorKind
	// Actor 只看这个名字的操作者的事件。
	Actor string
	// CardID 只看和这个任务有关的事件（建卡、改卡、移动、评论、认领等），0 表示不限。
	CardID int64
}

// ActivityQuery 是读活动流的请求：从最新的事件往回翻页。
type ActivityQuery struct {
	// Before 只看序号小于它的事件，0 表示从最新的开始；翻下一页时传上一页最后一条的序号。
	Before int64
	// Limit 一页最多返回几条，0 取默认值。
	Limit int
	ActivityFilter
}

// ActivityActor 是看板事件里出现过的一个操作者，网页用它生成「按操作者筛选」的选项。
type ActivityActor struct {
	Kind ActorKind `json:"kind"`
	Name string    `json:"name"`
}

// ActivityPage 是活动流的一页事件，最新的在前。
type ActivityPage struct {
	Events []*Event `json:"events"`
	// HasMore 为真表示这一页之后还有更早的、满足条件的事件。
	HasMore bool `json:"has_more"`
	// Latest 是看板当前最新的事件序号，不受筛选条件影响。
	Latest int64 `json:"latest"`
	// Actors 是这个看板的事件里出现过的全部操作者，不受筛选条件影响。
	Actors []ActivityActor `json:"actors"`
}

// normalizeEventTypes 检查事件类型都是已知类型，并去掉重复项，保持传入的顺序。
func normalizeEventTypes(in []string) ([]string, error) {
	var out []string
	for _, t := range in {
		if !slices.Contains(knownEventTypes, t) {
			return nil, invalidf("不认识的事件类型 %q", t)
		}
		if !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	return out, nil
}

// normalizeActivityFilter 校验并整理筛选条件：类型必须是已知类型，重复的去掉；
// 操作者类型必须是三种之一；操作者名称去掉首尾空白，长度不超过名称上限；任务编号不能是负数。
func normalizeActivityFilter(f ActivityFilter) (ActivityFilter, error) {
	out := ActivityFilter{ActorKind: f.ActorKind, Actor: strings.TrimSpace(f.Actor), CardID: f.CardID}
	if out.CardID < 0 {
		return out, invalidf("card_id 不能是负数")
	}
	var err error
	if out.Types, err = normalizeEventTypes(f.Types); err != nil {
		return out, err
	}
	if out.ExcludeTypes, err = normalizeEventTypes(f.ExcludeTypes); err != nil {
		return out, err
	}
	switch out.ActorKind {
	case "", ActorHuman, ActorAgent, ActorSystem:
	default:
		return out, invalidf("不认识的操作者类型 %q", out.ActorKind)
	}
	if utf8.RuneCountInString(out.Actor) > maxActorNameLen {
		return out, invalidf("操作者名称不能超过 %d 个字符", maxActorNameLen)
	}
	return out, nil
}

// ListActivity 返回看板的活动流：按序号从新到旧，可按事件类型和操作者筛选，用 Before 翻页。
// 人和 agent 都可以读。参数不合法返回 400，看板不存在返回 404。
// 筛选发生在数据库里，所以每页都是满的，不会出现「翻了一页却全被过滤掉」；
// 每个看板最多保留 10000 条事件，筛选条件很少命中时最坏也只扫这么多行。
func (c Core) ListActivity(ctx context.Context, boardID int64, q ActivityQuery) (*ActivityPage, error) {
	if q.Before < 0 {
		return nil, invalidf("before 不能是负数")
	}
	limit, err := normalizeLimit(q.Limit, defaultActivityLimit, maxActivityLimit)
	if err != nil {
		return nil, err
	}
	filter, err := normalizeActivityFilter(q.ActivityFilter)
	if err != nil {
		return nil, err
	}
	if _, err := c.store.Board().GetByID(ctx, boardID); err != nil {
		return nil, mapNotFound(err, ErrBoardNotFound, "读取看板")
	}
	events, err := c.store.Event().ListBefore(ctx, boardID, q.Before, filter, limit+1)
	if err != nil {
		return nil, wrapDB("读取活动流", err)
	}
	hasMore := len(events) > limit
	if hasMore {
		events = events[:limit]
	}
	latest, err := c.store.Event().LatestSeq(ctx, boardID)
	if err != nil {
		return nil, wrapDB("读取最新事件序号", err)
	}
	actors, err := c.store.Event().Actors(ctx, boardID)
	if err != nil {
		return nil, wrapDB("读取操作者列表", err)
	}
	return &ActivityPage{Events: events, HasMore: hasMore, Latest: latest, Actors: actors}, nil
}
