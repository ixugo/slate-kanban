package kanban

import (
	"context"
	"errors"
	"slices"
)

const (
	// positionStep 卡片位置的基本间距：追加到列头列尾时的步长，也是整列重排后相邻卡片的间距。
	positionStep = 1024.0
	// minPositionGap 插入后新位置和任意一个相邻位置的差小于它，就认为浮点精度快用完了，需要整列重排。
	minPositionGap = 1e-6
)

// errNeedRenumber 是包内的信号：位置间隙太小，调用方要先整列重排再重新计算。它不会返回给外部调用方。
var errNeedRenumber = errors.New("kanban: 卡片位置间隙太小，需要整列重排")

// computePosition 计算一张卡片放进目标列的位置，以及它在 cards 里的插入下标（0 到 len(cards)）。
// cards 是目标列里除去正在移动的这张以外的卡片，按位置升序。
// beforeID 和 afterID 最多一个非零：放到参照卡片的前面或后面；都为零就放到列尾。
// 位置用浮点数，插到两张卡之间取中点，所以拖动一次只需要改被拖的这一张卡。
// 参照卡片不在这一列里返回 ErrInvalidInput；间隙太小返回 errNeedRenumber。
func computePosition(cards []*Card, beforeID, afterID int64) (float64, int, error) {
	if beforeID != 0 && afterID != 0 {
		return 0, 0, invalidf("before_id 和 after_id 不能同时指定")
	}
	refID := max(beforeID, afterID)
	if refID == 0 {
		return tailPosition(cards), len(cards), nil
	}
	idx := slices.IndexFunc(cards, func(c *Card) bool { return c.ID == refID })
	if idx < 0 {
		return 0, 0, invalidf("参照卡片不在目标列里")
	}
	if afterID != 0 {
		idx++ // 放到参照卡片后面，插入下标就是它的下一个
	}
	pos, err := positionBetween(cards, idx-1, idx)
	return pos, idx, err
}

// appendPosition 读出某一列当前最大的位置，返回放到这一列末尾的位置，空列是第一个间距。
func (u *unit) appendPosition(ctx context.Context, columnID int64) (float64, error) {
	maxPos, err := u.card.MaxPosition(ctx, columnID)
	if err != nil {
		return 0, wrapDB("读取列尾位置", err)
	}
	return maxPos + positionStep, nil
}

// tailPosition 返回放到列尾的位置：最后一张的位置加一个间距，空列是第一个间距。
func tailPosition(cards []*Card) float64 {
	if len(cards) == 0 {
		return positionStep
	}
	return cards[len(cards)-1].Position + positionStep
}

// positionBetween 返回 cards[lo] 和 cards[hi] 之间的位置。下标越界表示那一侧是列头或列尾，
// 此时往外多取一个间距。两侧都有卡片时取中点，中点离任意一侧太近就返回 errNeedRenumber。
func positionBetween(cards []*Card, lo, hi int) (float64, error) {
	switch {
	case lo < 0:
		return cards[hi].Position - positionStep, nil
	case hi >= len(cards):
		return cards[lo].Position + positionStep, nil
	}
	low, high := cards[lo].Position, cards[hi].Position
	mid := (low + high) / 2
	if mid-low < minPositionGap || high-mid < minPositionGap {
		return 0, errNeedRenumber
	}
	return mid, nil
}

// placeCard 计算卡片放进目标列的位置和插入下标；间隙太小时先整列重排再重新计算。
// cards 的含义同 computePosition，整列重排会就地修改它们的位置。
func (u *unit) placeCard(ctx context.Context, cards []*Card, beforeID, afterID int64) (float64, int, error) {
	pos, idx, err := computePosition(cards, beforeID, afterID)
	if !errors.Is(err, errNeedRenumber) {
		return pos, idx, err
	}
	if err := u.renumberColumn(ctx, cards); err != nil {
		return 0, 0, err
	}
	return computePosition(cards, beforeID, afterID)
}

// renumberColumn 按现有顺序把整列的位置重写成间距的整数倍，只写位置真的变了的卡片。
// 重排是内部整理：位置的重写不涨版本号、不写事件，因为卡片的顺序本身没有变化。
func (u *unit) renumberColumn(ctx context.Context, cards []*Card) error {
	for i, c := range cards {
		want := float64(i+1) * positionStep
		if c.Position == want {
			continue
		}
		if err := u.card.SetPosition(ctx, c.ID, want); err != nil {
			return wrapDB("整列重排卡片位置", err)
		}
		c.Position = want
	}
	return nil
}
