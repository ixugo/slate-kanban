package mcpapi_test

import (
	"net/http"
	"testing"
	"time"

	"slate-kanban/internal/core/access"
	"slate-kanban/internal/web/mcpapi"
)

// streamEnd 是一条通知流结束时的情况：响应状态码和流持续了多久。
type streamEnd struct {
	code    int
	elapsed time.Duration
}

// openStream 用 GET 打开一条通知流，流结束（服务端关掉它）时把结果放进返回的通道。
// 流不结束，这个调用就一直占着一个协程，所以测试结束前一定要让服务端结束它。
func (s *stack) openStream(t *testing.T, token, session string) <-chan streamEnd {
	t.Helper()
	ended := make(chan streamEnd, 1)
	start := time.Now()
	go func() {
		code, _, _ := s.do(t, raw{token: token, session: session, method: http.MethodGet})
		ended <- streamEnd{code: code, elapsed: time.Since(start)}
	}()
	return ended
}

// TestStreamEndsWhenTokenRevoked 验证令牌被删除之后，已经连着的通知流也会结束：服务端定期重新校验令牌，
// 否则被撤销的 agent 能一直收到"看板有更新"的提示。重新校验的间隔内流还在，间隔到了就结束。
func TestStreamEndsWhenTokenRevoked(t *testing.T) {
	const recheck = 300 * time.Millisecond
	s := newStack(t, mcpapi.WithStreamRecheck(recheck))
	made, err := s.access.CreateToken(t.Context(), access.Principal{Admin: true, Name: access.AdminName}, "temp")
	if err != nil {
		t.Fatalf("创建令牌失败: %v", err)
	}
	ended := s.openStream(t, made.Plain, initSession(t, s, made.Plain))

	select {
	case got := <-ended:
		t.Fatalf("令牌还有效，通知流不应该结束: %+v", got)
	case <-time.After(3 * recheck):
	}
	if err := s.access.DeleteToken(t.Context(), access.Principal{Admin: true, Name: access.AdminName}, made.ID); err != nil {
		t.Fatalf("删除令牌失败: %v", err)
	}
	start := time.Now()
	select {
	case got := <-ended:
		if got.code != http.StatusOK || time.Since(start) > 5*time.Second {
			t.Errorf("令牌删除后通知流应该在一个重新校验间隔内正常结束，实际 %+v，等了 %v", got, time.Since(start))
		}
	case <-time.After(10 * time.Second):
		t.Fatal("令牌删除之后，通知流没有结束")
	}
}

// TestShutdownEndsStream 验证服务开始关闭时通知流立刻结束，而不是挂着让 Shutdown 一直等到超时。
func TestShutdownEndsStream(t *testing.T) {
	s := newStack(t)
	token := s.token(t, "claude")
	ended := s.openStream(t, token, initSession(t, s, token))
	time.Sleep(300 * time.Millisecond) // 让流先连上，再模拟关闭

	start := time.Now()
	s.closeNow()
	select {
	case got := <-ended:
		if elapsed := time.Since(start); elapsed > 3*time.Second {
			t.Errorf("关闭后通知流应该立刻结束，实际等了 %v (%+v)", elapsed, got)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("服务关闭之后，通知流没有结束")
	}
}
