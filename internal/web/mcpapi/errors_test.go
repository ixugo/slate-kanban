package mcpapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/ixugo/goddd/pkg/reason"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/lifecycle"
)

// captureLog 把默认日志器换成写进缓冲区的，返回缓冲区；测试结束时恢复原来的日志器。
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })
	return &buf
}

// TestToolError 验证错误转换：业务错误只给 reason 和 msg，不给 details；
// 服务端错误、未知错误不泄露细节但会记日志；服务关闭和客户端取消是正常情况，不记日志。
func TestToolError(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		want       string
		wantLogged bool
		notContain []string
	}{
		{
			name:       "业务错误带 details",
			err:        kanban.ErrHumanOnly.WithMsg("只有人能做").Withf("内部排查信息 secret-detail"),
			want:       "ErrHumanOnly: 只有人能做",
			notContain: []string{"secret-detail"},
		},
		{
			name:       "包了一层的业务错误",
			err:        fmt.Errorf("外层: %w", kanban.ErrHumanOnly.WithMsg("只有人能做")),
			want:       "ErrHumanOnly: 只有人能做",
			notContain: []string{"外层"},
		},
		{
			name:       "数据库错误",
			err:        reason.ErrDB.Withf("读取任务: SELECT * FROM cards secret-sql"),
			want:       "ErrStore: 数据发生错误",
			wantLogged: true,
			notContain: []string{"secret-sql", "SELECT"},
		},
		{
			name: "服务关闭打断",
			err:  lifecycle.ErrShuttingDown,
			want: "ErrServiceUnavailable: 服务正在关闭，请稍后重试",
		},
		{
			name: "客户端取消",
			err:  fmt.Errorf("等待更新: %w", context.Canceled),
			want: "请求已取消",
		},
		{
			name:       "未知错误",
			err:        errors.New("dial tcp 10.0.0.1:5432: secret-host refused"),
			want:       "ErrInternal: 服务器内部错误",
			wantLogged: true,
			notContain: []string{"secret-host"},
		},
	}
	for _, tc := range cases {
		buf := captureLog(t)
		got := toolError(t.Context(), tc.err)
		if got == nil || got.Error() != tc.want {
			t.Errorf("%s：期望 %q，实际 %v", tc.name, tc.want, got)
			continue
		}
		for _, bad := range tc.notContain {
			if strings.Contains(got.Error(), bad) {
				t.Errorf("%s：返回给 agent 的文本不应该包含 %q: %s", tc.name, bad, got)
			}
		}
		if logged := buf.Len() > 0; logged != tc.wantLogged {
			t.Errorf("%s：是否记日志期望 %v，实际 %v: %s", tc.name, tc.wantLogged, logged, buf)
		}
	}
}

// TestToolErrorKeepsDetailsInLog 验证记日志时保留完整的错误，方便排查。
func TestToolErrorKeepsDetailsInLog(t *testing.T) {
	buf := captureLog(t)
	_ = toolError(t.Context(), reason.ErrDB.Withf("读取任务: secret-sql"))
	if !strings.Contains(buf.String(), "secret-sql") {
		t.Errorf("日志里应该有完整的错误: %s", buf)
	}
}

// TestToolErrorNil 验证没有错误时原样返回 nil，调用方可以不判断直接转换。
func TestToolErrorNil(t *testing.T) {
	if got := toolError(t.Context(), nil); got != nil {
		t.Errorf("nil 应该返回 nil: %v", got)
	}
}
