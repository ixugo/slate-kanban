package kanban

import (
	"cmp"
	"slices"
	"time"
)

// CardView 是任务加上从列、依赖和租约推出来的只读信息。网页和 agent 读任务时都拿它，
// 这样"在不在完成列""是不是在等别的卡""领取是否还有效"由服务器按同一套规则算好，
// 客户端不用重复实现，也不会因为本机时钟不准而判断错。
type CardView struct {
	*Card
	// Tags 是任务上带文字的标签的文字（按任务上的顺序，没有文字的标签不在里面），给 agent 和脚本读，
	// 要连颜色一起用请拿 TagIDs 去看板的标签库里查。
	Tags []string `json:"tags"`
	// Column 是任务所在列的名称。
	Column string `json:"column"`
	// Done 表示任务位于完成列。
	Done bool `json:"done"`
	// Waiting 表示任务有还没完成的依赖。
	Waiting bool `json:"waiting"`
	// ClaimActive 表示任务正被某个 agent 领取，并且租约还没到期。
	ClaimActive bool `json:"claim_active"`
	// UnreadCount 是网页未读动态数，只给 REST 用；库里存在 Card 上，序列化时单独挂出来。
	UnreadCount int `json:"unread_count"`
}

// boardIndex 是一个看板的列和任务在内存里的索引，一次构建后可以反复用来生成 CardView 和做筛选匹配，
// 避免对每个任务都去查一次列和依赖。
type boardIndex struct {
	now      time.Time
	loc      *time.Location // 日期类筛选按这个时区的自然日比较
	cols     []*Column      // 看板的列，从左到右
	columns  map[int64]*Column
	colOrder map[int64]int // 列主键到它在看板里从左到右的序号
	all      []*Card       // 建索引时传入的任务，调用方把所有权交给了索引，排序时会就地改动顺序
	cards    map[int64]*Card
	doneCols map[int64]bool
	tagTexts map[int64]string // 标签主键到文字
}

// newBoardIndex 用看板的全部列（已按左到右排序）、全部标签和全部任务建立索引。
// 复杂度是列数、标签数加任务数的线性级。
func newBoardIndex(now time.Time, loc *time.Location, cols []*Column, tags []*Tag, cards []*Card) *boardIndex {
	ix := &boardIndex{
		now: now, loc: loc, cols: cols, all: cards,
		columns:  make(map[int64]*Column, len(cols)),
		colOrder: make(map[int64]int, len(cols)),
		cards:    make(map[int64]*Card, len(cards)),
		doneCols: make(map[int64]bool),
		tagTexts: make(map[int64]string, len(tags)),
	}
	for _, t := range tags {
		ix.tagTexts[t.ID] = t.Text
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

// isWaiting 判断任务是否有没完成的依赖。
func (ix *boardIndex) isWaiting(card *Card) bool {
	return len(unfinishedDeps(card, ix.cards, ix.doneCols)) > 0
}

// view 生成一个任务的完整视图。
func (ix *boardIndex) view(card *Card) *CardView {
	v := &CardView{
		Card: card, Tags: ix.tagNames(card), Done: ix.doneCols[card.ColumnID],
		Waiting: ix.isWaiting(card), ClaimActive: claimActive(card, ix.now),
		UnreadCount: card.UnreadCount,
	}
	if col, ok := ix.columns[card.ColumnID]; ok {
		v.Column = col.Name
	}
	return v
}

// tagNames 返回任务上带文字的标签的文字，按任务上的顺序；没有时返回空切片而不是 nil。
func (ix *boardIndex) tagNames(card *Card) []string {
	out := make([]string, 0, len(card.TagIDs))
	for _, id := range card.TagIDs {
		if text := ix.tagTexts[id]; text != "" {
			out = append(out, text)
		}
	}
	return out
}

// sortCards 把任务按看板里的显示顺序排列：先按列从左到右，同一列里按位置从小到大，位置相同按主键。
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
