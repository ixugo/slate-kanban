package mcpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ixugo/goddd/pkg/reason"

	"slate-kanban/internal/lifecycle"
)

// httpServerError 是 reason 错误里"服务端出错"的状态码下限：低于它的是调用方可以自己改正的错误
// （权限、校验、找不到、冲突），不低于它的是服务端自己的问题。
const httpServerError = 500

// toolError 把 Core 或系统返回的错误换成给 agent 看的错误，agent 读到的就是它的文本。
//   - 业务错误（状态码小于 500）：文本是 "reason: msg"，agent 可以据此改正参数或换个做法；
//   - 服务端错误（数据库出错等）：完整的错误记进日志，文本只有通用的 "reason: msg"；
//   - 服务关闭打断和客户端取消是正常情况，不记日志；
//   - 其他错误：记日志，文本固定为内部错误。
//
// 无论哪种，都不返回 reason 错误的 details，因为里面可能有 SQL、主机名之类不该进入模型上下文的内容。
func toolError(ctx context.Context, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, lifecycle.ErrShuttingDown):
		return describe(lifecycle.ErrShuttingDown)
	case errors.Is(err, context.Canceled):
		return errors.New("请求已取消")
	}
	var info reason.ErrorInfoer
	if !errors.As(err, &info) {
		slog.ErrorContext(ctx, "MCP 工具调用出现未预期的错误", "err", err)
		return describe(reason.ErrInternal)
	}
	if info.GetHTTPCode() >= httpServerError {
		slog.ErrorContext(ctx, "MCP 工具调用失败", "reason", info.GetReason(), "err", err)
	}
	return describe(info)
}

// describe 把带 reason 的错误写成 "reason: msg"，不带 details。
func describe(info reason.ErrorInfoer) error {
	return fmt.Errorf("%s: %s", info.GetReason(), info.GetMessage())
}
