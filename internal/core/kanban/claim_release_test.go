package kanban_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"slate-kanban/internal/core/kanban"
)

// TestReleaseCardByHolder 验证持有者主动释放：领取清空，版本加一，写 card.released 且原因是 released。
func TestReleaseCardByHolder(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	c := e.card(t, colDoing, "任务")
	if _, err := e.core.ClaimCard(ctx, testAgent, c.ID, kanban.ClaimCardInput{}); err != nil {
		t.Fatalf("领取失败: %v", err)
	}

	if err := e.core.ReleaseCard(ctx, testAgent, c.ID); err != nil {
		t.Fatalf("释放失败: %v", err)
	}
	got := e.stored(t, c.ID)
	if got.ClaimedBy != "" || got.ClaimExpiresAt != nil || got.Version != 3 || got.UpdatedBy != "claude" {
		t.Fatalf("释放后的状态不对: %+v", got)
	}
	want := `{"agent":"claude","key":"TST-1","title":"任务","why":"released"}`
	if ev := e.lastEvent(t); ev.Type != kanban.EventCardReleased || ev.CardID != c.ID || ev.Actor != "claude" || ev.Payload != want {
		t.Fatalf("事件不对: %+v，期望内容 %s", ev, want)
	}
}

// TestReleaseCardRevokedByHuman 验证人收回：原因是 revoked；原持有者随后再改这张卡得到 ErrNotHolder，知道自己已经不持有。
func TestReleaseCardRevokedByHuman(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	c := e.card(t, colDoing, "任务")
	if _, err := e.core.ClaimCard(ctx, testAgent, c.ID, kanban.ClaimCardInput{}); err != nil {
		t.Fatalf("领取失败: %v", err)
	}

	if err := e.core.ReleaseCard(ctx, testActor, c.ID); err != nil {
		t.Fatalf("收回失败: %v", err)
	}
	want := `{"agent":"claude","key":"TST-1","title":"任务","why":"revoked"}`
	if ev := e.lastEvent(t); ev.Type != kanban.EventCardReleased || ev.Actor != "admin" || ev.ActorKind != "human" || ev.Payload != want {
		t.Fatalf("事件不对: %+v，期望内容 %s", ev, want)
	}
	title := "agent 还想改"
	if _, err := e.core.UpdateCard(ctx, testAgent, c.ID, kanban.UpdateCardInput{Title: &title}); !errors.Is(err, kanban.ErrNotHolder) {
		t.Fatalf("被收回后再改应该得到 ErrNotHolder，实际 %v", err)
	}
}

// TestReleaseCardIsIdempotent 验证幂等：没被领取、任务不存在、重复释放都成功，并且不写事件。
func TestReleaseCardIsIdempotent(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	free := e.card(t, colDoing, "空闲")
	held := e.card(t, colDoing, "被领取")
	if _, err := e.core.ClaimCard(ctx, testAgent, held.ID, kanban.ClaimCardInput{}); err != nil {
		t.Fatalf("领取失败: %v", err)
	}
	if err := e.core.ReleaseCard(ctx, testAgent, held.ID); err != nil {
		t.Fatalf("第一次释放失败: %v", err)
	}
	events := len(e.events(t))

	for _, c := range []struct {
		actor kanban.Actor
		id    int64
	}{{testAgent, held.ID}, {testActor, held.ID}, {testAgent, free.ID}, {testActor, free.ID}, {testAgent, 9999}, {testActor, 9999}} {
		if err := e.core.ReleaseCard(ctx, c.actor, c.id); err != nil {
			t.Errorf("%s 释放 %d 应该成功: %v", c.actor.Name, c.id, err)
		}
	}
	if after := len(e.events(t)); after != events {
		t.Fatalf("幂等的释放不应该写事件，事件数 %d → %d", events, after)
	}
}

// TestReleaseCardByOtherAgent 验证别的 agent：对方租约有效时拒绝；对方租约已经过期时在它看来是空闲，什么都不做。
func TestReleaseCardByOtherAgent(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	c := e.card(t, colDoing, "任务")

	e.holdUntil(t, c.ID, "codex", claimNow.Add(time.Hour))
	if err := e.core.ReleaseCard(ctx, testAgent, c.ID); !errors.Is(err, kanban.ErrNotHolder) {
		t.Fatalf("对方租约有效应该得到 ErrNotHolder，实际 %v", err)
	}

	e.holdUntil(t, c.ID, "codex", claimNow.Add(-time.Hour))
	events := len(e.events(t))
	if err := e.core.ReleaseCard(ctx, testAgent, c.ID); err != nil {
		t.Fatalf("对方租约过期应该成功: %v", err)
	}
	if got := e.stored(t, c.ID); got.ClaimedBy != "codex" || len(e.events(t)) != events {
		t.Fatalf("别人过期的领取不应该被改动: %+v", got)
	}
}

// TestReleaseCardOfExpiredClaim 验证持有者释放自己已经过期的领取：仍然算持有，正常清除并写事件。
func TestReleaseCardOfExpiredClaim(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	c := e.card(t, colDoing, "任务")
	e.holdUntil(t, c.ID, "claude", claimNow.Add(-time.Hour))

	if err := e.core.ReleaseCard(ctx, testAgent, c.ID); err != nil {
		t.Fatalf("释放失败: %v", err)
	}
	if got := e.stored(t, c.ID); got.ClaimedBy != "" {
		t.Fatalf("领取应该已清除: %+v", got)
	}
	if ev := e.lastEvent(t); ev.Type != kanban.EventCardReleased {
		t.Fatalf("应该写 card.released: %+v", ev)
	}
}

// TestReleaseCardRejects 验证身份：自动化和身份无效的调用被拒绝。
func TestReleaseCardRejects(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	c := e.card(t, colDoing, "任务")
	if err := e.core.ReleaseCard(ctx, kanban.Actor{Kind: kanban.ActorSystem, Name: "automation"}, c.ID); !errors.Is(err, kanban.ErrHumanOnly) {
		t.Errorf("自动化应该得到 ErrHumanOnly，实际 %v", err)
	}
	if err := e.core.ReleaseCard(ctx, kanban.Actor{}, c.ID); !errors.Is(err, kanban.ErrInvalidActor) {
		t.Errorf("身份无效应该得到 ErrInvalidActor，实际 %v", err)
	}
}
