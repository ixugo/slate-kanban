package kanban_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"slate-kanban/internal/core/kanban"
)

// signaled 判断通道在给定时间内是否收到了信号。
func signaled(ch <-chan struct{}, within time.Duration) bool {
	select {
	case <-ch:
		return true
	case <-time.After(within):
		return false
	}
}

// TestHubWakesOnlyTheSameBoard 验证信号只唤醒订阅同一个看板的等待者。
func TestHubWakesOnlyTheSameBoard(t *testing.T) {
	hub := kanban.NewHub()
	same, cancelSame := hub.Subscribe(1)
	defer cancelSame()
	other, cancelOther := hub.Subscribe(2)
	defer cancelOther()

	if err := kanban.PublishForTest(hub, kanban.Published{BoardID: 1, Seq: 5}); err != nil {
		t.Fatalf("广播失败: %v", err)
	}
	if !signaled(same, time.Second) {
		t.Fatal("同一个看板的等待者应该被唤醒")
	}
	if signaled(other, 50*time.Millisecond) {
		t.Fatal("别的看板的等待者不应该被唤醒")
	}
}

// TestHubCoalescesSignals 验证等待者没来得及取信号时，多次广播合并成一次唤醒，
// 并且广播本身不会因为通道已满而阻塞。
func TestHubCoalescesSignals(t *testing.T) {
	hub := kanban.NewHub()
	ch, cancel := hub.Subscribe(1)
	defer cancel()

	done := make(chan struct{})
	go func() {
		for i := range 100 {
			_ = kanban.PublishForTest(hub, kanban.Published{BoardID: 1, Seq: int64(i)})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("广播被阻塞了")
	}

	if !signaled(ch, time.Second) {
		t.Fatal("应该有一次唤醒")
	}
	if signaled(ch, 50*time.Millisecond) {
		t.Fatal("多次广播应该合并成一次唤醒")
	}
}

// TestHubCancelStopsDelivery 验证取消订阅后不再收到信号，并且重复取消是安全的。
func TestHubCancelStopsDelivery(t *testing.T) {
	hub := kanban.NewHub()
	ch, cancel := hub.Subscribe(1)
	cancel()
	cancel()

	_ = kanban.PublishForTest(hub, kanban.Published{BoardID: 1, Seq: 1})
	if signaled(ch, 50*time.Millisecond) {
		t.Fatal("取消订阅后不应该再收到信号")
	}
}

// TestHubNotifiesObservers 验证长期观察者能收到广播内容；
// 观察者返回的错误会交给调用方，但不会妨碍等待者被唤醒。
func TestHubNotifiesObservers(t *testing.T) {
	hub := kanban.NewHub()
	var got atomic.Int64
	hub.Register("seq", func(_ context.Context, p kanban.Published) error {
		got.Store(p.Seq)
		return nil
	})
	if err := kanban.PublishForTest(hub, kanban.Published{BoardID: 1, Seq: 42}); err != nil {
		t.Fatalf("广播失败: %v", err)
	}
	if got.Load() != 42 {
		t.Fatalf("观察者应该收到序号 42，实际 %d", got.Load())
	}

	failing := kanban.NewHub()
	boom := errors.New("boom")
	failing.Register("fail", func(context.Context, kanban.Published) error { return boom })
	ch, cancel := failing.Subscribe(1)
	defer cancel()
	if err := kanban.PublishForTest(failing, kanban.Published{BoardID: 1}); !errors.Is(err, boom) {
		t.Fatalf("应该返回观察者的错误，实际: %v", err)
	}
	if !signaled(ch, time.Second) {
		t.Fatal("观察者出错也应该先唤醒等待者")
	}
}

// TestHubRegisterDuplicateKeyPanics 验证重复注册同一个 key 会 panic，防止观察者被悄悄覆盖。
func TestHubRegisterDuplicateKeyPanics(t *testing.T) {
	hub := kanban.NewHub()
	hub.Register("dup", func(context.Context, kanban.Published) error { return nil })
	defer func() {
		if recover() == nil {
			t.Fatal("重复注册应该 panic")
		}
	}()
	hub.Register("dup", func(context.Context, kanban.Published) error { return nil })
}
