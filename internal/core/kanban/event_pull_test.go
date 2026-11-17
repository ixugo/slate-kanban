package kanban_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"slate-kanban/internal/core/kanban"

	"github.com/ixugo/goddd/pkg/orm"
)

// pull 以 agent 的身份拉取更新，失败就让测试终止。
func (e *cardEnv) pull(t *testing.T, agent kanban.Actor, since *int64, limit int) *kanban.Updates {
	t.Helper()
	out, err := e.core.PullUpdates(context.Background(), agent, e.board.ID, kanban.PullInput{Since: since, Limit: limit})
	if err != nil {
		t.Fatalf("%s 拉取更新失败: %v", agent.Name, err)
	}
	return out
}

// emitRaw 直接往看板追加一条事件，用来构造排序类这种正常操作很难单独产生的事件。
func (e *cardEnv) emitRaw(t *testing.T, actor kanban.Actor, eventType string) {
	t.Helper()
	err := e.core.RunForTest(context.Background(), e.board.ID, func(u *kanban.Unit) error {
		return u.EmitForTest(context.Background(), actor, kanban.NewEvent{BoardID: e.board.ID, Type: eventType})
	})
	if err != nil {
		t.Fatalf("追加事件 %s 失败: %v", eventType, err)
	}
}

// ptr64 返回整数的指针，用来填 Since。
func ptr64(n int64) *int64 { return &n }

// storedCursor 直接读出 agent 在数据库里的进度。
func (e *cardEnv) storedCursor(t *testing.T, agent string) int64 {
	t.Helper()
	cur, err := e.db.Cursor().Get(context.Background(), e.board.ID, agent)
	if err != nil {
		t.Fatalf("读取 %s 的进度失败: %v", agent, err)
	}
	return cur.LastSeq
}

// TestPullFirstTimeStartsAtLatest 验证第一次读一个看板：不补发历史，进度直接设在最新处并标记 first，
// 之后的新事件正常送达。
func TestPullFirstTimeStartsAtLatest(t *testing.T) {
	e := newCardEnv(t)
	e.card(t, colTodo, "历史卡片 A")
	e.card(t, colTodo, "历史卡片 B")
	latest := e.lastEvent(t).ID

	first := e.pull(t, testAgent, nil, 0)
	if !first.First || len(first.Events) != 0 || first.Events == nil || first.Cursor != latest || first.Latest != latest {
		t.Fatalf("第一次应该是空数组、first、进度在最新处: %+v", first)
	}
	if got := e.storedCursor(t, testAgent.Name); got != latest {
		t.Fatalf("进度应该已经记下: %d，期望 %d", got, latest)
	}

	c := e.card(t, colTodo, "新卡片 C")
	next := e.pull(t, testAgent, nil, 0)
	if next.First || len(next.Events) != 1 || next.Events[0].CardID != c.ID || next.Events[0].Type != kanban.EventCardCreated {
		t.Fatalf("第二次应该只收到新事件: %+v", next)
	}
	if next.Cursor != e.lastEvent(t).ID || next.HasMore || next.Truncated {
		t.Fatalf("进度和标记不对: %+v", next)
	}
}

// TestPullRedeliversUntilAcknowledged 验证确认语义：只有下一次调用带上 Since 才推进进度，
// 所以返回的结果丢了，再拉一次还是同样的事件。
func TestPullRedeliversUntilAcknowledged(t *testing.T) {
	e := newCardEnv(t)
	e.pull(t, testAgent, nil, 0)
	for _, title := range []string{"A", "B", "C"} {
		e.card(t, colTodo, title)
	}

	got := e.pull(t, testAgent, nil, 0)
	if len(got.Events) != 3 {
		t.Fatalf("应该收到 3 条: %+v", got)
	}
	again := e.pull(t, testAgent, nil, 0)
	if !slices.Equal(eventSeqs(again.Events), eventSeqs(got.Events)) {
		t.Fatalf("没确认就再拉，应该收到同样的事件: %v 和 %v", eventSeqs(again.Events), eventSeqs(got.Events))
	}
	acked := e.pull(t, testAgent, ptr64(got.Cursor), 0)
	if len(acked.Events) != 0 || acked.Cursor != got.Cursor {
		t.Fatalf("确认之后应该没有新事件: %+v", acked)
	}
	if after := e.pull(t, testAgent, nil, 0); len(after.Events) != 0 {
		t.Fatalf("确认之后不带 Since 也不应该再收到旧事件: %+v", after)
	}
}

