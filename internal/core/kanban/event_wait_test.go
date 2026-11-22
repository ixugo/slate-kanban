package kanban_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"slate-kanban/internal/core/kanban"
)

// longWait 是测试里"足够长"的等待时间：功能正常时远没到就返回，卡住了才会走到它，让测试失败而不是挂死。
const longWait = 5 * time.Second

// wait 以 agent 的身份阻塞等待更新，失败就让测试终止。
func (e *cardEnv) wait(t *testing.T, agent kanban.Actor, since *int64, limit int, timeout time.Duration) *kanban.Updates {
	t.Helper()
	in := kanban.WaitInput{PullInput: kanban.PullInput{Since: since, Limit: limit}, Timeout: timeout}
	out, err := e.core.WaitUpdates(context.Background(), agent, e.board.ID, in)
	if err != nil {
		t.Fatalf("%s 等待更新失败: %v", agent.Name, err)
	}
	return out
}

// later 在 delay 之后执行 fn，用来在等待进行的同时制造更新。fn 里不能调用 t.Fatal，出错用 t.Error。
func later(delay time.Duration, fn func()) {
	time.AfterFunc(delay, fn)
}

// TestWaitReturnsImmediatelyWhenPending 验证已经有待读事件时不等待：马上返回，也不会标记超时。
func TestWaitReturnsImmediatelyWhenPending(t *testing.T) {
	e := newCardEnv(t)
	e.pull(t, testAgent, nil, 0)
	c := e.card(t, colTodo, "A")

	got := e.wait(t, testAgent, nil, 0, longWait)
	if got.TimedOut || len(got.Events) != 1 || got.Events[0].CardID != c.ID {
		t.Fatalf("应该马上收到已有的事件: %+v", got)
	}
}

// TestWaitFirstReturnsImmediately 验证第一次读一个看板时不等待，马上告诉调用方 first。
func TestWaitFirstReturnsImmediately(t *testing.T) {
	e := newCardEnv(t)
	got := e.wait(t, testAgent, nil, 0, longWait)
	if !got.First || got.TimedOut || len(got.Events) != 0 {
		t.Fatalf("第一次应该马上返回 first: %+v", got)
	}
}

// TestWaitWakesOnNewEvent 验证阻塞等待：等待期间别人建了任务，等待立刻醒来并带回这条事件。
func TestWaitWakesOnNewEvent(t *testing.T) {
	e := newCardEnv(t)
	e.pull(t, testAgent, nil, 0)
	created := make(chan *kanban.Card, 1)
	later(100*time.Millisecond, func() {
		c, err := e.core.CreateCard(context.Background(), testActor, e.board.ID, kanban.CreateCardInput{Title: "等待中新建", ColumnID: e.col(colTodo)})
		if err != nil {
			t.Errorf("创建任务失败: %v", err)
		}
		created <- c
	})

	got := e.wait(t, testAgent, nil, 0, longWait)
	c := <-created
	if c == nil || got.TimedOut || len(got.Events) != 1 || got.Events[0].CardID != c.ID {
		t.Fatalf("应该被新事件唤醒: %+v", got)
	}
	if got.Cursor != e.lastEvent(t).ID {
		t.Fatalf("进度应该在最新事件上: %+v", got)
	}
}

// TestWaitTimesOut 验证超时：到时间没有事件就返回空结果并标记超时，不是错误，
// 进度还是原来的，等待本身不确认任何东西。
func TestWaitTimesOut(t *testing.T) {
	e := newCardEnv(t)
	base := e.pull(t, testAgent, nil, 0).Cursor

	start := time.Now()
	got := e.wait(t, testAgent, nil, 0, 80*time.Millisecond)
	if elapsed := time.Since(start); elapsed < 60*time.Millisecond {
		t.Fatalf("应该等到超时才返回，只等了 %v", elapsed)
	}
	if !got.TimedOut || got.Events == nil || len(got.Events) != 0 || got.Cursor != base {
		t.Fatalf("超时应该返回空数组、标记超时、进度不变: %+v", got)
	}
	if e.storedCursor(t, testAgent.Name) != base {
		t.Fatalf("等待不应该推进进度")
	}
}

