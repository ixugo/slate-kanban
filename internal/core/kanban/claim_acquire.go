package kanban

import (
	"context"
	"time"
)

// ClaimCard 让 agent 领取指定的任务，任务已经由自己持有时只是续租。
// 只有 agent 能领取；资格见 checkClaimable。领取成功返回任务详情，
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
		if in.RenewOnly {
			if !claimActive(card, ix.now) || card.ClaimedBy != actor.Name || card.ClaimEngineerID != in.EngineerID {
				return ErrNotHolder
			}
		} else if in.DesktopDispatch && claimActive(card, ix.now) {
			return ErrCardClaimed
		}
		engineer, err := u.chooseEngineer(ctx, actor, card, in.EngineerID)
		if err != nil {
			return err
		}
		return u.grantClaim(ctx, actor, card, ix.now, lease, engineer, in.DesktopDispatch)
	})
	if err != nil {
		return nil, err
	}
	return c.GetCardDetail(ctx, cardID)
}

// NextCard 给 agent 取下一个该做的任务并领取。取卡在锁住看板行的事务里完成，
// 所以多个 agent 同时取也不会拿到同一张。顺序完全由人决定：agent 按提示词选定一列，该列从上到下，
// 优先级只是展示，不参与排序。自己已经持有的可领任务优先，这样 agent 重启后能接着做原来的活。
// 没有可做的任务不是错误，返回 nil，由 agent 决定是等待还是结束。
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
		cardID, err = u.claimNext(ctx, actor, boardID, in.ColumnID, lease, in.EngineerID)
		return err
	})
	if err != nil || cardID == 0 {
		return nil, err
	}
	return c.GetCardDetail(ctx, cardID)
}

// claimNext 在事务里找出下一个可领的任务并领取，返回它的主键，没有可领的返回 0。
// 复杂度：一次读出看板的全部任务，逐个判断资格，每个任务的判断只和它的依赖数有关。
// 看板有工程师时，任务还要有空闲的所选类型工程师能承担才算可领，没有就留在原处排队；
// 租约有效的任务只有自己接着做时才会再次取到：指定了工程师就必须是原来那一位，没指定视为接着做原来那一位。
// 同名的多个 agent 只要各自带着自己的工程师来取，就不会互相抢卡。
func (u *unit) claimNext(ctx context.Context, actor Actor, boardID, columnID int64, lease time.Duration, engineerID int64) (int64, error) {
	ix, err := u.boardIndex(ctx, boardID)
	if err != nil {
		return 0, err
	}
	col := ix.columns[columnID]
	if col == nil {
		return 0, invalidf("必须指定本看板的 column_id，并按该列提示词选择任务")
	}
	if col.IsDone {
		return 0, invalidf("不能从完成列领取下一个任务")
	}
	pool, err := u.loadEngineerPool(ctx, boardID, ix.all)
	if err != nil {
		return 0, err
	}
	var pick *Card
	var pickBy *Engineer
	for _, card := range ix.sortCards(ix.all) {
		if card.ColumnID != columnID || checkClaimable(actor, card, ix) != nil {
			continue
		}
		by, ok := pickEngineerFor(pool, actor, card, engineerID, ix.now)
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
	return pick.ID, u.grantClaim(ctx, actor, pick, ix.now, lease, pickBy, false)
}

// pickEngineerFor 判断取下一张卡时这张卡能不能由工程师承担，返回承担者和是否可领。
// 看板没有工程师时永远可领；有工程师时，租约有效的任务只有持有者带着同一位工程师才可领。
func pickEngineerFor(pool *engineerPool, actor Actor, card *Card, engineerID int64, now time.Time) (*Engineer, bool) {
	if pool == nil {
		return nil, true
	}
	if claimActive(card, now) {
		if card.ClaimedBy != actor.Name || (engineerID != 0 && card.ClaimEngineerID != engineerID) {
			return nil, false
		}
		engineerID = card.ClaimEngineerID
	}
	id, err := pool.engineerFor(card, engineerID)
	if err != nil {
		return nil, false
	}
	return pool.find(id), true
}

// chooseEngineer 为领取指定的任务决定承担的工程师，并核对租约有效时的持有者。
// 没有指定工程师而任务正由自己持有时，视为接着做原来的工程师那一份（续租）。
func (u *unit) chooseEngineer(ctx context.Context, actor Actor, card *Card, requested int64) (*Engineer, error) {
	pool, err := u.loadEngineerPool(ctx, card.BoardID, nil)
	if err != nil || pool == nil {
		return nil, err
	}
	now := u.now()
	if requested == 0 && card.ClaimedBy == actor.Name && claimActive(card, now) {
		requested = card.ClaimEngineerID
	}
	id, err := pool.engineerFor(card, requested)
	if err != nil {
		return nil, err
	}
	if err := checkHolder(actor, card, id, now, time.Local); err != nil {
		return nil, err
	}
	return pool.find(id), nil
}

// boardIndex 在事务里读出看板的全部列、标签和任务，建好索引。
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
		return nil, wrapDB("读取任务", err)
	}
	return newBoardIndex(u.now(), time.Local, cols, tags, cards), nil
}

// claimIndex 为判断单个任务的领取资格建一个小索引：看板的列，加上这个任务和它依赖的任务，
// 不用把整个看板的任务都读出来。
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

// grantClaim 把任务记到 agent 和工程师名下，租约从现在起算。
// 同一个有效租约只更新到期时间，不涨版本号，也不追加事件：续租没有断过，就不重复记录。
// 新领取才追加 card.claimed；盖掉的是一条已经到期的旧租约时，先追加一条租约到期（expireClaim），再追加新的领取。
func (u *unit) grantClaim(ctx context.Context, actor Actor, card *Card, now time.Time, lease time.Duration, engineer *Engineer, desktopDispatch bool) error {
	var engineerID int64
	if engineer != nil {
		engineerID = engineer.ID
	}
	expires := now.Add(lease).UTC()
	renewal := claimActive(card, now) && card.ClaimedBy == actor.Name && card.ClaimEngineerID == engineerID
	if err := u.expireClaim(ctx, actor, card, now); err != nil {
		return err
	}
	card.ClaimedBy, card.ClaimExpiresAt, card.ClaimEngineerID = actor.Name, &expires, engineerID
	if engineer != nil && stageOf(engineer.Kind) == EngineerDev && card.EngineerID == 0 {
		card.EngineerID = engineerID
		if renewal {
			return u.saveCard(ctx, actor, card)
		}
	}
	if renewal {
		if err := u.card.Save(ctx, card); err != nil {
			return wrapDB("续租", err)
		}
		return nil
	}
	if err := u.saveCard(ctx, actor, card); err != nil {
		return err
	}
	var engineerName *string
	if engineer != nil {
		// 空名称让普通 agent 的新事件可与尚未记录名称的旧桌面事件区分。
		name := ""
		if desktopDispatch {
			name = engineer.Name
		}
		engineerName = &name
	}
	return u.emitClaimed(ctx, actor, card, expires, engineerName)
}