// TestPullFiltersOwnAndQuietEvents 验证过滤：agent 自己做的操作和排序类、自动化流水类事件不送达，
// 但进度照样走过它们；别的 agent 做的操作要送达。
func TestPullFiltersOwnAndQuietEvents(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	e.pull(t, testAgent, nil, 0)

	card := e.card(t, colTodo, "X")
	if _, err := e.core.AddComment(ctx, testAgent, card.ID, "我自己评论的"); err != nil {
		t.Fatalf("agent 评论失败: %v", err)
	}
	if _, err := e.core.AddComment(ctx, codex, card.ID, "另一个 agent 评论的"); err != nil {
		t.Fatalf("另一个 agent 评论失败: %v", err)
	}
	e.emitRaw(t, testActor, kanban.EventCardReordered)
	e.emitRaw(t, testActor, kanban.EventColumnReordered)
	e.emitRaw(t, kanban.AutomationActor("起步"), kanban.EventAutomationFired)

	got := e.pull(t, testAgent, nil, 0)
	if len(got.Events) != 2 || got.Events[0].Type != kanban.EventCardCreated || got.Events[1].ActorKind != "agent" || got.Events[1].Actor != codex.Name {
		t.Fatalf("应该只收到人建卡和别的 agent 的评论: %+v", got.Events)
	}
	if got.Cursor != e.lastEvent(t).ID || got.Cursor != got.Latest {
		t.Fatalf("进度应该走过被过滤的事件: %+v", got)
	}
	if again := e.pull(t, testAgent, ptr64(got.Cursor), 0); len(again.Events) != 0 {
		t.Fatalf("确认后不应该再有事件: %+v", again)
	}
}

// TestPullLimitAndHasMore 验证分页：limit 截断时 has_more 为真，用返回的进度当 Since 逐页读完；
// 整页都被过滤掉时事件为空，但进度前进、has_more 仍然为真，调用方应该继续拉。
func TestPullLimitAndHasMore(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	e.pull(t, testAgent, nil, 0)
	var created []int64
	for _, title := range []string{"A", "B", "C", "D", "E"} {
		created = append(created, e.card(t, colTodo, title).ID)
	}

	var got []int64
	var since *int64
	for page := 1; ; page++ {
		out := e.pull(t, testAgent, since, 2)
		for _, ev := range out.Events {
			got = append(got, ev.CardID)
		}
		if page > 3 {
			t.Fatalf("5 条事件每页 2 条，最多 3 页")
		}
		if !out.HasMore {
			break
		}
		since = ptr64(out.Cursor)
	}
	if len(got) != 5 || got[0] != created[0] || got[4] != created[4] {
		t.Fatalf("分页应该不重不漏地读完: %v，期望 %v", got, created)
	}

	e.pull(t, testAgent, ptr64(e.lastEvent(t).ID), 0)
	for range 3 {
		if _, err := e.core.AddComment(ctx, testAgent, created[0], "自己的评论"); err != nil {
			t.Fatalf("评论失败: %v", err)
		}
	}
	filtered := e.pull(t, testAgent, nil, 2)
	if len(filtered.Events) != 0 || !filtered.HasMore || filtered.Cursor == e.lastEvent(t).ID {
		t.Fatalf("整页被过滤时应该空事件、有更多、进度前进但没到头: %+v", filtered)
	}
	rest := e.pull(t, testAgent, ptr64(filtered.Cursor), 2)
	if len(rest.Events) != 0 || rest.HasMore || rest.Cursor != e.lastEvent(t).ID {
		t.Fatalf("剩下的也是自己的事件: %+v", rest)
	}
}

// TestPullSinceRules 验证 Since 的规则：只增不减，比已有进度小的被忽略；
// 超过最新序号的截到最新序号，不会把以后的新事件跳过去；负数返回 400。
func TestPullSinceRules(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	base := e.pull(t, testAgent, nil, 0).Cursor
	e.card(t, colTodo, "A")
	e.card(t, colTodo, "B")
	second := e.lastEvent(t).ID

	if got := e.pull(t, testAgent, ptr64(base+1), 0); len(got.Events) != 1 || got.Events[0].ID != second {
		t.Fatalf("确认到第一条后只剩第二条: %+v", got)
	}
	if got := e.pull(t, testAgent, ptr64(0), 0); len(got.Events) != 1 || e.storedCursor(t, testAgent.Name) != base+1 {
		t.Fatalf("比已有进度小的 Since 应该被忽略: %+v 进度 %d", got, e.storedCursor(t, testAgent.Name))
	}
	if got := e.pull(t, testAgent, ptr64(1<<40), 0); len(got.Events) != 0 || got.Cursor != second || e.storedCursor(t, testAgent.Name) != second {
		t.Fatalf("超大的 Since 应该截到最新序号: %+v", got)
	}
	e.card(t, colTodo, "C")
	if got := e.pull(t, testAgent, nil, 0); len(got.Events) != 1 {
		t.Fatalf("截断之后的新事件不应该被跳过: %+v", got)
	}
	if _, err := e.core.PullUpdates(ctx, testAgent, e.board.ID, kanban.PullInput{Since: ptr64(-1)}); !errors.Is(err, kanban.ErrInvalidInput) {
		t.Fatalf("负数 Since 应该返回 400: %v", err)
	}
	if _, err := e.core.PullUpdates(ctx, testAgent, e.board.ID, kanban.PullInput{Limit: -1}); !errors.Is(err, kanban.ErrInvalidInput) {
		t.Fatalf("负数 Limit 应该返回 400: %v", err)
	}
}

