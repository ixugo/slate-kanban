package kanban

import (
	"context"
	"errors"
)

// ReleaseCard 释放卡片上的领取。agent 只能释放自己持有的，人可以收回任何卡片的领取。
// 释放是幂等的：卡片不存在、没有被领取都返回成功，也不写事件。
// 原持有者被人收回以后再写这张卡，会收到 409，从而知道自己已经不持有了。
func (c Core) ReleaseCard(ctx context.Context, actor Actor, cardID int64) error {
	if err := requireWriter(actor); err != nil {
		return err
	}
	boardID, err := c.boardOfCard(ctx, cardID)
	if errors.Is(err, ErrCardNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return c.run(ctx, boardID, func(u *unit) error {
		card, err := u.loadCard(ctx, boardID, cardID)
		if errors.Is(err, ErrCardNotFound) {
			return nil // 在查看板和锁看板之间，别人已经删掉了
		}
		if err != nil {
			return err
		}
		return u.releaseCard(ctx, actor, card)
	})
}

// releaseCard 在事务里完成释放。持有者主动释放写 released，人收回写 revoked。
// 别的 agent 想释放：对方的租约还有效就拒绝；已经过期的领取在它看来本来就是空闲，什么都不做。
func (u *unit) releaseCard(ctx context.Context, actor Actor, card *Card) error {
	if card.ClaimedBy == "" {
		return nil
	}
	why := releaseRevoked
	if actor.IsAgent() {
		if card.ClaimedBy != actor.Name {
			if claimActive(card, u.now()) {
				return ErrNotHolder
			}
			return nil
		}
		why = releaseReleased
	}
	if err := u.clearClaim(ctx, actor, card, why); err != nil {
		return err
	}
	return u.saveCard(ctx, actor, card)
}
