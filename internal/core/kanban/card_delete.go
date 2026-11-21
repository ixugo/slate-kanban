package kanban

import (
	"context"
	"errors"
	"slices"
)

// DeleteCard 删除一张卡片，只允许人操作。卡片的评论一起删除，其他卡片依赖它的记录也一并清掉，
// 否则等待它的卡片会永远显示"等待中"。卡片编号不会回收，之后创建的卡片继续往后编。
// 删除是幂等的：卡片已经不存在也返回成功，并且不写事件。
func (c Core) DeleteCard(ctx context.Context, actor Actor, cardID int64) error {
	if err := requireHuman(actor); err != nil {
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
		return u.deleteCard(ctx, actor, card)
	})
}

// deleteCard 在事务里删除评论和变更记录、清理依赖、删除卡片本身，最后追加 card.deleted 事件。
func (u *unit) deleteCard(ctx context.Context, actor Actor, card *Card) error {
	texts, err := u.cardTexts(ctx, card)
	if err != nil {
		return err
	}
	if err := u.comment.DeleteByCard(ctx, card.ID); err != nil {
		return wrapDB("删除任务的评论", err)
	}
	if err := u.cardChange.DeleteByCard(ctx, card.ID); err != nil {
		return wrapDB("删除任务的变更记录", err)
	}
	if err := u.dropDependency(ctx, actor, card); err != nil {
		return err
	}
	if err := u.card.Delete(ctx, card); err != nil {
		return wrapDB("删除任务", err)
	}
	if err := u.releaseImages(ctx, card.BoardID, attachmentTokens(texts...)); err != nil {
		return err
	}
	return u.emit(ctx, actor, newEvent{
		BoardID: card.BoardID, CardID: card.ID, Type: EventCardDeleted, Payload: cardPayload(card, nil),
	})
}

// cardTexts 返回卡片描述和全部评论的正文，删除卡片前用它找出这张卡片引用过的图片。
func (u *unit) cardTexts(ctx context.Context, card *Card) ([]string, error) {
	comments, err := u.comment.ListByCard(ctx, card.ID)
	if err != nil {
		return nil, wrapDB("读取任务的评论", err)
	}
	texts := []string{card.Description}
	for _, c := range comments {
		texts = append(texts, c.Body)
	}
	return texts, nil
}

// dropDependency 把 card 从同看板其他卡片的依赖里移除，被改动的卡片版本加一。
// 这是删除卡片的连带清理，不单独写事件，界面和 agent 通过 card.deleted 就知道要刷新。
// 复杂度：一次读取看板的全部卡片，线性扫描。
func (u *unit) dropDependency(ctx context.Context, actor Actor, card *Card) error {
	cards, err := u.card.ListByBoard(ctx, card.BoardID)
	if err != nil {
		return wrapDB("读取看板的任务", err)
	}
	for _, other := range cards {
		i := slices.Index(other.WaitsOn, card.ID)
		if i < 0 {
			continue
		}
		other.WaitsOn = slices.Delete(other.WaitsOn, i, i+1)
		if err := u.saveCard(ctx, actor, other); err != nil {
			return err
		}
	}
	return nil
}