// TestPullCursorsAreIndependent 验证每个 agent、每个看板各有一份进度：
// 一个 agent 确认不影响另一个，在别的看板上读也不影响这个看板。
func TestPullCursorsAreIndependent(t *testing.T) {
	e := newCardEnv(t)
	other := newCardEnvOn(t, e.core, e.db, "OTH")
	e.pull(t, testAgent, nil, 0)
	e.pull(t, codex, nil, 0)
	other.pull(t, testAgent, nil, 0)
	old := e.lastEvent(t).ID
	e.card(t, colTodo, "A")
	other.card(t, colTodo, "别的看板的")

	if got := e.pull(t, testAgent, ptr64(e.lastEvent(t).ID), 0); len(got.Events) != 0 {
		t.Fatalf("确认之后不应该有事件: %+v", got)
	}
	if got := e.pull(t, codex, nil, 0); len(got.Events) != 1 || got.Events[0].BoardID != e.board.ID {
		t.Fatalf("codex 没确认，应该还能收到这个看板的事件，并且没有别的看板的: %+v", got)
	}
	if e.storedCursor(t, codex.Name) != old {
		t.Fatalf("codex 的进度不应该被 claude 的确认推进")
	}
	if got := other.pull(t, testAgent, nil, 0); len(got.Events) != 1 || got.Events[0].BoardID != other.board.ID {
		t.Fatalf("别的看板的进度独立，应该只收到它自己的事件: %+v", got)
	}
}

// TestPullTruncated 验证离线太久：事件被清理后，进度落在保留范围之前，
// 返回 truncated 并从保留范围的开头给出剩下的全部事件；确认之后恢复正常。
func TestPullTruncated(t *testing.T) {
	e := newCardEnv(t, kanban.WithRetentionForTest(5, 1))
	e.pull(t, testAgent, nil, 0)
	for _, title := range []string{"A", "B", "C", "D", "E", "F", "G", "H"} {
		e.card(t, colTodo, title)
	}
	kept := e.events(t)

	got := e.pull(t, testAgent, nil, 0)
	if !got.Truncated || !slices.Equal(eventSeqs(got.Events), eventSeqs(kept)) || got.Cursor != kept[len(kept)-1].ID {
		t.Fatalf("应该标记 truncated 并给出保留的全部事件: %+v，保留 %v", got, eventSeqs(kept))
	}
	if again := e.pull(t, testAgent, ptr64(got.Cursor), 0); again.Truncated || len(again.Events) != 0 {
		t.Fatalf("确认之后应该恢复正常: %+v", again)
	}
}

// TestPullRejects 验证拒绝的情况：人和系统不能拉取，身份不合法返回 401，
// 看板不存在返回 404，并且不会为不存在的看板留下进度记录。
func TestPullRejects(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	cases := map[string]struct {
		actor   kanban.Actor
		boardID int64
		want    error
	}{
		"人":     {testActor, e.board.ID, kanban.ErrAgentOnly},
		"系统":    {kanban.AutomationActor("规则"), e.board.ID, kanban.ErrAgentOnly},
		"身份不合法": {kanban.Actor{}, e.board.ID, kanban.ErrInvalidActor},
		"看板不存在": {testAgent, 9999, kanban.ErrBoardNotFound},
	}
	for name, c := range cases {
		if _, err := e.core.PullUpdates(ctx, c.actor, c.boardID, kanban.PullInput{}); !errors.Is(err, c.want) {
			t.Errorf("%s: 期望 %v，实际 %v", name, c.want, err)
		}
	}
	if _, err := e.db.Cursor().Get(ctx, 9999, testAgent.Name); !orm.IsErrRecordNotFound(err) {
		t.Errorf("不存在的看板不应该留下进度记录: %v", err)
	}
}
