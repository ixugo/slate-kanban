package kanban

import (
	"context"
	"time"
)

// Snapshot 是整板快照：看板、列、规则，加上全部任务和读取数据之前的最新事件序号。
// 网页首次加载和断线恢复时用它，再从 Seq 之后接收实时事件。
type Snapshot struct {
	*BoardDetail
	// Cards 是看板里的全部任务，顺序和看板上看到的一致：先按列从左到右，同一列里按位置。
	Cards []*CardView `json:"cards"`
	// Seq 是读取数据之前看板的最新事件序号。快照里的数据一定不比它旧，
	// 所以从 Seq 之后接收事件不会漏；读取期间发生的变化可能让事件被重复收到，只会多刷新一次。
	Seq int64 `json:"seq"`
}

// BoardSnapshot 一次读出整块看板。人和 agent 都可以读；看板不存在返回 404。
// 必须先读事件序号再读数据：反过来的话，两次读取之间发生的变化既不在数据里也不在序号之后的事件里，会漏掉。
// 数据量级：全部任务一次读出，个人和小团队的看板任务数在几千以内，足够用。
func (c Core) BoardSnapshot(ctx context.Context, boardID int64) (*Snapshot, error) {
	seq, err := c.store.Event().LatestSeq(ctx, boardID)
	if err != nil {
		return nil, wrapDB("读取最新事件序号", err)
	}
	detail, err := c.GetBoard(ctx, boardID)
	if err != nil {
		return nil, err
	}
	cards, err := c.store.Card().ListByBoard(ctx, boardID)
	if err != nil {
		return nil, wrapDB("读取任务", err)
	}
	ix := newBoardIndex(c.now(), time.Local, detail.Columns, detail.Tags, cards)
	sorted := ix.sortCards(ix.all)
	views := make([]*CardView, len(sorted))
	for i, card := range sorted {
		views[i] = ix.view(card)
	}
	return &Snapshot{BoardDetail: detail, Cards: views, Seq: seq}, nil
}
