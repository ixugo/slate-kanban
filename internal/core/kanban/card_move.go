package kanban

import (
	"context"
	"slices"
)

// MoveCard 把卡片移到目标列的指定位置，也用来在同一列里调整顺序。
// 入参的 ColumnID 为零表示留在当前列；BeforeID 和 AfterID 最多给一个，表示放到那张卡的前面或后面，
// 都不给就放到目标列末尾。人可以移动任何卡片；agent 只能移动自己持有的卡片，
// 并且不能把卡片放进开启了 agent 领取的列，否则它就能把卡片重新放回队列。
// 换列会清除领取并写 card.moved；同列调整顺序写 card.reordered；位置没有变化时什么都不做。
func (c Core) MoveCard(ctx context.Context, actor Actor, cardID int64, in MoveCardInput) (*Card, error) {
	if err := requireWriter(actor); err != nil {
		return nil, err
	}
	boardID, err := c.boardOfCard(ctx, cardID)
	if err != nil {
		return nil, err
	}
	var out *Card
	err = c.run(ctx, boardID, func(u *unit) error {
		card, err := u.loadCard(ctx, boardID, cardID)
		if err != nil {
			return err
		}
		out = card
		return u.moveCard(ctx, actor, card, in)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// moveCard 在事务里完成 MoveCard：检查权限、找出原列和目标列、计算新位置，再按换列或同列分别保存。
func (u *unit) moveCard(ctx context.Context, actor Actor, card *Card, in MoveCardInput) error {
	if err := checkCardWritable(actor, card); err != nil {
		return err
	}
	from, to, err := u.resolveMove(ctx, actor, card, in.ColumnID)
	if err != nil {
		return err
	}
	others, err := u.cardsExcept(ctx, to.ID, card.ID)
	if err != nil {
		return err
	}
	// 当前下标必须在 placeCard 之前算：整列重排会改动其他卡片的位置，之后再比较就不准了。
	current := indexInColumn(others, card)
	pos, idx, err := u.placeCard(ctx, others, in.BeforeID, in.AfterID)
	if err != nil {
		return err
	}
	if from.ID == to.ID {
		if idx == current {
			return nil
		}
		return u.reorderCard(ctx, actor, card, pos)
	}
	return u.relocateCard(ctx, actor, card, from, to, pos)
}

// resolveMove 读出看板的列，返回卡片当前所在的列和移动的目标列。
// 目标列不属于这个看板、或者 agent 想把卡片放进领活列，都在这里拒绝。
func (u *unit) resolveMove(ctx context.Context, actor Actor, card *Card, columnID int64) (from, to *Column, err error) {
	cols, err := u.column.ListByBoard(ctx, card.BoardID)
	if err != nil {
		return nil, nil, wrapDB("读取列", err)
	}
	if from = columnByID(cols, card.ColumnID); from == nil {
		return nil, nil, ErrColumnNotFound
	}
	if columnID == 0 || columnID == from.ID {
		return from, from, nil
	}
	if to = columnByID(cols, columnID); to == nil {
		return nil, nil, invalidf("目标列不存在，或者不属于这个看板")
	}
	if actor.IsAgent() && to.AgentPickup {
		return nil, nil, ErrAgentPickupColumn
	}
	return from, to, nil
}

// cardsExcept 返回某一列里除了 exceptID 这张以外的卡片，按位置升序。
func (u *unit) cardsExcept(ctx context.Context, columnID, exceptID int64) ([]*Card, error) {
	cards, err := u.card.ListByColumn(ctx, columnID)
	if err != nil {
		return nil, wrapDB("读取目标列的卡片", err)
	}
	return slices.DeleteFunc(cards, func(c *Card) bool { return c.ID == exceptID }), nil
}

// indexInColumn 返回卡片按位置排序后在 others 里的插入下标，也就是排在它前面的卡片有几张。
// 位置相同时按主键排，和存储层的排序规则一致。
func indexInColumn(others []*Card, card *Card) int {
	n := 0
	for _, o := range others {
		if o.Position < card.Position || (o.Position == card.Position && o.ID < card.ID) {
			n++
		}
	}
	return n
}

// reorderCard 保存同一列里的新位置，并追加 card.reordered 事件。
// 这个事件只带编号和列，agent 拉取更新时会忽略它，不会因为人拖了一下顺序就被唤醒。
func (u *unit) reorderCard(ctx context.Context, actor Actor, card *Card, pos float64) error {
	card.Position = pos
	if err := u.saveCard(ctx, actor, card); err != nil {
		return err
	}
	return u.emit(ctx, actor, newEvent{
		BoardID: card.BoardID, CardID: card.ID, Type: EventCardReordered,
		Payload: map[string]any{"key": card.Key, "column_id": card.ColumnID},
	})
}

// relocateCard 把卡片搬到另一列：先清除领取（卡片不在原来的列了，原来的领取没有意义），
// 再保存新的列和位置，最后追加 card.moved 事件。
func (u *unit) relocateCard(ctx context.Context, actor Actor, card *Card, from, to *Column, pos float64) error {
	if err := u.clearClaim(ctx, actor, card, releaseMoved); err != nil {
		return err
	}
	card.ColumnID, card.Position = to.ID, pos
	return u.commitMove(ctx, actor, card, from, to)
}

// commitMove 在卡片的新列和位置已经写到内存里之后收尾：先追加 card.moved 事件，
// 再触发离开原列和进入新列的规则，最后保存一次卡片，规则对卡片的改动和换列一起落库。
// 手动移动和完成交接都走这里，换列的收尾只有这一处，所以规则对谁的操作都一样生效。
func (u *unit) commitMove(ctx context.Context, actor Actor, card *Card, from, to *Column) error {
	err := u.emit(ctx, actor, newEvent{
		BoardID: card.BoardID, CardID: card.ID, Type: EventCardMoved,
		Payload: cardPayload(card, movedPayload(from, to)),
	})
	if err != nil {
		return err
	}
	if err := u.newRuleRun(card).moved(ctx, from.ID, to.ID); err != nil {
		return err
	}
	return u.saveCard(ctx, actor, card)
}

// movedPayload 是 card.moved 事件里除了卡片编号和标题以外的内容：原列和目标列的主键与名称。
func movedPayload(from, to *Column) map[string]any {
	return map[string]any{"from_id": from.ID, "from": from.Name, "to_id": to.ID, "to": to.Name}
}
