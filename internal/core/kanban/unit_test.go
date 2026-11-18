package kanban_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/kanbantest"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
)

var testActor = kanban.Actor{Kind: kanban.ActorHuman, Name: "admin"}

// env 是业务测试的公共环境：内存数据库、一个现成的看板、接好的 Core，
// 并且记录 Core 对外广播过的所有更新信号。
type env struct {
	db    kanbandb.DB
	core  kanban.Core
	board *kanban.Board

	mu  sync.Mutex
	got []kanban.Published
}

// newEnv 创建测试环境，opts 会传给 Core。
func newEnv(t *testing.T, opts ...kanban.Option) *env {
	t.Helper()
	db := kanbantest.NewDB(t)
	board := &kanban.Board{Name: "测试看板", Slug: "TST"}
	if err := db.Board().Create(context.Background(), board); err != nil {
		t.Fatalf("创建测试看板失败: %v", err)
	}
	e := &env{db: db, core: kanban.NewCore(db, append([]kanban.Option{kanbantest.Resources(t)}, opts...)...), board: board}
	e.core.Observe("record", func(_ context.Context, p kanban.Published) error {
		e.mu.Lock()
		defer e.mu.Unlock()
		e.got = append(e.got, p)
		return nil
	})
	return e
}

// published 返回目前收到的广播的副本。
func (e *env) published() []kanban.Published {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]kanban.Published(nil), e.got...)
}

// events 返回看板里全部事件，按序号升序。
func (e *env) events(t *testing.T) []*kanban.Event {
	t.Helper()
	items, err := e.db.Event().ListAfter(context.Background(), e.board.ID, 0, 100000)
	if err != nil {
		t.Fatalf("读取事件失败: %v", err)
	}
	return items
}

// within 在限定时间内运行 fn。事务没释放连接时 SQLite 会死锁，
// 用它让测试快速失败，而不是一直挂着等到整体超时。
func within(t *testing.T, d time.Duration, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatal("超时，疑似事务没有释放连接")
	}
}

// emitOne 在事务里追加一条测试事件。
func emitOne(ctx context.Context, u *kanban.Unit, boardID int64) error {
	return u.EmitForTest(ctx, testActor, kanban.NewEvent{
		BoardID: boardID, Type: kanban.EventBoardUpdated, Payload: map[string]any{"fields": []string{"name"}},
	})
}

