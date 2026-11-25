package kanban_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"slate-kanban/internal/core/kanban"
)

// claimedFor 让 testAgent 用指定的租约分钟数领取一个进行中的新任务，返回任务主键。
func (e *cardEnv) claimedFor(t *testing.T, minutes int) int64 {
	t.Helper()
	c := e.card(t, colDoing, "任务")
	if _, err := e.core.ClaimCard(context.Background(), testAgent, c.ID, kanban.ClaimCardInput{LeaseMinutes: minutes}); err != nil {
		t.Fatalf("领取失败: %v", err)
	}
	return c.ID
}

// expiry 读出任务当前的租约到期时间，没有则让测试失败。
func (e *cardEnv) expiry(t *testing.T, id int64) time.Time {
	t.Helper()
	c := e.stored(t, id)
	if c.ClaimExpiresAt == nil {
		t.Fatalf("任务 %d 没有租约", id)
	}
	return *c.ClaimExpiresAt
}

// TestHolderUpdateRenewsLease 验证持有者改卡会顺延租约：比默认时长短的租约顺延到"现在加 30 分钟"，
// 更长的租约保持不变（从不缩短）；租约已经过期但仍是持有者时，也会续上。
func TestHolderUpdateRenewsLease(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	short := e.claimedFor(t, 5)
	long := e.claimedFor(t, 120)
	expired := e.claimedFor(t, 5)
	e.holdUntil(t, expired, "claude", claimNow.Add(-time.Hour))
	title := "新标题"

	for _, id := range []int64{short, long, expired} {
		if _, err := e.core.UpdateCard(ctx, testAgent, id, kanban.UpdateCardInput{Title: &title}); err != nil {
			t.Fatalf("持有者改卡失败: %v", err)
		}
	}
	if got := e.expiry(t, short); !got.Equal(claimNow.Add(30 * time.Minute)) {
		t.Errorf("短租约应该顺延到 30 分钟后，实际 %v", got)
	}
	if got := e.expiry(t, long); !got.Equal(claimNow.Add(120 * time.Minute)) {
		t.Errorf("长租约不应该被缩短，实际 %v", got)
	}
	if got := e.expiry(t, expired); !got.Equal(claimNow.Add(30 * time.Minute)) {
		t.Errorf("已过期的持有者继续干活应该续上，实际 %v", got)
	}
}

// TestUpdateWithoutChangeDoesNotRenew 验证没有真实修改不算动作，不续租；人改卡也不会续 agent 的租约。
func TestUpdateWithoutChangeDoesNotRenew(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	id := e.claimedFor(t, 5)
	before := e.expiry(t, id)
	same, human := "任务", "人改的标题"

	if _, err := e.core.UpdateCard(ctx, testAgent, id, kanban.UpdateCardInput{Title: &same}); err != nil {
		t.Fatalf("空修改失败: %v", err)
	}
	if got := e.expiry(t, id); !got.Equal(before) {
		t.Errorf("没有变化不应该续租: %v → %v", before, got)
	}
	if _, err := e.core.UpdateCard(ctx, testActor, id, kanban.UpdateCardInput{Title: &human}); err != nil {
		t.Fatalf("人改卡失败: %v", err)
	}
	if got := e.expiry(t, id); !got.Equal(before) {
		t.Errorf("人改卡不应该续 agent 的租约: %v → %v", before, got)
	}
}

// TestHolderCommentRenewsLease 验证持有者写评论会顺延租约，但不涨版本号，也不多写事件；
// 人或别的 agent 写评论不影响租约。
func TestHolderCommentRenewsLease(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	id := e.claimedFor(t, 5)
	before := e.expiry(t, id)

	for _, actor := range []kanban.Actor{testActor, codex} {
		if _, err := e.core.AddComment(ctx, actor, id, "路过"); err != nil {
			t.Fatalf("%s 写评论失败: %v", actor.Name, err)
		}
	}
	if got := e.expiry(t, id); !got.Equal(before) {
		t.Fatalf("别人写评论不应该续租: %v → %v", before, got)
	}

	events := len(e.events(t))
	if _, err := e.core.AddComment(ctx, testAgent, id, "进展汇报"); err != nil {
		t.Fatalf("持有者写评论失败: %v", err)
	}
	got := e.stored(t, id)
	if !got.ClaimExpiresAt.Equal(claimNow.Add(30*time.Minute)) || got.Version != 2 {
		t.Fatalf("持有者写评论应该续租且不涨版本: %+v", got)
	}
	if after := e.events(t); len(after) != events+1 || after[events].Type != kanban.EventCommentAdded {
		t.Fatalf("只应该多一条 comment.added: %+v", after[events:])
	}
}

// eventTypes 取出一批事件的类型，按写入顺序排列，用来核对一次操作依次写了哪些事件。
func eventTypes(events []*kanban.Event) []string {
	types := make([]string, len(events))
	for i, ev := range events {
		types[i] = ev.Type
	}
	return types
}

// 租约到期后没人接手，原持有者回来继续写入：租约没有续上过，这是一次新的领取。
// 先记租约到期，再记新的领取，最后才是评论或修改本身；%s 是事件发生时任务的标题。
const (
	wantLapsedPayload  = `{"agent":"claude","expired_at":"2026-11-17T11:00:00Z","key":"TST-1","title":"%s","why":"expired"}`
	wantRestartPayload = `{"agent":"claude","engineer_id":0,"expires_at":"2026-11-17T12:30:00Z","key":"TST-1","title":"%s"}`
)

