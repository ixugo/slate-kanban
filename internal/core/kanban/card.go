package kanban

import (
	"context"
	"fmt"
	"slices"

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
// agent 只能填标题、描述、标签、分支和链接，规划类字段只有人能填。
// 各列允许 agent 做什么由列的提示词约定，这里不按列限制。编号在锁住看板行的事务里递增分配，所以并发创建也不会重复。
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
		err = u.emit(ctx, actor, newEvent{
			BoardID: boardID, CardID: card.ID, Type: EventCardCreated,
			Payload: map[string]any{
				"key": card.Key, "title": card.Title, "column_id": card.ColumnID, "column": columnName,
			},
		})
		if err != nil {
			return err
		}
		return u.fireCreateRules(ctx, actor, card)
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
	now := u.now()
	card := &Card{
		BoardID: boardID, ColumnID: target.ID, Version: 1,
		CreatedBy: actor.Name, UpdatedBy: actor.Name, CreatedAt: now, UpdatedAt: now,
	}
	patch, tagsAdded, err := u.resolveCardTags(ctx, boardID, in.asPatch(actor.IsAgent()))
	if err != nil {
		return nil, "", err
	}
	if _, err := patch.apply(card); err != nil {
		return nil, "", err
	}
	if err := u.checkEngineerID(ctx, boardID, card.EngineerID); err != nil {
		return nil, "", err
	}
	if tagsAdded {
		if err := u.emitTagsUpdated(ctx, actor, boardID); err != nil {
			return nil, "", err
		}
	}
	if len(card.WaitsOn) > 0 {
		if err := u.checkWaitsOn(ctx, boardID, 0, card.WaitsOn); err != nil {
			return nil, "", err
		}
	}
	if err := u.assignNumberAndPosition(ctx, card, in.Top); err != nil {
		return nil, "", err
	}
	if err := u.card.Create(ctx, card); err != nil {
		return nil, "", wrapDB("创建卡片", err)
	}
	if err := u.commitImages(ctx, boardID, card.Description); err != nil {
		return nil, "", err
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
	if col := columnByID(cols, columnID); col != nil {
		return col, nil
	}
	return nil, invalidf("目标列不存在，或者不属于这个看板")
}

// columnByID 在看板的列里按主键找一列，找不到返回 nil。
func columnByID(cols []*Column, id int64) *Column {
	for _, col := range cols {
		if col.ID == id {
			return col
		}
	}
	return nil
}

// assignNumberAndPosition 给新卡片分配看板内的编号和目标列的位置：默认放列尾，top 为真放列头。
// 编号由看板上的计数器加一得到，永不回收；Key 由看板标识和编号拼成。
func (u *unit) assignNumberAndPosition(ctx context.Context, card *Card, top bool) error {
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
	if !top {
		pos, err := u.appendPosition(ctx, card.ColumnID)
		card.Position = pos
		return err
	}
	others, err := u.card.ListByColumn(ctx, card.ColumnID)
	if err != nil {
		return wrapDB("读取目标列的卡片", err)
	}
	card.Position, _, err = u.placeCard(ctx, others, topRef(others, true, 0, 0), 0)
	return err
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

// boardOfCard 在事务之外查出卡片属于哪个看板，用来决定要锁哪个看板的行。
// 卡片不会换看板，所以事务里再读卡片时只需要确认它还在。
func (c Core) boardOfCard(ctx context.Context, cardID int64) (int64, error) {
	card, err := c.store.Card().GetByID(ctx, cardID)
	if err != nil {
		return 0, mapNotFound(err, ErrCardNotFound, "读取卡片")
	}
	return card.BoardID, nil
}

// loadCard 在事务里读取卡片，并确认它属于 boardID。卡片已经被删除（比如在查看板和锁看板之间）
// 或者不属于这个看板，都按不存在处理。
func (u *unit) loadCard(ctx context.Context, boardID, cardID int64) (*Card, error) {
	card, err := u.card.GetByID(ctx, cardID)
	if err != nil {
		return nil, mapNotFound(err, ErrCardNotFound, "读取卡片")
	}
	if card.BoardID != boardID {
		return nil, ErrCardNotFound
	}
	return card, nil
}

// checkCardWritable 判断操作者能不能写这张卡片：人可以；agent 只能写自己持有的。
// 持有的判断只看 ClaimedBy，租约过期但还没被别人接手时仍然算持有，
// 这样耗时很长的任务不会因为忘了续租而白做。
func checkCardWritable(actor Actor, card *Card) error {
	if actor.IsAgent() && card.ClaimedBy != actor.Name {
		return ErrNotHolder
	}
	return nil
}

// saveCard 保存一次真实的卡片修改：版本号加一，记下修改者和时间，写入数据库。
func (u *unit) saveCard(ctx context.Context, actor Actor, card *Card) error {
	card.Version++
	card.UpdatedBy = actor.Name
	card.UpdatedAt = u.now()
	if err := u.card.Save(ctx, card); err != nil {
		return wrapDB("保存卡片", err)
	}
	return nil
}

// cardPayload 返回卡片事件的公共内容：卡片编号和标题，再并入 extra 里的字段。
func cardPayload(card *Card, extra map[string]any) map[string]any {
	payload := map[string]any{"key": card.Key, "title": card.Title}
	for k, v := range extra {
		payload[k] = v
	}
	return payload
}

// saveCardChange 保存卡片并追加一条以它为主角的事件，事件内容是 cardPayload(card, extra)。
func (u *unit) saveCardChange(ctx context.Context, actor Actor, card *Card, eventType string, extra map[string]any) error {
	if err := u.saveCard(ctx, actor, card); err != nil {
		return err
	}
	return u.emit(ctx, actor, newEvent{BoardID: card.BoardID, CardID: card.ID, Type: eventType, Payload: cardPayload(card, extra)})
}

// UpdateCard 修改卡片，只更新入参里出现的字段，值没变的字段不算修改。
// 人可以改全部字段；agent 只能改自己持有的卡片，并且只能改标题、描述、标签、分支和链接，
// 优先级、依赖、时间这些规划类字段只有人能动。
// 入参带了 Version 并且和当前不一致返回 409；没有任何字段真的变化时不涨版本、不写事件。
func (c Core) UpdateCard(ctx context.Context, actor Actor, cardID int64, in UpdateCardInput) (*Card, error) {
	if err := requireWriter(actor); err != nil {
		return nil, err
	}
	if actor.IsAgent() && in.touchesPlanning() {
		return nil, ErrHumanOnly.WithMsg("agent 不能修改优先级、依赖、时间和仅限人处理这些规划类字段")
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
		return u.updateCard(ctx, actor, card, in)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// updateCard 在事务里完成 UpdateCard 的检查、赋值和保存。card 是刚读出的最新数据。
func (u *unit) updateCard(ctx context.Context, actor Actor, card *Card, in UpdateCardInput) error {
	if err := checkCardWritable(actor, card); err != nil {
		return err
	}
	if in.Version != nil && *in.Version != card.Version {
		return ErrCardVersionConflict
	}
	in, tagsAdded, err := u.resolveCardTags(ctx, card.BoardID, in)
	if err != nil {
		return err
	}
	if tagsAdded {
		if err := u.emitTagsUpdated(ctx, actor, card.BoardID); err != nil {
			return err
		}
	}
	oldDescription := card.Description
	changed, err := in.apply(card)
	if err != nil || len(changed) == 0 {
		return err
	}
	if slices.Contains(changed, "engineer_id") {
		if err := u.checkEngineerID(ctx, card.BoardID, card.EngineerID); err != nil {
			return err
		}
	}
	if slices.Contains(changed, "waits_on") {
		if err := u.checkWaitsOn(ctx, card.BoardID, card.ID, card.WaitsOn); err != nil {
			return err
		}
	}
	u.renewLease(actor, card)
	if err := u.saveCard(ctx, actor, card); err != nil {
		return err
	}
	if slices.Contains(changed, "description") {
		if err := u.syncImages(ctx, card.BoardID, oldDescription, card.Description); err != nil {
			return err
		}
	}
	return u.emitCardUpdates(ctx, actor, card, changed)
}

// emitCardUpdates 按改了什么写事件：标签的变化单独写 card.tags_updated，
// 其余字段写 card.updated，同时改了两类就两条都写。
// 标签变化很琐碎，拆成独立类型后，网页的活动流可以用 exclude_types 把它整条排除，而实时广播照常有。
func (u *unit) emitCardUpdates(ctx context.Context, actor Actor, card *Card, changed []string) error {
	rest := slices.DeleteFunc(slices.Clone(changed), func(f string) bool { return f == "tag_ids" })
	if len(rest) > 0 {
		ev := newEvent{BoardID: card.BoardID, CardID: card.ID, Type: EventCardUpdated, Payload: cardPayload(card, map[string]any{"fields": rest})}
		if err := u.emit(ctx, actor, ev); err != nil {
			return err
		}
	}
	if len(rest) == len(changed) {
		return nil
	}
	ev := newEvent{BoardID: card.BoardID, CardID: card.ID, Type: EventCardTagsUpdated, Payload: cardPayload(card, map[string]any{"fields": []string{"tag_ids"}})}
	return u.emit(ctx, actor, ev)
}
