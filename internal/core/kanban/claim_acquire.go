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
		return u.grantClaim(ctx, actor, card, lease)
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
		cardID, err = u.claimNext(ctx, actor, boardID, lease)
		return err
	})
	if err != nil || cardID == 0 {
		return nil, err
	}
	return c.GetCardDetail(ctx, cardID)
}

// claimNext 在事务里找出下一张可领的卡片并领取，返回它的主键，没有可领的返回 0。
// 复杂度：一次读出看板的全部卡片，逐张判断资格，每张卡的判断只和它的依赖数有关。
func (u *unit) claimNext(ctx context.Context, actor Actor, boardID int64, lease time.Duration) (int64, error) {
	ix, err := u.boardIndex(ctx, boardID)
	if err != nil {
		return 0, err
	}
	var pick *Card
	for _, card := range ix.sortCards(ix.all) {
		if col := ix.columns[card.ColumnID]; col == nil || !col.AgentPickup || checkClaimable(actor, card, ix) != nil {
			continue
		}
		if card.ClaimedBy == actor.Name {
			pick = card
			break
		}
		if pick == nil {
			pick = card
		}
	}
	if pick == nil {
		return 0, nil
	}
	return pick.ID, u.grantClaim(ctx, actor, pick, lease)
}

// boardIndex 在事务里读出看板的全部列和卡片，建好索引。
func (u *unit) boardIndex(ctx context.Context, boardID int64) (*boardIndex, error) {
	cols, err := u.column.ListByBoard(ctx, boardID)
	if err != nil {
		return nil, wrapDB("读取列", err)
	}
	cards, err := u.card.ListByBoard(ctx, boardID)
	if err != nil {
		return nil, wrapDB("读取卡片", err)
	}
	return newBoardIndex(u.now(), time.Local, cols, cards), nil
}

// claimIndex 为判断单张卡片的领取资格建一个小索引：看板的列，加上这张卡片和它依赖的卡片，
// 不用把整个看板的卡片都读出来。
func (u *unit) claimIndex(ctx context.Context, card *Card) (*boardIndex, error) {
	cols, err := u.column.ListByBoard(ctx, card.BoardID)
	if err != nil {
		return nil, wrapDB("读取列", err)
	}
	deps, err := loadDependencies(ctx, u.card, card)
	if err != nil {
		return nil, err
	}
	return newBoardIndex(u.now(), time.Local, cols, append([]*Card{card}, deps...)), nil
}

// grantClaim 把卡片记到 agent 名下，租约从现在起算 lease 这么长，并追加 card.claimed 事件。
// 持有者本人再次领取只是顺延到期时间，不涨版本号，因为卡片内容没有变；
// 新领取或者接手别人过期的领取，版本号加一。
func (u *unit) grantClaim(ctx context.Context, actor Actor, card *Card, lease time.Duration) error {
	expires := u.now().Add(lease).UTC()
	renewal := card.ClaimedBy == actor.Name
	card.ClaimedBy, card.ClaimExpiresAt = actor.Name, &expires
	if renewal {
		if err := u.card.Save(ctx, card); err != nil {
			return wrapDB("续租", err)
		}
	} else if err := u.saveCard(ctx, actor, card); err != nil {
		return err
	}
	return u.emit(ctx, actor, newEvent{
		BoardID: card.BoardID, CardID: card.ID, Type: EventCardClaimed,
		Payload: cardPayload(card, map[string]any{"agent": actor.Name, "expires_at": expires.Format(time.RFC3339)}),
	})
}
