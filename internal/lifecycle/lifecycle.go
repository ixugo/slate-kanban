// Package lifecycle 让长连接的处理函数感知服务正在关闭。
//
// 实时流和阻塞等待可以挂很久。如果不通知它们，关闭服务时 http.Server.Shutdown 会一直等到超时，
// 还会留下一条错误日志。做法是：应用在关闭前先取消一个"关闭中"的上下文，Wrap 把它放进每个请求的上下文，
// 长连接的处理函数用 Bind 拿到一个同时随它取消的上下文。普通请求不受影响，照常处理完。
package lifecycle

import (
	"context"
	"errors"
	"net/http"

	"github.com/ixugo/goddd/pkg/reason"
)

// ErrShuttingDown 是服务关闭期间，被打断的长连接得到的错误：503，提示稍后重试。
var ErrShuttingDown = reason.ErrServiceUnavailable.WithMsg("服务正在关闭，请稍后重试")

// closingKey 是请求上下文里存放"关闭中"上下文的键。
type closingKey struct{}

// Wrap 包装处理器：每个请求的上下文里都带上 closing。closing 被取消表示服务开始关闭。
func Wrap(next http.Handler, closing context.Context) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), closingKey{}, closing)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Bind 返回一个上下文，请求结束或服务开始关闭时都会被取消，长连接的处理函数应该用它。
// 用完要调用返回的 cancel，释放对关闭信号的监听。没有经过 Wrap 的上下文也能用，此时只随请求结束而取消。
func Bind(ctx context.Context) (context.Context, context.CancelFunc) {
	bound, cancel := context.WithCancel(ctx)
	closing, ok := ctx.Value(closingKey{}).(context.Context)
	if !ok {
		return bound, cancel
	}
	stop := context.AfterFunc(closing, cancel)
	return bound, func() {
		stop()
		cancel()
	}
}

// Interrupted 判断一次等待是不是被服务关闭打断的：等待被取消了，但请求本身还好好的。
// request 是请求自己的上下文（不是 Bind 返回的那个）。
func Interrupted(request context.Context, err error) bool {
	return errors.Is(err, context.Canceled) && request.Err() == nil
}