// TestWaitIgnoresOwnWakeups 验证过滤后的唤醒：agent 自己的操作也会唤醒等待，
// 但它被过滤掉了，等待要继续，直到别人的事件出现才返回。
func TestWaitIgnoresOwnWakeups(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	card := e.card(t, colTodo, "X")
	e.pull(t, testAgent, nil, 0)
	later(50*time.Millisecond, func() {
		if _, err := e.core.AddComment(ctx, testAgent, card.ID, "我自己的评论"); err != nil {
			t.Errorf("agent 评论失败: %v", err)
		}
	})
	later(250*time.Millisecond, func() {
		if _, err := e.core.AddComment(ctx, testActor, card.ID, "人的评论"); err != nil {
			t.Errorf("人评论失败: %v", err)
		}
	})

	got := e.wait(t, testAgent, nil, 0, longWait)
	if got.TimedOut || len(got.Events) != 1 || got.Events[0].Actor != testActor.Name || got.Cursor != e.lastEvent(t).ID {
		t.Fatalf("应该跳过自己的评论，只返回人的评论: %+v", got)
	}
}

// TestWaitOnlyOwnEventsTimesOut 验证只有自己的操作时等到超时，并且返回的进度已经越过那条被过滤的事件，
// 调用方确认之后下一轮不会再碰到它。
func TestWaitOnlyOwnEventsTimesOut(t *testing.T) {
	e := newCardEnv(t)
	card := e.card(t, colTodo, "X")
	e.pull(t, testAgent, nil, 0)
	later(30*time.Millisecond, func() {
		if _, err := e.core.AddComment(context.Background(), testAgent, card.ID, "我自己的评论"); err != nil {
			t.Errorf("agent 评论失败: %v", err)
		}
	})

	got := e.wait(t, testAgent, nil, 0, 400*time.Millisecond)
	if !got.TimedOut || len(got.Events) != 0 || got.Cursor != e.lastEvent(t).ID {
		t.Fatalf("应该超时，空事件，进度越过自己的评论: %+v", got)
	}
}

// TestWaitKeepsScanningWhenPageFiltered 验证整页被过滤但后面还有事件时不进入等待：
// 直接读下一页，把后面的人的事件带回来，而不是挂到超时。
func TestWaitKeepsScanningWhenPageFiltered(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	card := e.card(t, colTodo, "X")
	e.pull(t, testAgent, nil, 0)
	for range 3 {
		if _, err := e.core.AddComment(ctx, testAgent, card.ID, "自己的评论"); err != nil {
			t.Fatalf("评论失败: %v", err)
		}
	}
	if _, err := e.core.AddComment(ctx, testActor, card.ID, "人的评论"); err != nil {
		t.Fatalf("评论失败: %v", err)
	}

	got := e.wait(t, testAgent, nil, 2, longWait)
	if got.TimedOut || len(got.Events) != 1 || got.Events[0].Actor != testActor.Name {
		t.Fatalf("应该翻页读到人的评论: %+v", got)
	}
}

// TestWaitContextCancelled 验证请求取消：等待中途取消，返回取消错误，不会一直挂着。
func TestWaitContextCancelled(t *testing.T) {
	e := newCardEnv(t)
	e.pull(t, testAgent, nil, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	later(50*time.Millisecond, cancel)

	_, err := e.core.WaitUpdates(ctx, testAgent, e.board.ID, kanban.WaitInput{Timeout: longWait})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("应该返回取消错误: %v", err)
	}
}

