package kanban

import (
	"context"
	"time"
)

// 领取原因，写进 card.released 事件，让订阅者知道卡片为什么被放开。
const (
	// releaseMoved 卡片被移动到了另一列。
	releaseMoved = "moved"
)

// clearClaim 清除卡片上的领取并追加 card.released 事件，调用方负责随后保存卡片。
// 只有卡片真的带着领取记录时才有动作；租约已经过期但还没人接手的记录也会清掉，
// 因为这张卡接下来要去别的地方了，原来的领取不再有意义。
func (u *unit) clearClaim(ctx context.Context, actor Actor, card *Card, why string) error {
	if card.ClaimedBy == "" {
		return nil
	}
	agent := card.ClaimedBy
	card.ClaimedBy, card.ClaimExpiresAt = "", nil
	return u.emit(ctx, actor, newEvent{
		BoardID: card.BoardID, CardID: card.ID, Type: EventCardReleased,
		Payload: cardPayload(card, map[string]any{"agent": agent, "why": why}),
	})
}

// claimActive 判断卡片当前是否处在有效的领取中：有持有者并且租约还没到期。
// 租约过期后卡片对其他 agent 重新开放，但在被别人接手之前原持有者仍然能写，
// 所以"有效领取"和"持有"是两个概念，这里只回答前者。
func claimActive(card *Card, now time.Time) bool {
	return card.ClaimedBy != "" && card.ClaimExpiresAt != nil && card.ClaimExpiresAt.After(now)
}
