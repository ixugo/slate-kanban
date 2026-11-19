package kanban

import (
	"context"
	"time"
)

// ClaimCard 让 agent 领取指定的卡片，卡片已经由自己持有时只是续租。
// 只有 agent 能领取；资格见 checkClaimable。领取成功返回卡片详情，
// 里面有所在列给 agent 的工作指令、依赖和全部评论，agent 拿到就能开工。
func (c Core) ClaimCard(ctx context.Context, actor Actor, cardID int64, in ClaimCardInput) (*CardDetail, error) {
	if err := requireAgent(actor); err != nil {
		return nil, err
	}
	lease, err := normalizeLease(in.LeaseMinutes)
	if err != nil {
		return nil, err
	}
	boardID, err := c.boardOfCard(ctx, cardID)
	if err != nil {
		return nil, err
	}
	err = c.run(ctx, boardID, func(u *unit) error {
		card, err := u.loadCard(ctx, boardID, cardID)
		if err != nil {
			return err
		}
		ix, err := u.claimIndex(ctx, card)
		if err != nil {
			return err
		}
		if err := checkClaimable(actor, card, ix); err != nil {
			return err
		}
		by, err := u.chooseEngineer(ctx, actor, card, ix.columns[card.ColumnID], in.EngineerID)
		if err != nil {
			return err
		}
		return u.grantClaim(ctx, actor, card, lease, by)
	})
	if err != nil {
		return nil, err
	}
	return c.GetCardDetail(ctx, cardID)
}

// NextCard 给 agent 取下一张该做的卡片并领取。取卡在锁住看板行的事务里完成，
// 所以多个 agent 同时取也不会拿到同一张。顺序完全由人决定：领活列从左到右，同一列从上到下，
// 优先级只是展示，不参与排序。自己已经持有的可领卡片优先，这样 agent 重启后能接着做原来的活。
// 没有可做的卡片不是错误，返回 nil，由 agent 决定是等待还是结束。
func (c Core) NextCard(ctx context.Context, actor Actor, boardID int64, in NextCardInput) (*CardDetail, error) {
	if err := requireAgent(actor); err != nil {
		return nil, err
	}
	lease, err := normalizeLease(in.LeaseMinutes)
	if err != nil {
		return nil, err
	}
	var cardID int64
	err = c.run(ctx, boardID, func(u *unit) error {
		var err error
		cardID, err = u.claimNext(ctx, actor, boardID, lease, in.EngineerID)
		return err
	})
	if err != nil || cardID == 0 {
		return nil, err
	}
	return c.GetCardDetail(ctx, cardID)
}

// claimNext 在事务里找出下一张可领的卡片并领取，返回它的主键，没有可领的返回 0。
// 复杂度：一次读出看板的全部卡片，逐张判断资格，每张卡的判断只和它的依赖数有关。
// 看板有工程师时，卡片还要有空闲的研发工程师能承担才算可领，没有就留在原处排队；
// 租约有效的卡片只有自己接着做时才会再次取到：指定了工程师就必须是原来那一位，没指定视为接着做原来那一位。
// 同名的多个 agent 只要各自带着自己的工程师来取，就不会互相抢卡。
func (u *unit) claimNext(ctx context.Context, actor Actor, boardID int64, lease time.Duration, engineerID int64) (int64, error) {
	ix, err := u.boardIndex(ctx, boardID)
	if err != nil {
		return 0, err
	}
	pool, err := u.loadEngineerPool(ctx, boardID, ix.all)
	if err != nil {
		return 0, err
	}
	var pick *Card
	var pickBy claimBy
	for _, card := range ix.sortCards(ix.all) {
		col := ix.columns[card.ColumnID]
		if col == nil || !col.AgentPickup || checkClaimable(actor, card, ix) != nil {
			continue
		}
		by, ok := pickEngineerFor(pool, actor, card, col, engineerID, ix.now)
		if !ok {
			continue
		}
		if card.ClaimedBy == actor.Name {
			pick, pickBy = card, by
			break
		}
		if pick == nil {
			pick, pickBy = card, by
		}
	}
	if pick == nil {
		return 0, nil
	}
	return pick.ID, u.grantClaim(ctx, actor, pick, lease, pickBy)
}

