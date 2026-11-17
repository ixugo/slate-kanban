package kanban

import (
	"context"
)

const (
	// defaultEventLimit 网页补发和活动列表一次最多返回的事件数，没指定时的默认值。
	defaultEventLimit = 200
	// maxEventLimit 网页补发和活动列表一次最多返回的事件数的上限。
	maxEventLimit = 1000
)

// eventWindow 是从某个序号之后读出的一页原始事件，还没有按读取者过滤。
type eventWindow struct {
	events    []*Event
	latest    int64 // 看板当前最新的事件序号
	hasMore   bool  // 这一页之后还有事件
	truncated bool  // 起点早于还保留着的事件，中间有事件已经被清理
}

// readWindow 读出看板里序号大于 from 的至多 limit 条事件，按序号升序。
// 起点比保留着的最早事件还早一个以上的序号时，说明中间可能有事件被清理了：
// 起点改成保留范围的开头并标记 truncated，让读取者知道要重新读整块看板。
// 序号是所有看板共用的，所以相邻事件的序号可能不连续，只有"起点早于最早事件减一"才说明可能有缺口。
func (c Core) readWindow(ctx context.Context, boardID, from int64, limit int) (*eventWindow, error) {
	latest, err := c.store.Event().LatestSeq(ctx, boardID)
	if err != nil {
		return nil, wrapDB("读取最新事件序号", err)
	}
	minSeq, err := c.store.Event().MinSeq(ctx, boardID)
	if err != nil {
		return nil, wrapDB("读取最早事件序号", err)
	}
	w := &eventWindow{latest: latest}
	if minSeq > 0 && from+1 < minSeq {
		w.truncated = true
		from = minSeq - 1
	}
	events, err := c.store.Event().ListAfter(ctx, boardID, from, limit+1)
	if err != nil {
		return nil, wrapDB("读取事件", err)
	}
	if len(events) > limit {
		w.hasMore = true
		events = events[:limit]
	}
	w.events = events
	return w, nil
}

// EventPage 是 ListEvents 返回的一页事件。
type EventPage struct {
	Events []*Event `json:"events"`
	// Latest 是看板当前最新的事件序号。
	Latest  int64 `json:"latest"`
	HasMore bool  `json:"has_more"`
	// Truncated 为真表示请求的起点之后有事件已经被清理，网页应该重新读取整块看板。
	Truncated bool `json:"truncated"`
}

// ListEvents 返回看板里序号大于 afterSeq 的事件，按序号升序，不做任何过滤。
// 网页的实时流重连时用它补发断线期间的事件，活动列表也用它；人和 agent 都可以读。
// limit 为 0 取默认值，超过上限取上限，负数返回 400；看板不存在返回 404。
func (c Core) ListEvents(ctx context.Context, boardID, afterSeq int64, limit int) (*EventPage, error) {
	if afterSeq < 0 {
		return nil, invalidf("after_seq 不能是负数")
	}
	limit, err := normalizeLimit(limit, defaultEventLimit, maxEventLimit)
	if err != nil {
		return nil, err
	}
	if _, err := c.store.Board().GetByID(ctx, boardID); err != nil {
		return nil, mapNotFound(err, ErrBoardNotFound, "读取看板")
	}
	w, err := c.readWindow(ctx, boardID, afterSeq, limit)
	if err != nil {
		return nil, err
	}
	return &EventPage{Events: w.events, Latest: w.latest, HasMore: w.hasMore, Truncated: w.truncated}, nil
}
