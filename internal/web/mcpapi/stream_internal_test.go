package mcpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
)

// TestEndWhenRevokedIgnoresAuthErrors 验证认证本身出错（比如查库失败）时通知流不会被结束，
// 只有令牌确定无效才结束；并且校验用的是请求里带的那个令牌。
func TestEndWhenRevokedIgnoresAuthErrors(t *testing.T) {
	var calls atomic.Int32
	var seen atomic.Value
	verify := func(_ context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
		seen.Store(token)
		if calls.Add(1) <= 2 {
			return nil, errAuthUnavailable
		}
		return nil, auth.ErrInvalidToken
	}
	r := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	r.Header.Set("Authorization", "Bearer secret")
	ended := make(chan struct{})

	go endWhenRevoked(t.Context(), func() { close(ended) }, verify, r, 20*time.Millisecond)

	select {
	case <-ended:
	case <-time.After(5 * time.Second):
		t.Fatal("令牌无效之后，通知流没有结束")
	}
	if n := calls.Load(); n != 3 {
		t.Errorf("前两次认证出错不该结束流，第三次令牌无效才结束，实际校验了 %d 次", n)
	}
	if got := seen.Load(); got != "secret" {
		t.Errorf("校验用的令牌 = %v，期望请求里带的 secret", got)
	}
}

// TestEndWhenRevokedStopsWithContext 验证流自己结束（上下文取消）之后，重新校验的协程会退出，不留后台协程，也不再校验。
func TestEndWhenRevokedStopsWithContext(t *testing.T) {
	var calls atomic.Int32
	verify := func(context.Context, string, *http.Request) (*auth.TokenInfo, error) {
		calls.Add(1)
		return nil, auth.ErrInvalidToken
	}
	r := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	r.Header.Set("Authorization", "Bearer secret")
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})

	go func() {
		endWhenRevoked(ctx, func() {}, verify, r, time.Hour)
		close(done)
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("上下文取消之后，重新校验的协程没有退出")
	}
	if n := calls.Load(); n != 0 {
		t.Errorf("没到校验间隔就退出了，不该校验，实际校验了 %d 次", n)
	}
}

// TestEndWhenRevokedWithoutToken 验证请求没带合法的 Authorization 头时直接返回，不校验也不 panic。
// 正常情况下认证中间件已经拦过这种请求，这里只是兜底。
func TestEndWhenRevokedWithoutToken(t *testing.T) {
	var calls atomic.Int32
	verify := func(context.Context, string, *http.Request) (*auth.TokenInfo, error) {
		calls.Add(1)
		return nil, auth.ErrInvalidToken
	}
	for _, header := range []string{"", "Bearer", "Bearer a b"} {
		r := httptest.NewRequest(http.MethodGet, "/mcp", nil)
		if header != "" {
			r.Header.Set("Authorization", header)
		}
		endWhenRevoked(t.Context(), func() { t.Errorf("头 %q 不该结束流", header) }, verify, r, time.Millisecond)
	}
	if n := calls.Load(); n != 0 {
		t.Errorf("没有合法令牌时不该校验，实际校验了 %d 次", n)
	}
}
