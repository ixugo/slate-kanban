package kanban

import (
	"context"

	"github.com/ixugo/goddd/pkg/orm"
)

const (
	// defaultPullLimit agent 一次拉取最多返回的事件数，没指定时的默认值。
	defaultPullLimit = 100
	// maxPullLimit agent 一次拉取最多返回的事件数的上限。
	maxPullLimit = 500
)

// quietEventTypes 是只影响看板上卡片或列的排列、或者只是自动化的流水记录的事件。
// 它们对 agent 没有处理价值，拉取时直接跳过，避免 agent 被拖动排序之类的操作反复唤醒。
var quietEventTypes = map[string]bool{
	EventCardReordered:   true,
	EventColumnReordered: true,
	EventAutomationFired: true,
}

// PullInput 是 agent 拉取更新的参数。
type PullInput struct {
	// Since 是 agent 确认已经处理完的事件序号，一般填上一次返回的 Cursor。
	// 不填表示不确认任何东西，从服务端记着的进度继续读，所以上一批没处理完可以再读一遍。
	Since *int64 `json:"since"`
	// Limit 一次最多返回多少条事件，0 取默认值，超过上限取上限。
	Limit int `json:"limit"`
}

// Updates 是一次拉取的结果。
type Updates struct {
	// Events 是 agent 需要看的事件，按序号升序。自己做的操作和排序类事件已经被过滤掉了。
	Events []*Event `json:"events"`
	// Cursor 是这一批读到的最后一个序号，下一次拉取把它填进 Since 就表示这一批已经处理完。
	// 事件被过滤掉时它依然会往前走，所以过滤不会让 agent 反复读到同样的东西。
	Cursor int64 `json:"cursor"`
	// Latest 是看板当前最新的事件序号。
	Latest int64 `json:"latest"`
	// HasMore 为真表示这一批之后还有事件，应该立刻再拉一次。
	HasMore bool `json:"has_more"`
	// First 为真表示这是这个 agent 第一次读这个看板：不补发历史，进度直接设在最新处，
	// 要了解现状请读看板快照。
	First bool `json:"first"`
	// Truncated 为真表示 agent 离线太久，中间有事件已经被清理，需要重新读看板快照。
	Truncated bool `json:"truncated"`
	// TimedOut 为真表示等待超时仍然没有新事件，只会出现在 WaitUpdates 的结果里。
	TimedOut bool `json:"timed_out"`
}

// PullUpdates 返回某个 agent 在看板上还没读到的事件，是 agent 离线后补读和日常轮询的入口。
// 进度记在服务端，每个 (看板, agent) 一份：只有下一次调用的 Since 才会推进进度，
// 所以返回的结果丢了或者没处理完，重新拉取还能拿到同样的事件，保证不丢通知。
// 只有 agent 能调用；看板不存在返回 404；Since 为负数返回 400。
func (c Core) PullUpdates(ctx context.Context, actor Actor, boardID int64, in PullInput) (*Updates, error) {
	if err := requireAgent(actor); err != nil {
		return nil, err
	}
	limit, err := normalizeLimit(in.Limit, defaultPullLimit, maxPullLimit)
	if err != nil {
		return nil, err
	}
	from, first, err := c.ackCursor(ctx, actor.Name, boardID, in.Since)
	if err != nil {
		return nil, err
	}
	if first {
		return &Updates{Events: []*Event{}, Cursor: from, Latest: from, First: true}, nil
	}
	return c.scanUpdates(ctx, actor.Name, boardID, from, limit)
}

// ackCursor 处理 Since 确认并返回这次应该从哪个序号之后开始读。
// 带了 Since 就把进度推进到它（进度只增不减，超过最新序号的部分截到最新序号，
// 免得写错的大数字把以后的新事件都跳过去）；没带 Since 且这个 agent 从没读过这个看板，
// 就把进度设在最新处并返回 first，不补发历史；其余情况沿用已经记着的进度。
func (c Core) ackCursor(ctx context.Context, agent string, boardID int64, since *int64) (from int64, first bool, err error) {
	if since != nil && *since < 0 {
		return 0, false, invalidf("since 不能是负数")
	}
	if _, err := c.store.Board().GetByID(ctx, boardID); err != nil {
		return 0, false, mapNotFound(err, ErrBoardNotFound, "读取看板")
	}
	latest, err := c.store.Event().LatestSeq(ctx, boardID)
	if err != nil {
		return 0, false, wrapDB("读取最新事件序号", err)
	}
	if since != nil {
		from, err = c.advanceCursor(ctx, boardID, agent, min(*since, latest))
		return from, false, err
	}
	cur, err := c.store.Cursor().Get(ctx, boardID, agent)
	if err == nil {
		return cur.LastSeq, false, nil
	}
	if !orm.IsErrRecordNotFound(err) {
		return 0, false, wrapDB("读取 agent 进度", err)
	}
	from, err = c.advanceCursor(ctx, boardID, agent, latest)
	return from, true, err
}

// advanceCursor 把 agent 的进度推进到 seq 并返回推进后的值，进度只增不减。
func (c Core) advanceCursor(ctx context.Context, boardID int64, agent string, seq int64) (int64, error) {
	got, err := c.store.Cursor().Advance(ctx, boardID, agent, seq)
	if err != nil {
		return 0, wrapDB("推进 agent 进度", err)
	}
	return got, nil
}

// scanUpdates 从 from 之后读一页事件，按 agent 的视角过滤，并把进度位置算好。
func (c Core) scanUpdates(ctx context.Context, agent string, boardID, from int64, limit int) (*Updates, error) {
	w, err := c.readWindow(ctx, boardID, from, limit)
	if err != nil {
		return nil, err
	}
	out := &Updates{Events: []*Event{}, Cursor: from, Latest: w.latest, HasMore: w.hasMore, Truncated: w.truncated}
	for _, ev := range w.events {
		if relevantToAgent(agent, ev) {
			out.Events = append(out.Events, ev)
		}
	}
	if n := len(w.events); n > 0 {
		out.Cursor = w.events[n-1].ID
	}
	return out, nil
}

// relevantToAgent 判断一条事件是否值得通知这个 agent：
// 它自己做的操作不用告诉它，排序类和自动化流水也不用。
func relevantToAgent(agent string, ev *Event) bool {
	if quietEventTypes[ev.Type] {
		return false
	}
	return ev.ActorKind != string(ActorAgent) || ev.Actor != agent
}
