package kanban

import (
	"context"
	"fmt"

	"github.com/ixugo/goddd/pkg/orm"
)

// CardStorer 是卡片的数据持久化接口。读出的卡片都已经调用过 Normalize。
// 卡片的修改都发生在已经锁住看板行的事务里，所以更新直接用 Save，不再重复加行锁。
type CardStorer interface {
	// WithTx 返回绑定到事务的副本，事务内的所有读写都必须用它。
	WithTx(orm.Tx) CardStorer
	// Create 新增一张卡片。Key 重复时返回可用 orm.IsDuplicatedKey 判断的错误。
	Create(context.Context, *Card) error
	// Save 把整张卡片写回数据库，布尔值和 0 值也会写入。
	Save(context.Context, *Card) error
	// Delete 按主键删除，幂等：卡片不存在也返回成功。
	Delete(context.Context, *Card) error
	// GetByID 按主键读取，不存在时返回可用 orm.IsErrRecordNotFound 判断的错误。
	GetByID(context.Context, int64) (*Card, error)
	// GetByKey 按卡片编号（例如 SLATE-12）读取，不存在时错误同 GetByID。
	GetByKey(context.Context, string) (*Card, error)
	// ListByBoard 返回看板的全部卡片，按所在列的主键、位置、主键升序。
	ListByBoard(context.Context, int64) ([]*Card, error)
	// ListByColumn 返回某一列的卡片，按位置升序，位置相同按主键升序。
	ListByColumn(context.Context, int64) ([]*Card, error)
	// CountByColumn 返回某一列有几张卡片。
	CountByColumn(context.Context, int64) (int64, error)
	// MaxPosition 返回某一列里最大的位置，空列返回 0。
	MaxPosition(context.Context, int64) (float64, error)
	// SetPosition 只更新位置，用于整列重排。
	SetPosition(ctx context.Context, id int64, position float64) error
	// DeleteByBoard 删除看板的全部卡片，用于删除看板。
	DeleteByBoard(context.Context, int64) error
}

// CreateCard 在看板里创建一张卡片，放在目标列的末尾。人和 agent 都可以创建：
// agent 只能填标题、描述、标签、分支和链接，也不能放进开启了 agent 领取的列，
// 否则它就能自己创建卡片再自己领取，绕开由人决定做什么的约定。
// 编号在锁住看板行的事务里递增分配，所以并发创建也不会重复。
func (c Core) CreateCard(ctx context.Context, actor Actor, boardID int64, in CreateCardInput) (*Card, error) {
	if err := requireWriter(actor); err != nil {
		return nil, err
	}
	if actor.IsAgent() && in.planningSet() {
		return nil, ErrHumanOnly.WithMsg("agent 不能设置优先级、依赖、时间和仅限人处理这些规划类字段")
	}
	var out *Card
	err := c.run(ctx, boardID, func(u *unit) error {
		card, columnName, err := u.buildNewCard(ctx, actor, boardID, in)
		if err != nil {
			return err
		}
		out = card
		return u.emit(ctx, actor, newEvent{
			BoardID: boardID, CardID: card.ID, Type: EventCardCreated,
			Payload: map[string]any{
				"key": card.Key, "title": card.Title, "column_id": card.ColumnID, "column": columnName,
			},
		})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// buildNewCard 校验入参、分配编号和位置并写入新卡片，返回卡片和它所在列的名称（事件里要用）。
// 它被 CreateCard 放在事务里调用。
func (u *unit) buildNewCard(ctx context.Context, actor Actor, boardID int64, in CreateCardInput) (*Card, string, error) {
	cols, err := u.column.ListByBoard(ctx, boardID)
	if err != nil {
		return nil, "", wrapDB("读取列", err)
	}
	target, err := pickCreateColumn(cols, in.ColumnID)
	if err != nil {
		return nil, "", err
	}
	if actor.IsAgent() && target.AgentPickup {
		return nil, "", ErrAgentPickupColumn
	}
	now := u.now()
	card := &Card{
		BoardID: boardID, ColumnID: target.ID, Version: 1,
		CreatedBy: actor.Name, UpdatedBy: actor.Name, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := in.asPatch(actor.IsAgent()).apply(card); err != nil {
		return nil, "", err
	}
	if len(card.WaitsOn) > 0 {
		if err := u.checkWaitsOn(ctx, boardID, 0, card.WaitsOn); err != nil {
			return nil, "", err
		}
	}
	if err := u.assignNumberAndPosition(ctx, card); err != nil {
		return nil, "", err
	}
	if err := u.card.Create(ctx, card); err != nil {
		return nil, "", wrapDB("创建卡片", err)
	}
	return card, target.Name, nil
}

// pickCreateColumn 决定新卡片放进哪一列：指定了就必须属于这个看板，没指定就放第一列。
func pickCreateColumn(cols []*Column, columnID int64) (*Column, error) {
	if columnID == 0 {
		if len(cols) == 0 {
			return nil, invalidf("看板还没有任何列，请先创建列")
		}
		return cols[0], nil
	}
	for _, col := range cols {
		if col.ID == columnID {
			return col, nil
		}
	}
	return nil, invalidf("目标列不存在，或者不属于这个看板")
}

// assignNumberAndPosition 给新卡片分配看板内的编号和目标列末尾的位置。
// 编号由看板上的计数器加一得到，永不回收；Key 由看板标识和编号拼成。
func (u *unit) assignNumberAndPosition(ctx context.Context, card *Card) error {
	board, err := u.board.GetByID(ctx, card.BoardID)
	if err != nil {
		return mapNotFound(err, ErrBoardNotFound, "读取看板")
	}
	number, err := u.board.IncrCardSeq(ctx, card.BoardID)
	if err != nil {
		return wrapDB("分配卡片编号", err)
	}
	card.Number = number
	card.Key = fmt.Sprintf("%s-%d", board.Slug, number)
	maxPos, err := u.card.MaxPosition(ctx, card.ColumnID)
	if err != nil {
		return wrapDB("读取列尾位置", err)
	}
	card.Position = maxPos + positionStep
	return nil
}

// checkWaitsOn 读出看板的全部卡片，校验 deps 能不能作为 cardID 的依赖。新建卡片时 cardID 传 0。
func (u *unit) checkWaitsOn(ctx context.Context, boardID, cardID int64, deps []int64) error {
	cards, err := u.card.ListByBoard(ctx, boardID)
	if err != nil {
		return wrapDB("读取卡片", err)
	}
	return validateWaitsOn(cardID, deps, cards)
}

// GetCard 按主键读取卡片。卡片不存在返回 404。人和 agent 都可以读。
func (c Core) GetCard(ctx context.Context, id int64) (*Card, error) {
	card, err := c.store.Card().GetByID(ctx, id)
	if err != nil {
		return nil, mapNotFound(err, ErrCardNotFound, "读取卡片")
	}
	return card, nil
}
