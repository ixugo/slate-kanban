package kanban

import "context"

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