// TestWaitBoardDeleted 验证等待期间看板被删除：等待被唤醒，返回看板不存在。
func TestWaitBoardDeleted(t *testing.T) {
	e := newCardEnv(t)
	e.pull(t, testAgent, nil, 0)
	later(50*time.Millisecond, func() {
		if err := e.core.DeleteBoard(context.Background(), testActor, e.board.ID); err != nil {
			t.Errorf("删除看板失败: %v", err)
		}
	})

	_, err := e.core.WaitUpdates(context.Background(), testAgent, e.board.ID, kanban.WaitInput{Timeout: longWait})
	if !errors.Is(err, kanban.ErrBoardNotFound) {
		t.Fatalf("应该返回看板不存在: %v", err)
	}
}

// TestWaitTruncatedReturnsImmediately 验证离线太久：事件被清理时不等待，马上告诉调用方 truncated。
func TestWaitTruncatedReturnsImmediately(t *testing.T) {
	e := newCardEnv(t, kanban.WithRetentionForTest(5, 1))
	e.pull(t, testAgent, nil, 0)
	for i := range 8 {
		e.card(t, colTodo, fmt.Sprintf("任务 %d", i))
	}
	got := e.wait(t, testAgent, nil, 0, longWait)
	if !got.Truncated || got.TimedOut {
		t.Fatalf("应该马上返回 truncated: %+v", got)
	}
}

// TestWaitManyAgentsWakeTogether 验证多个 agent 同时等待同一个看板：
// 一条新事件把它们全部唤醒，每个人都恰好收到这一条。
func TestWaitManyAgentsWakeTogether(t *testing.T) {
	e := newCardEnv(t)
	const agents = 5
	var wg sync.WaitGroup
	results := make([]*kanban.Updates, agents)
	for i := range agents {
		agent := kanban.Actor{Kind: kanban.ActorAgent, Name: fmt.Sprintf("agent-%d", i)}
		e.pull(t, agent, nil, 0)
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := e.core.WaitUpdates(context.Background(), agent, e.board.ID, kanban.WaitInput{Timeout: longWait})
			if err != nil {
				t.Errorf("%s 等待失败: %v", agent.Name, err)
				return
			}
			results[i] = out
		}()
	}
	later(100*time.Millisecond, func() {
		if _, err := e.core.CreateCard(context.Background(), testActor, e.board.ID, kanban.CreateCardInput{Title: "大家都看这个", ColumnID: e.col(colTodo)}); err != nil {
			t.Errorf("创建任务失败: %v", err)
		}
	})
	wg.Wait()

	for i, got := range results {
		if got == nil || got.TimedOut || len(got.Events) != 1 || got.Events[0].Type != kanban.EventCardCreated {
			t.Errorf("agent-%d 应该恰好收到建卡事件: %+v", i, got)
		}
	}
}

// TestWaitRejects 验证拒绝的情况：人不能等待，参数错误返回 400，看板不存在马上返回 404 而不是等到超时。
func TestWaitRejects(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	cases := map[string]struct {
		actor   kanban.Actor
		boardID int64
		in      kanban.WaitInput
		want    error
	}{
		"人":     {testActor, e.board.ID, kanban.WaitInput{Timeout: longWait}, kanban.ErrAgentOnly},
		"负数超时":  {testAgent, e.board.ID, kanban.WaitInput{Timeout: -time.Second}, kanban.ErrInvalidInput},
		"负数数量":  {testAgent, e.board.ID, kanban.WaitInput{PullInput: kanban.PullInput{Limit: -1}, Timeout: longWait}, kanban.ErrInvalidInput},
		"负数确认":  {testAgent, e.board.ID, kanban.WaitInput{PullInput: kanban.PullInput{Since: ptr64(-1)}, Timeout: longWait}, kanban.ErrInvalidInput},
		"看板不存在": {testAgent, 9999, kanban.WaitInput{Timeout: longWait}, kanban.ErrBoardNotFound},
	}
	for name, c := range cases {
		if _, err := e.core.WaitUpdates(ctx, c.actor, c.boardID, c.in); !errors.Is(err, c.want) {
			t.Errorf("%s: 期望 %v，实际 %v", name, c.want, err)
		}
	}
}
