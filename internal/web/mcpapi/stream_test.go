package mcpapi_test

import (
	"net/http"
	"testing"
	"time"
)

type streamEnd struct {
	code    int
	elapsed time.Duration
}

func (s *stack) openStream(t *testing.T, name, session string) <-chan streamEnd {
	t.Helper()
	ended := make(chan streamEnd, 1)
	start := time.Now()
	go func() {
		code, _, _ := s.do(t, raw{name: name, session: session, method: http.MethodGet})
		ended <- streamEnd{code: code, elapsed: time.Since(start)}
	}()
	return ended
}

// TestShutdownEndsStream 验证服务开始关闭时通知流立刻结束。
func TestShutdownEndsStream(t *testing.T) {
	s := newStack(t)
	token := s.token(t, "claude")
	ended := s.openStream(t, token, initSession(t, s, token))
	time.Sleep(300 * time.Millisecond)
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
