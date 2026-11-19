package kanban

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"time"
)

// engineerPool 是领取时用到的工程师名册快照：每位工程师的类型，以及此刻谁正占着哪张卡。
// 看板没有招募任何工程师时 pool 为 nil，领取不涉及工程师，行为和以前一样。
type engineerPool struct {
	all  []*Engineer
	busy map[int64]int64 // 工程师主键 → 他正在持有的卡片主键（租约还有效的）
}

// newEngineerPool 用名册和看板的全部卡片建快照，名册为空返回 nil。
// 复杂度 O(卡片数)，只在领取时算一次。
func newEngineerPool(all []*Engineer, cards []*Card, now time.Time) *engineerPool {
	if len(all) == 0 {
		return nil
	}
	p := &engineerPool{all: all, busy: make(map[int64]int64)}
	for _, c := range cards {
		if c.ClaimEngineerID != 0 && claimActive(c, now) {
			p.busy[c.ClaimEngineerID] = c.ID
		}
	}
	return p
}

// loadEngineerPool 在事务里读出名册，建领取用的快照；看板没有工程师时返回 nil。
// cards 是调用方已经读出的看板全部卡片，传 nil 时这里再读一次。
func (u *unit) loadEngineerPool(ctx context.Context, boardID int64, cards []*Card) (*engineerPool, error) {
	all, err := u.engineer.ListByBoard(ctx, boardID)
	if err != nil {
		return nil, wrapDB("读取工程师名册", err)
	}
	if len(all) == 0 {
		return nil, nil
	}
	if cards == nil {
		if cards, err = u.card.ListByBoard(ctx, boardID); err != nil {
			return nil, wrapDB("读取卡片", err)
		}
	}
	return newEngineerPool(all, cards, u.now()), nil
}

// find 按主键找工程师，找不到返回 nil。
func (p *engineerPool) find(id int64) *Engineer {
	if i := slices.IndexFunc(p.all, func(e *Engineer) bool { return e.ID == id }); i >= 0 {
		return p.all[i]
	}
	return nil
}

// free 判断工程师能不能接这张卡：没有在做别的卡，或者在做的就是这一张（续租）。
func (p *engineerPool) free(id, cardID int64) bool {
	holding, ok := p.busy[id]
	return !ok || holding == cardID
}

// engineerFor 决定这次领取由哪位工程师承担，返回他的主键；0 表示不涉及工程师。
// 优先级：调用方指定的 requested → 卡片上已绑定的研发工程师（研发时）→ 随机一位空闲的同类型工程师。
// 指定工程师的类型由 agent 按列提示词选择，服务端不从列推断；他必须在名册且空闲，研发领取还要符合卡片绑定；
// 同类型的工程师全在忙时返回 ErrNoIdleEngineer，调用方应当排队等一等。
// 看板里没有这种类型的工程师（例如简单模板没有测试）时不要求工程师。
func (p *engineerPool) engineerFor(card *Card, requested int64) (int64, error) {
	if p == nil {
		return 0, nil
	}
	kind := EngineerDev
	if requested != 0 {
		if engineer := p.find(requested); engineer != nil {
			kind = stageOf(engineer.Kind)
		}
	}
	bound := int64(0)
	if kind == EngineerDev && card.EngineerID != 0 && p.find(card.EngineerID) != nil {
		bound = card.EngineerID
	}
	switch {
	case requested != 0:
		return p.checkChoice(card, kind, requested, bound)
	case bound != 0:
		return p.checkChoice(card, kind, bound, 0)
	}
	return p.randomFree(card, kind)
}

// checkChoice 校验一个明确选定的工程师：存在、类型对、没被别的卡占着、没有和卡片的绑定冲突。
func (p *engineerPool) checkChoice(card *Card, kind string, id, bound int64) (int64, error) {
	e := p.find(id)
	if e == nil {
		return 0, invalidf("工程师 %d 不存在，或者不属于这个看板", id)
	}
	if stageOf(e.Kind) != kind {
		return 0, invalidf("%s 的类型不符：这张卡现在要 %s 工程师", e.Name, kindLabel(kind))
	}
	if bound != 0 && bound != id {
		return 0, ErrEngineerBound.WithMsg(fmt.Sprintf("卡片绑定了 %s，只能由他承担", p.find(bound).Name))
	}
	if !p.free(id, card.ID) {
		return 0, ErrEngineerBusy.WithMsg(fmt.Sprintf("%s 正在做别的卡片，换一位空闲的工程师，或者排队等一等", e.Name))
	}
	return id, nil
}

// randomFree 在同类型且空闲的工程师里随机挑一位。没有这种类型的工程师返回 0，全在忙返回 ErrNoIdleEngineer。
func (p *engineerPool) randomFree(card *Card, kind string) (int64, error) {
	var same, idle []*Engineer
	for _, e := range p.all {
		if stageOf(e.Kind) != kind {
			continue
		}
		same = append(same, e)
		if p.free(e.ID, card.ID) {
			idle = append(idle, e)
		}
	}
	switch {
	case len(same) == 0:
		return 0, nil
	case len(idle) == 0:
		return 0, ErrNoIdleEngineer.WithMsg(fmt.Sprintf("所有%s工程师都在忙，排队等一等", kindLabel(kind)))
	}
	return idle[rand.IntN(len(idle))].ID, nil
}

// kindLabel 返回工程师阶段的中文叫法（研发或测试），只用在错误提示里。
func kindLabel(kind string) string {
	if kind == EngineerReview {
		return "测试"
	}
	return "研发"
}

// checkHolder 在卡片的租约还有效时核对持有者：名字不同，或者指定的工程师和现在持有的不同，都算别人持有。
// 同一个默认名字下连着多个 agent 时，靠工程师把它们区分开。engineerID 为 0（看板没有工程师）时只比名字。
func checkHolder(actor Actor, card *Card, engineerID int64, now time.Time, loc *time.Location) error {
	if !claimActive(card, now) {
		return nil
	}
	if card.ClaimedBy == actor.Name && (engineerID == 0 || card.ClaimEngineerID == engineerID) {
		return nil
	}
	return ErrCardClaimed.WithMsg(fmt.Sprintf("卡片已被 %s 领取，租约在 %s 到期",
		card.ClaimedBy, card.ClaimExpiresAt.In(loc).Format(time.RFC3339)))
}