// pickEngineerFor 判断取下一张卡时这张卡能不能由工程师承担，返回承担者和是否可领。
// 看板没有工程师时永远可领；有工程师时，租约有效的卡片只有持有者带着同一位工程师才可领。
func pickEngineerFor(pool *engineerPool, actor Actor, card *Card, col *Column, engineerID int64, now time.Time) (claimBy, bool) {
	if pool == nil {
		return claimBy{}, true
	}
	if claimActive(card, now) {
		if card.ClaimedBy != actor.Name || (engineerID != 0 && card.ClaimEngineerID != engineerID) {
			return claimBy{}, false
		}
		engineerID = card.ClaimEngineerID
	}
	id, err := pool.engineerFor(card, col, engineerID)
	if err != nil {
		return claimBy{}, false
	}
	return claimBy{engineerID: id, dev: stageKind(col) == EngineerDev}, true
}

// chooseEngineer 为领取指定的卡片决定承担的工程师，并核对租约有效时的持有者。
// 没有指定工程师而卡片正由自己持有时，视为接着做原来的工程师那一份（续租）。
func (u *unit) chooseEngineer(ctx context.Context, actor Actor, card *Card, col *Column, requested int64) (claimBy, error) {
	pool, err := u.loadEngineerPool(ctx, card.BoardID, nil)
	if err != nil || pool == nil {
		return claimBy{}, err
	}
	now := u.now()
	if requested == 0 && card.ClaimedBy == actor.Name && claimActive(card, now) {
		requested = card.ClaimEngineerID
	}
	id, err := pool.engineerFor(card, col, requested)
	if err != nil {
		return claimBy{}, err
	}
	if err := checkHolder(actor, card, id, now, time.Local); err != nil {
		return claimBy{}, err
	}
	return claimBy{engineerID: id, dev: stageKind(col) == EngineerDev}, nil
}

// boardIndex 在事务里读出看板的全部列、标签和卡片，建好索引。
func (u *unit) boardIndex(ctx context.Context, boardID int64) (*boardIndex, error) {
	cols, err := u.column.ListByBoard(ctx, boardID)
	if err != nil {
		return nil, wrapDB("读取列", err)
	}
	tags, err := u.tag.ListByBoard(ctx, boardID)
	if err != nil {
		return nil, wrapDB("读取标签库", err)
	}
	cards, err := u.card.ListByBoard(ctx, boardID)
	if err != nil {
		return nil, wrapDB("读取卡片", err)
	}
	return newBoardIndex(u.now(), time.Local, cols, tags, cards), nil
}

// claimIndex 为判断单张卡片的领取资格建一个小索引：看板的列，加上这张卡片和它依赖的卡片，
// 不用把整个看板的卡片都读出来。
func (u *unit) claimIndex(ctx context.Context, card *Card) (*boardIndex, error) {
	cols, err := u.column.ListByBoard(ctx, card.BoardID)
	if err != nil {
		return nil, wrapDB("读取列", err)
	}
	tags, err := u.tag.ListByBoard(ctx, card.BoardID)
	if err != nil {
		return nil, wrapDB("读取标签库", err)
	}
	deps, err := loadDependencies(ctx, u.card, card)
	if err != nil {
		return nil, err
	}
	return newBoardIndex(u.now(), time.Local, cols, tags, append([]*Card{card}, deps...)), nil
}

// claimBy 是一次领取承担者里和工程师有关的部分：engineerID 是承担的工程师（0 表示没有），
// dev 表示这次是研发领取，此时卡片还没有绑定工程师就把这一位记成卡片的工程师。
type claimBy struct {
	engineerID int64
	dev        bool
}

// grantClaim 把卡片记到 agent 和工程师名下，租约从现在起算 lease 这么长，并追加 card.claimed 事件。
// 持有者本人用同一位工程师再次领取只是顺延到期时间，不涨版本号，因为卡片内容没有变；
// 新领取或者接手别人过期的领取，版本号加一。
func (u *unit) grantClaim(ctx context.Context, actor Actor, card *Card, lease time.Duration, by claimBy) error {
	expires := u.now().Add(lease).UTC()
	renewal := card.ClaimedBy == actor.Name && card.ClaimEngineerID == by.engineerID
	card.ClaimedBy, card.ClaimExpiresAt, card.ClaimEngineerID = actor.Name, &expires, by.engineerID
	if by.dev && by.engineerID != 0 && card.EngineerID == 0 {
		card.EngineerID, renewal = by.engineerID, false
	}
	if renewal {
		if err := u.card.Save(ctx, card); err != nil {
			return wrapDB("续租", err)
		}
	} else if err := u.saveCard(ctx, actor, card); err != nil {
		return err
	}
	return u.emit(ctx, actor, newEvent{
		BoardID: card.BoardID, CardID: card.ID, Type: EventCardClaimed,
		Payload: cardPayload(card, map[string]any{"agent": actor.Name, "engineer_id": by.engineerID, "expires_at": expires.Format(time.RFC3339)}),
	})
}
