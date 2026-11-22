package kanban

import (
	"context"

	"github.com/ixugo/goddd/pkg/orm"
)

// CardChangeStorer 是任务变更日志的数据持久化接口。记录只增不改，随任务或看板一起删除。
type CardChangeStorer interface {
	// WithTx 返回绑定到事务的副本，事务内的所有读写都必须用它。
	WithTx(orm.Tx) CardChangeStorer
	// Create 新增一条变更记录。
	Create(context.Context, *CardChange) error
	// GetByID 按主键读取，不存在时返回可用 orm.IsErrRecordNotFound 判断的错误。
	GetByID(context.Context, int64) (*CardChange, error)
	// Neighbors 返回同一张任务里编号紧挨着 changeID 的前一条和后一条记录的编号，没有就是 0。
	Neighbors(ctx context.Context, cardID, changeID int64) (prev, next int64, err error)
	// DeleteByCard 删除某个任务的全部变更记录，用于删除任务。
	DeleteByCard(context.Context, int64) error
	// DeleteByBoard 删除看板的全部变更记录，用于删除看板。
	DeleteByBoard(context.Context, int64) error
}

// recordDescriptionChange 在事务里把一次描述修改写进变更日志，返回记录的编号。
// card 是已经改好的任务，oldValue 是修改前的描述；事件载荷带上这个编号，网页点「查看变更」就用它取回前后两个版本。
func (u *unit) recordDescriptionChange(ctx context.Context, actor Actor, card *Card, oldValue string) (int64, error) {
	change := &CardChange{
		BoardID: card.BoardID, CardID: card.ID, Field: CardFieldDescription,
		OldValue: oldValue, NewValue: card.Description,
		ActorKind: string(actor.Kind), Actor: actor.Name, CreatedAt: u.now(),
	}
	if err := u.cardChange.Create(ctx, change); err != nil {
		return 0, wrapDB("记录描述变更", err)
	}
	return change.ID, nil
}

// GetCardChange 读取任务的一次变更记录，人和 agent 都可以读。
// 记录不存在，或者不属于这个任务，都按不存在处理：拿别的任务的记录编号来问，也读不到那边的内容。
func (c Core) GetCardChange(ctx context.Context, cardID, changeID int64) (*CardChange, error) {
	change, err := c.store.CardChange().GetByID(ctx, changeID)
	if err != nil {
		return nil, mapNotFound(err, ErrCardChangeNotFound, "读取变更记录")
	}
	if change.CardID != cardID {
		return nil, ErrCardChangeNotFound
	}
	if change.PrevID, change.NextID, err = c.store.CardChange().Neighbors(ctx, cardID, changeID); err != nil {
		return nil, wrapDB("读取相邻变更记录", err)
	}
	if change.PrevID != 0 {
		previous, err := c.store.CardChange().GetByID(ctx, change.PrevID)
		if err != nil {
			return nil, mapNotFound(err, ErrCardChangeNotFound, "读取前一条变更记录")
		}
		change.PreviousAt = previous.CreatedAt
	} else {
		card, err := c.store.Card().GetByID(ctx, cardID)
		if err != nil {
			return nil, mapNotFound(err, ErrCardChangeNotFound, "读取任务初始版本")
		}
		change.PreviousAt = card.CreatedAt
	}
	return change, nil
}