// TestHolderCommentAfterExpiryRecordsNewClaim 验证租约已经到期的持有者写评论：
// 事件依次是租约到期、新的领取、评论；版本加一，租约从现在起重新算 30 分钟。
func TestHolderCommentAfterExpiryRecordsNewClaim(t *testing.T) {
	ctx := t.Context()
	e := newClaimEnv(t)
	c := e.card(t, colDoing, "任务")
	e.holdUntil(t, c.ID, testAgent.Name, claimNow.Add(-time.Hour))
	events := len(e.events(t))

	if _, err := e.core.AddComment(ctx, testAgent, c.ID, "回来继续"); err != nil {
		t.Fatalf("租约到期的持有者写评论失败: %v", err)
	}
	got := e.stored(t, c.ID)
	if !got.ClaimExpiresAt.Equal(claimNow.Add(30*time.Minute)) || got.Version != c.Version+1 {
		t.Fatalf("应该从现在起重新算 30 分钟租约，并按新领取涨版本: %+v", got)
	}
	after := e.events(t)[events:]
	want := []string{kanban.EventCardReleased, kanban.EventCardClaimed, kanban.EventCommentAdded}
	if types := eventTypes(after); !slices.Equal(types, want) {
		t.Fatalf("事件顺序应该是 %v，实际 %v", want, types)
	}
	if after[0].Payload != fmt.Sprintf(wantLapsedPayload, "任务") || after[1].Payload != fmt.Sprintf(wantRestartPayload, "任务") {
		t.Fatalf("租约到期或新领取的内容不对: %s / %s", after[0].Payload, after[1].Payload)
	}
	for _, ev := range after {
		if ev.CardID != c.ID || ev.Actor != testAgent.Name {
			t.Fatalf("事件应该记在持有者名下: %+v", ev)
		}
	}
}

// TestHolderUpdateAfterExpiryRecordsNewClaim 验证租约已经到期的持有者改卡：
// 事件依次是租约到期、新的领取、card.updated；改卡本来就涨一次版本，不会因为重新领取多涨。
func TestHolderUpdateAfterExpiryRecordsNewClaim(t *testing.T) {
	ctx := t.Context()
	e := newClaimEnv(t)
	c := e.card(t, colDoing, "任务")
	e.holdUntil(t, c.ID, testAgent.Name, claimNow.Add(-time.Hour))
	events := len(e.events(t))
	title := "新标题"

	if _, err := e.core.UpdateCard(ctx, testAgent, c.ID, kanban.UpdateCardInput{Title: &title}); err != nil {
		t.Fatalf("租约到期的持有者改卡失败: %v", err)
	}
	got := e.stored(t, c.ID)
	if !got.ClaimExpiresAt.Equal(claimNow.Add(30*time.Minute)) || got.Version != c.Version+1 {
		t.Fatalf("应该重新算 30 分钟租约，版本只涨一次: %+v", got)
	}
	after := e.events(t)[events:]
	want := []string{kanban.EventCardReleased, kanban.EventCardClaimed, kanban.EventCardUpdated}
	if types := eventTypes(after); !slices.Equal(types, want) {
		t.Fatalf("事件顺序应该是 %v，实际 %v", want, types)
	}
	if after[0].Payload != fmt.Sprintf(wantLapsedPayload, title) || after[1].Payload != fmt.Sprintf(wantRestartPayload, title) {
		t.Fatalf("租约到期或新领取的内容不对: %s / %s", after[0].Payload, after[1].Payload)
	}
}

// TestHolderUpdateWithinLeaseWritesNoClaimEvents 验证租约一直有效时，持有者改卡只顺延到期时间：
// 不写租约到期，也不写新的领取，活动里只有这次修改本身。
func TestHolderUpdateWithinLeaseWritesNoClaimEvents(t *testing.T) {
	ctx := t.Context()
	e := newClaimEnv(t)
	id := e.claimedFor(t, 5)
	events := len(e.events(t))
	title := "新标题"

	if _, err := e.core.UpdateCard(ctx, testAgent, id, kanban.UpdateCardInput{Title: &title}); err != nil {
		t.Fatalf("持有者改卡失败: %v", err)
	}
	if types := eventTypes(e.events(t)[events:]); !slices.Equal(types, []string{kanban.EventCardUpdated}) {
		t.Fatalf("租约有效时只应该写 card.updated，实际 %v", types)
	}
}

// TestHolderCommentAfterExpiryKeepsEngineer 验证带工程师名额的领取在到期后被续上时，
// 新的领取事件沿用原来的工程师编号，名称留空（和普通 agent 的新领取一样，由事件的 actor 决定显示名）。
func TestHolderCommentAfterExpiryKeepsEngineer(t *testing.T) {
	ctx := t.Context()
	e := newClaimEnv(t)
	c := e.card(t, colDoing, "任务")
	stored := e.stored(t, c.ID)
	expired := claimNow.Add(-time.Hour)
	stored.ClaimedBy, stored.ClaimExpiresAt, stored.ClaimEngineerID = testAgent.Name, &expired, 7
	if err := e.db.Card().Save(ctx, stored); err != nil {
		t.Fatal(err)
	}
	events := len(e.events(t))

	if _, err := e.core.AddComment(ctx, testAgent, c.ID, "回来继续"); err != nil {
		t.Fatal(err)
	}
	want := `{"agent":"claude","engineer_id":7,"engineer_name":"","expires_at":"2026-11-17T12:30:00Z","key":"TST-1","title":"任务"}`
	after := e.events(t)[events:]
	if len(after) < 2 || after[1].Type != kanban.EventCardClaimed || after[1].Payload != want {
		t.Fatalf("第二条应该是带原来工程师编号的新领取，实际 %v，期望内容 %s", after, want)
	}
	if got := e.stored(t, c.ID); got.ClaimEngineerID != 7 {
		t.Fatalf("工程师编号不应该丢: %+v", got)
	}
}
