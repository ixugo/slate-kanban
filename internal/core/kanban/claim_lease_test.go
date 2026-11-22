package kanban_test

import (
	"context"
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
