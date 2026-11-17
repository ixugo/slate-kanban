package kanban

import (
	"cmp"
	"slices"
	"time"
)

// CardView 是卡片加上从列、依赖和租约推出来的只读信息。网页和 agent 读卡片时都拿它，
// 这样"在不在完成列""是不是在等别的卡""领取是否还有效"由服务器按同一套规则算好，
// 客户端不用重复实现，也不会因为本机时钟不准而判断错。
type CardView struct {
	*Card
	// Column 是卡片所在列的名称。
	Column string `json:"column"`
	// Done 表示卡片位于完成列。
	Done bool `json:"done"`
	// Waiting 表示卡片有还没完成的依赖。
	Waiting bool `json:"waiting"`
	// ClaimActive 表示卡片正被某个 agent 领取，并且租约还没到期。
	ClaimActive bool `json:"claim_active"`
}

// boardIndex 是一个看板的列和卡片在内存里的索引，一次构建后可以反复用来生成 CardView 和做筛选匹配，
// 避免对每张卡片都去查一次列和依赖。
type boardIndex struct {
	now      time.Time
	loc      *time.Location // 日期类筛选按这个时区的自然日比较
	columns  map[int64]*Column
	colOrder map[int64]int // 列主键到它在看板里从左到右的序号
	cards    map[int64]*Card
	doneCols map[int64]bool
}

// newBoardIndex 用看板的全部列（已按左到右排序）和全部卡片建立索引。
// 复杂度是列数加卡片数的线性级。
func newBoardIndex(now time.Time, loc *time.Location, cols []*Column, cards []*Card) *boardIndex {
	ix := &boardIndex{
		now: now, loc: loc,
		columns:  make(map[int64]*Column, len(cols)),
		colOrder: make(map[int64]int, len(cols)),
		cards:    make(map[int64]*Card, len(cards)),
		doneCols: make(map[int64]bool),
	}
	for i, col := range cols {
		ix.columns[col.ID] = col
		ix.colOrder[col.ID] = i
		ix.doneCols[col.ID] = col.IsDone
	}
	for _, card := range cards {
		ix.cards[card.ID] = card
	}
	return ix
}

// isWaiting 判断卡片是否有没完成的依赖。
func (ix *boardIndex) isWaiting(card *Card) bool {
	return len(unfinishedDeps(card, ix.cards, ix.doneCols)) > 0
}

// view 生成一张卡片的完整视图。
func (ix *boardIndex) view(card *Card) *CardView {
	v := &CardView{
		Card: card, Done: ix.doneCols[card.ColumnID],
		Waiting: ix.isWaiting(card), ClaimActive: claimActive(card, ix.now),
	}
	if col, ok := ix.columns[card.ColumnID]; ok {
		v.Column = col.Name
	}
	return v
}

// sortCards 把卡片按看板里的显示顺序排列：先按列从左到右，同一列里按位置从小到大，位置相同按主键。
// 就地排序并返回同一个切片。
func (ix *boardIndex) sortCards(cards []*Card) []*Card {
	slices.SortStableFunc(cards, func(a, b *Card) int {
		return cmp.Or(
			cmp.Compare(ix.colOrder[a.ColumnID], ix.colOrder[b.ColumnID]),
			cmp.Compare(a.Position, b.Position),
			cmp.Compare(a.ID, b.ID),
		)
	})
	return cards
}
