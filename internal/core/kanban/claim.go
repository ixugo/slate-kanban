package kanban

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	// defaultLeaseMinutes 领取的默认租约时长：到期前没有任何写入就视为 agent 放弃了。
	defaultLeaseMinutes = 30
	// minLeaseMinutes 租约最短时长，太短的租约来不及做任何事。
	minLeaseMinutes = 1
	// maxLeaseMinutes 租约最长时长（24 小时），防止任务被无限期占住。
	maxLeaseMinutes = 24 * 60
)

// 领取被放开的原因，写进 card.released 事件，让订阅者知道任务为什么被放开。
const (
	// releaseReleased 持有者主动释放。
	releaseReleased = "released"
	// releaseRevoked 人把领取收回。
	releaseRevoked = "revoked"
	// releaseMoved agent 把任务移动到了另一列。人移动任务不会放开领取。
	releaseMoved = "moved"
)

// normalizeLease 把调用方给的租约分钟数换成时长：0 取默认值，其余必须在最短和最长之间。
func normalizeLease(minutes int) (time.Duration, error) {
	if minutes == 0 {
		minutes = defaultLeaseMinutes
	}
	if minutes < minLeaseMinutes || minutes > maxLeaseMinutes {
		return 0, invalidf("租约时长必须在 %d 到 %d 分钟之间", minLeaseMinutes, maxLeaseMinutes)
	}
	return time.Duration(minutes) * time.Minute, nil
}

// claimActive 判断任务当前是否处在有效的领取中：有持有者并且租约还没到期。
// 租约过期后任务对其他 agent 重新开放，但在被别人接手之前原持有者仍然能写，
// 所以"有效领取"和"持有"是两个概念，这里只回答前者。
func claimActive(card *Card, now time.Time) bool {
	return card.ClaimedBy != "" && card.ClaimExpiresAt != nil && card.ClaimExpiresAt.After(now)
}

// checkClaimable 按固定顺序判断 agent 能不能领取这个任务，第一条不满足就返回对应的错误：
// 不是只能由人处理、没有未完成的依赖、没有被别的 agent 持有。哪一列的任务该不该领，
// 由列的提示词约定，这里不按列限制，审查 agent 才能领审查列的任务。
// ix 里必须有任务的全部依赖。
func checkClaimable(actor Actor, card *Card, ix *boardIndex) error {
	if err := checkCardUnarchived(card); err != nil {
		return err
	}
	if card.HumanOnly {
		return ErrCardHumanOnly
	}
	if waiting := unfinishedDeps(card, ix.cards, ix.doneCols); len(waiting) > 0 {
		keys := make([]string, len(waiting))
		for i, dep := range waiting {
			keys[i] = dep.Key
		}
		return ErrCardWaiting.WithMsg(fmt.Sprintf("任务在等待 %s 完成", strings.Join(keys, "、")))
	}
	if card.ClaimedBy != actor.Name && claimActive(card, ix.now) {
		return ErrCardClaimed.WithMsg(fmt.Sprintf("任务已被 %s 领取，租约在 %s 到期",
			card.ClaimedBy, card.ClaimExpiresAt.In(ix.loc).Format(time.RFC3339)))
	}
	return nil
}

// clearClaim 清除任务上的领取并追加 card.released 事件，调用方负责随后保存任务。
// 只有任务真的带着领取记录时才有动作；租约已经过期但还没人接手的记录也会清掉，
// 因为这张卡接下来要去别的地方了，原来的领取不再有意义。
func (u *unit) clearClaim(ctx context.Context, actor Actor, card *Card, why string) error {
	if card.ClaimedBy == "" {
		return nil
	}
	agent := card.ClaimedBy
	card.ClaimedBy, card.ClaimExpiresAt, card.ClaimEngineerID = "", nil, 0
	return u.emit(ctx, actor, newEvent{
		BoardID: card.BoardID, CardID: card.ID, Type: EventCardReleased,
		Payload: cardPayload(card, map[string]any{"agent": agent, "why": why}),
	})
}

// clearClaimOnMove 任务换列时决定要不要放开领取：只有 agent 移动才放开（持有者自己换列就是交接，
// 别的 agent 只能移动租约已过期的任务）；人移动不动领取，领取留给 agent 自己放开、租约到期或人明确收回，
// 免得人拖一下任务，agent 手里还在做的活就被悄悄收走。调用方负责随后保存任务。
func (u *unit) clearClaimOnMove(ctx context.Context, actor Actor, card *Card) error {
	if !actor.IsAgent() {
		return nil
	}
	return u.clearClaim(ctx, actor, card, releaseMoved)
}

// renewLease 在持有者对自己的任务有动作（改卡、写评论）时顺延租约，返回到期时间有没有变化，
// 调用方负责保存任务。顺延到"现在加默认租约时长"，但从不缩短：agent 领取时要的更长租约会被保留。
// 租约已经过期但还没人接手的持有者同样适用，这样它回来继续干活就自动续上了。
// 不是 agent、或者不是这张卡的持有者，什么都不做。
func (u *unit) renewLease(actor Actor, card *Card) bool {
	if !actor.IsAgent() || card.ClaimedBy != actor.Name {
		return false
	}
	renewed := u.now().Add(defaultLeaseMinutes * time.Minute).UTC()
	if card.ClaimExpiresAt != nil && !card.ClaimExpiresAt.Before(renewed) {
		return false
	}
	card.ClaimExpiresAt = &renewed
	return true
}