// TestRunCommitsEventsAndPublishesOnce 验证提交成功后事件落库，
// 并且同一个看板的多条事件只广播一次，带的是最大序号。
func TestRunCommitsEventsAndPublishesOnce(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)

	err := e.core.RunForTest(ctx, e.board.ID, func(u *kanban.Unit) error {
		for range 3 {
			if err := emitOne(ctx, u, e.board.ID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}

	events := e.events(t)
	if len(events) != 3 {
		t.Fatalf("应该有 3 条事件，实际 %d", len(events))
	}
	first := events[0]
	if first.Type != kanban.EventBoardUpdated || first.ActorKind != "human" || first.Actor != "admin" ||
		first.Payload != `{"fields":["name"]}` || first.CreatedAt.IsZero() {
		t.Fatalf("事件内容不对: %+v", first)
	}
	got := e.published()
	if len(got) != 1 || got[0].BoardID != e.board.ID || got[0].Seq != events[2].ID {
		t.Fatalf("应该广播一次且序号为 %d，实际 %+v", events[2].ID, got)
	}
}

// TestRunRollsBackOnError 验证 fn 返回错误时，已经追加的事件一并撤销，也不广播，
// 并且原错误原样返回（Web 层靠它里面的 HTTP 状态码）。
func TestRunRollsBackOnError(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	boom := kanban.ErrInvalidActor

	err := e.core.RunForTest(ctx, e.board.ID, func(u *kanban.Unit) error {
		if err := emitOne(ctx, u, e.board.ID); err != nil {
			return err
		}
		return boom
	})
	if err != boom {
		t.Fatalf("应该原样返回 fn 的错误，实际: %v", err)
	}
	if n := len(e.events(t)); n != 0 {
		t.Fatalf("回滚后不应该有事件，实际 %d", n)
	}
	if got := e.published(); len(got) != 0 {
		t.Fatalf("失败不应该广播，实际 %+v", got)
	}
}

// TestRunRollsBackOnPanic 验证 fn panic 时也会回滚并释放连接，之后还能继续执行新事务。
func TestRunRollsBackOnPanic(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)

	func() {
		defer func() {
			if recover() == nil {
				t.Error("fn 的 panic 应该继续向外传播")
			}
		}()
		err := e.core.RunForTest(ctx, e.board.ID, func(u *kanban.Unit) error {
			if err := emitOne(ctx, u, e.board.ID); err != nil {
				return err
			}
			panic("boom")
		})
		t.Errorf("不应该正常返回: %v", err)
	}()

	within(t, 5*time.Second, func() {
		err := e.core.RunForTest(ctx, e.board.ID, func(u *kanban.Unit) error { return emitOne(ctx, u, e.board.ID) })
		if err != nil {
			t.Errorf("panic 之后新的事务应该能正常执行: %v", err)
		}
	})
	if n := len(e.events(t)); n != 1 {
		t.Fatalf("panic 的事务应该已回滚，只剩后一次的 1 条事件，实际 %d", n)
	}
}

// TestRunMissingBoard 验证看板不存在时返回 404 对应的领域错误，fn 不会被调用。
func TestRunMissingBoard(t *testing.T) {
	e := newEnv(t)
	called := false
	err := e.core.RunForTest(context.Background(), 9999, func(*kanban.Unit) error {
		called = true
		return nil
	})
	if !errors.Is(err, kanban.ErrBoardNotFound) {
		t.Fatalf("应该返回 ErrBoardNotFound，实际: %v", err)
	}
	if called {
		t.Fatal("看板不存在时不应该执行 fn")
	}
}

// TestRunWithoutBoardSkipsLock 验证 boardID 传 0（创建看板的场景）时不检查看板，直接执行 fn。
func TestRunWithoutBoardSkipsLock(t *testing.T) {
	e := newEnv(t)
	called := false
	err := e.core.RunForTest(context.Background(), 0, func(*kanban.Unit) error {
		called = true
		return nil
	})
	if err != nil || !called {
		t.Fatalf("应该直接执行 fn，实际 called=%v err=%v", called, err)
	}
}

// TestRunTouchBroadcastsWithoutEvent 验证只标记看板有变化时，会广播（序号为 0）但不产生事件，
// 删除看板靠它让等待者醒来发现看板没了。
func TestRunTouchBroadcastsWithoutEvent(t *testing.T) {
	e := newEnv(t)
	err := e.core.RunForTest(context.Background(), e.board.ID, func(u *kanban.Unit) error {
		u.TouchForTest(e.board.ID)
		return nil
	})
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	got := e.published()
	if len(got) != 1 || got[0].BoardID != e.board.ID || got[0].Seq != 0 {
		t.Fatalf("应该广播一次且序号为 0，实际 %+v", got)
	}
	if n := len(e.events(t)); n != 0 {
		t.Fatalf("不应该产生事件，实际 %d", n)
	}
}

// TestRunPublishErrorDoesNotFail 验证广播处理函数出错不影响本次结果，因为数据已经提交。
func TestRunPublishErrorDoesNotFail(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	e.core.Observe("fail", func(context.Context, kanban.Published) error { return errors.New("boom") })

	err := e.core.RunForTest(ctx, e.board.ID, func(u *kanban.Unit) error { return emitOne(ctx, u, e.board.ID) })
	if err != nil {
		t.Fatalf("广播出错不应该让请求失败: %v", err)
	}
	if n := len(e.events(t)); n != 1 {
		t.Fatalf("数据应该已经提交，实际事件 %d 条", n)
	}
}

// TestEmitRejectsUnencodablePayload 验证事件内容无法编码成 JSON 时返回错误，并且整个事务回滚。
func TestEmitRejectsUnencodablePayload(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	err := e.core.RunForTest(ctx, e.board.ID, func(u *kanban.Unit) error {
		return u.EmitForTest(ctx, testActor, kanban.NewEvent{
			BoardID: e.board.ID, Type: kanban.EventBoardUpdated, Payload: make(chan int),
		})
	})
	if err == nil {
		t.Fatal("无法编码的内容应该返回错误")
	}
	if n := len(e.events(t)); n != 0 {
		t.Fatalf("不应该留下事件，实际 %d", n)
	}
}

// TestEmitPrunesOldEvents 验证事件序号是清理间隔的整数倍时，只保留最新的若干条。
// 保留 5 条、每 10 条清理一次，写入 25 条后，第 10 条触发时留下 6~10，
// 第 20 条触发时留下 16~20，再写 5 条后剩 16~25。
func TestEmitPrunesOldEvents(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t, kanban.WithRetentionForTest(5, 10))
	err := e.core.RunForTest(ctx, e.board.ID, func(u *kanban.Unit) error {
		for range 25 {
			if err := emitOne(ctx, u, e.board.ID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	events := e.events(t)
	if len(events) != 10 || events[0].ID != 16 || events[9].ID != 25 {
		t.Fatalf("应该剩序号 16 到 25 共 10 条，实际 %d 条，首 %d 尾 %d",
			len(events), events[0].ID, events[len(events)-1].ID)
	}
}

// TestRunSerializesWritersOnSameBoard 验证多个写事务同时进入同一个看板不会死锁，
// 全部成功，事件序号互不重复，每次提交各广播一次。
func TestRunSerializesWritersOnSameBoard(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	const writers = 8

	within(t, 10*time.Second, func() {
		var wg sync.WaitGroup
		for range writers {
			wg.Go(func() {
				err := e.core.RunForTest(ctx, e.board.ID, func(u *kanban.Unit) error { return emitOne(ctx, u, e.board.ID) })
				if err != nil {
					t.Errorf("并发写入失败: %v", err)
				}
			})
		}
		wg.Wait()
	})

	events := e.events(t)
	if len(events) != writers {
		t.Fatalf("应该有 %d 条事件，实际 %d", writers, len(events))
	}
	seen := make(map[int64]bool, writers)
	for _, ev := range events {
		if seen[ev.ID] {
			t.Fatalf("事件序号重复: %d", ev.ID)
		}
		seen[ev.ID] = true
	}
	if got := e.published(); len(got) != writers {
		t.Fatalf("每次提交应该各广播一次，共 %d 次，实际 %d", writers, len(got))
	}
}
