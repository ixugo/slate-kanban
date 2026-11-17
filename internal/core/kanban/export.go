package kanban

import (
	"context"
	"time"
)

// ExportCard 是导出里的一张卡片：视图字段加评论列表；没有评论时用空切片而不是 null。
type ExportCard struct {
	*CardView
	Comments []*Comment `json:"comments"`
}

// BoardExport 是整板导出在 Core 层的结果，不含 JSON/Markdown 包装字段。
type BoardExport struct {
	*BoardDetail
	Cards []*ExportCard `json:"cards"`
	Seq   int64         `json:"seq"`
}

// ExportBoard 读取整板导出数据，权限与快照相同：人和 agent 都可读；看板不存在返回 404。
// 评论按看板一次性批量读出，再按卡片分组挂到各卡片下。
func (c Core) ExportBoard(ctx context.Context, _ Actor, boardID int64) (*BoardExport, error) {
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
		return nil, wrapDB("读取卡片", err)
	}
	allComments, err := c.store.Comment().ListByBoard(ctx, boardID)
	if err != nil {
		return nil, wrapDB("读取评论", err)
	}
	byCard := groupCommentsByCard(allComments)
	ix := newBoardIndex(c.now(), time.Local, detail.Columns, cards)
	sorted := ix.sortCards(ix.all)
	out := make([]*ExportCard, len(sorted))
	for i, card := range sorted {
		cmts := byCard[card.ID]
		if cmts == nil {
			cmts = []*Comment{}
		}
		out[i] = &ExportCard{CardView: ix.view(card), Comments: cmts}
	}
	return &BoardExport{BoardDetail: detail, Cards: out, Seq: seq}, nil
}

// groupCommentsByCard 把评论按卡片主键分组，输入应已按卡片、评论主键排好序。
func groupCommentsByCard(items []*Comment) map[int64][]*Comment {
	if len(items) == 0 {
		return map[int64][]*Comment{}
	}
	out := make(map[int64][]*Comment)
	for _, c := range items {
		out[c.CardID] = append(out[c.CardID], c)
	}
	return out
}
