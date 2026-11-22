package kanban

import (
	"context"
	"time"
)

const (
	// defaultListLimit 列表接口不指定数量时返回的任务数。
	defaultListLimit = 200
	// maxListLimit 列表接口一次最多返回的任务数，防止一次响应过大。
	maxListLimit = 500
)

// ListCardsInput 是列出任务的入参。Query 是筛选字符串，语法见 kanban.card.go.md；
// Limit 为 0 表示用默认值，超过上限按上限处理，负数不合法。
type ListCardsInput struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

// CardList 是列出任务的结果。Total 是命中的总数，Truncated 表示命中数超过了 Limit，
// 只返回了前面一部分；调用方可以收紧筛选条件后再查。
type CardList struct {
	Cards     []*CardView `json:"cards"`
	Total     int         `json:"total"`
	Truncated bool        `json:"truncated"`
}

// normalizeListLimit 把调用方给的数量换成实际使用的数量：0 取默认值，超过上限取上限，负数返回错误。
func normalizeListLimit(n int) (int, error) {
	return normalizeLimit(n, defaultListLimit, maxListLimit)
}

// ListCards 按筛选条件列出看板里的任务，顺序和看板上看到的一致：先按列从左到右，同一列里按位置。
// 人和 agent 都可以读。命中的任务带有从列、依赖、租约推出来的信息，见 CardView。
// 数据量级：一次读出看板的全部任务在内存里筛选，个人和小团队的看板任务数在几千以内，足够用。
func (c Core) ListCards(ctx context.Context, boardID int64, in ListCardsInput) (*CardList, error) {
	limit, err := normalizeListLimit(in.Limit)
	if err != nil {
		return nil, err
	}
	f, err := parseFilter(in.Query)
	if err != nil {
		return nil, err
	}
	if _, err := c.store.Board().GetByID(ctx, boardID); err != nil {
		return nil, mapNotFound(err, ErrBoardNotFound, "读取看板")
	}
	cols, err := c.store.Column().ListByBoard(ctx, boardID)
	if err != nil {
		return nil, wrapDB("读取列", err)
	}
	tags, err := c.store.Tag().ListByBoard(ctx, boardID)
	if err != nil {
		return nil, wrapDB("读取标签库", err)
	}
	cards, err := c.store.Card().ListByBoard(ctx, boardID)
	if err != nil {
		return nil, wrapDB("读取任务", err)
	}
	ix := newBoardIndex(c.now(), time.Local, cols, tags, cards)
	return ix.list(f, limit), nil
}

// list 把索引里的任务按显示顺序排好，筛选后只为前 limit 个命中的任务生成视图。
func (ix *boardIndex) list(f *filter, limit int) *CardList {
	out := &CardList{Cards: []*CardView{}}
	for _, card := range ix.sortCards(ix.all) {
		if !f.match(card, ix) {
			continue
		}
		out.Total++
		if len(out.Cards) < limit {
			out.Cards = append(out.Cards, ix.view(card))
		}
	}
	out.Truncated = out.Total > len(out.Cards)
	return out
}
