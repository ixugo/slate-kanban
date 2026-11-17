package lifecycle_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"slate-kanban/internal/lifecycle"
)

// within 是等一件应该立刻发生的事的最长时间。
const within = 3 * time.Second

// waitDone 要求 ch 在 within 内被关闭，否则让测试终止。
func waitDone(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(within):
		t.Fatalf("%s：等了很久也没有发生", what)
	}
}

// serveBound 用 Wrap 包一个处理器，请求进来后调用 Bind，并把绑定后的上下文交给 got。
// 返回的通道在处理器返回时关闭。
func serveBound(t *testing.T, closing context.Context, req *http.Request, got func(request, bound context.Context)) <-chan struct{} {
	t.Helper()
	done := make(chan struct{})
	h := lifecycle.Wrap(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		defer close(done)
		bound, cancel := lifecycle.Bind(r.Context())
		defer cancel()
		got(r.Context(), bound)
	}), closing)
	go h.ServeHTTP(httptest.NewRecorder(), req)
	return done
}

// TestBindEndsWhenClosing 验证服务开始关闭时，绑定后的上下文被取消，而请求本身的上下文不受影响。
func TestBindEndsWhenClosing(t *testing.T) {
	closing, closeNow := context.WithCancel(t.Context())
	defer closeNow()
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	reqErr := make(chan error, 1)
	done := serveBound(t, closing, req, func(request, bound context.Context) {
		<-bound.Done()
		reqErr <- request.Err()
	})
	closeNow()
	waitDone(t, done, "关闭后处理函数应该退出")
	if err := <-reqErr; err != nil {
		t.Fatalf("关闭服务不应该取消请求本身的上下文: %v", err)
	}
}

// TestBindEndsWhenRequestDone 验证请求自己结束（比如客户端断开）时，绑定后的上下文同样被取消，
// 此时服务并没有在关闭。
func TestBindEndsWhenRequestDone(t *testing.T) {
	closing, closeNow := context.WithCancel(t.Context())
	defer closeNow()
	reqCtx, hangUp := context.WithCancel(t.Context())
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody).WithContext(reqCtx)
	done := serveBound(t, closing, req, func(_, bound context.Context) { <-bound.Done() })
	hangUp()
	waitDone(t, done, "请求结束后处理函数应该退出")
	if closing.Err() != nil {
		t.Fatal("请求结束不应该让服务进入关闭状态")
	}
}

// TestBindWithoutWrap 验证没有经过 Wrap 的上下文（比如直接调用处理函数的测试）也能用，
// 效果和普通的 WithCancel 一样：父上下文取消、自己调用取消，都会结束。
func TestBindWithoutWrap(t *testing.T) {
	parent, cancelParent := context.WithCancel(t.Context())
	bound, cancel := lifecycle.Bind(parent)
	defer cancel()
	if bound.Err() != nil {
		t.Fatal("刚绑定时不应该已经取消")
	}
	cancelParent()
	waitDone(t, bound.Done(), "父上下文取消后应该跟着取消")

	own, ownCancel := lifecycle.Bind(t.Context())
	ownCancel()
	waitDone(t, own.Done(), "自己调用取消后应该结束")
}

// TestBindCancelUnderWrap 验证经过 Wrap 的请求里，处理函数自己调用返回的 cancel，绑定后的上下文立刻结束。
func TestBindCancelUnderWrap(t *testing.T) {
	closing, closeNow := context.WithCancel(t.Context())
	defer closeNow()
	var err error
	h := lifecycle.Wrap(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		bound, cancel := lifecycle.Bind(r.Context())
		cancel()
		err = bound.Err()
	}), closing)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("调用 cancel 之后应该已经取消: %v", err)
	}
}

// TestBindAfterClosed 验证服务已经在关闭时才进来的请求，绑定后的上下文一开始就是取消的，
// 长连接不会在关闭期间新建起来。
func TestBindAfterClosed(t *testing.T) {
	closing, closeNow := context.WithCancel(t.Context())
	closeNow()
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	done := serveBound(t, closing, req, func(_, bound context.Context) { <-bound.Done() })
	waitDone(t, done, "关闭之后进来的请求应该立刻结束")
}

// TestInterrupted 验证怎样判断一次等待是被服务关闭打断的：被取消，而请求本身还好好的。
func TestInterrupted(t *testing.T) {
	alive := t.Context()
	gone, hangUp := context.WithCancel(t.Context())
	hangUp()
	cases := []struct {
		name    string
		request context.Context
		err     error
		want    bool
	}{
		{"请求还在，等待被取消", alive, context.Canceled, true},
		{"错误被包了一层", alive, errors.Join(errors.New("读取"), context.Canceled), true},
		{"请求自己已经结束", gone, context.Canceled, false},
		{"别的错误", alive, errors.New("数据库错误"), false},
		{"没有错误", alive, nil, false},
		{"超时不算", alive, context.DeadlineExceeded, false},
	}
	for _, tc := range cases {
		if got := lifecycle.Interrupted(tc.request, tc.err); got != tc.want {
			t.Errorf("%s：期望 %v，实际 %v", tc.name, tc.want, got)
		}
	}
}

// TestErrShuttingDown 验证关闭期间给客户端的错误是 503，客户端据此知道稍后重试。
func TestErrShuttingDown(t *testing.T) {
	if got := lifecycle.ErrShuttingDown.GetHTTPCode(); got != http.StatusServiceUnavailable {
		t.Errorf("应该是 503: %d", got)
	}
	if lifecycle.ErrShuttingDown.GetReason() != "ErrServiceUnavailable" {
		t.Errorf("reason 应该沿用 ErrServiceUnavailable: %s", lifecycle.ErrShuttingDown.GetReason())
	}
}
