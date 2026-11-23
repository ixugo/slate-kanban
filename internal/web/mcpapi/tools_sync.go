package mcpapi

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/lifecycle"
)

// maxTimeoutSeconds 在换算成纳秒前使用，避免超大的输入溢出。
const maxTimeoutSeconds = int(kanban.MaxWaitTimeout / time.Second)

// pullIn 是 pull_updates 的入参。since 用指针：没出现和填 0 是两回事，没出现表示"不确认任何东西"。
type pullIn struct {
	BoardID int64  `json:"board_id" jsonschema:"看板编号，来自 list_boards"`
	Since   *int64 `json:"since,omitempty" jsonschema:"上一次返回的 cursor，表示那一批已经处理完；第一次调用不要填"`
	Limit   int    `json:"limit,omitempty" jsonschema:"一次最多返回几条事件，默认 100，最多 500"`
}

// waitIn 是 wait_for_updates 的入参：拉取的参数加一个最长等待时间。
type waitIn struct {
	pullIn
	TimeoutSeconds int `json:"timeout_seconds,omitempty" jsonschema:"最长等待秒数，默认 30，最多 3600；超时不是错误"`
}

// registerSyncTools 注册同步类的工具。拉取更新和阻塞等待只给 agent：它们依赖服务端记录的 agent 读取进度，
// 人在网页上直接看，不需要进度。
func (a API) registerSyncTools(s *mcp.Server) {
	register(s, spec{
		name: "pull_updates", kind: kindWrite,
		desc: "非阻塞拉取更新。只在处理完上一批后用 since 确认其 cursor；has_more 继续拉取。first 或 truncated 时用 list_cards 对账。",
	}, a.pullUpdates)
	register(s, spec{
		name: "wait_for_updates", kind: kindWrite,
		desc: "沿用 pull_updates 的游标语义，阻塞到有更新或超时。timed_out 不是错误；服务关闭后重试。",
	}, a.waitForUpdates)
}

// pullUpdates 返回 agent 还没读到的更新，并按入参里的 since 确认上一批。
func (a API) pullUpdates(ctx context.Context, actor kanban.Actor, in pullIn) (any, error) {
	return a.core.PullUpdates(ctx, actor, in.BoardID, kanban.PullInput{Since: in.Since, Limit: in.Limit})
}

// waitForUpdates 和 pullUpdates 一样，但没有值得处理的事件时会等到有更新或超时。
// 等待期间服务开始关闭会被打断，返回"服务正在关闭"，agent 稍后重试即可；
// 客户端取消调用时 SDK 会取消 ctx，Core 立刻返回，不会白占着协程。
func (a API) waitForUpdates(ctx context.Context, actor kanban.Actor, in waitIn) (any, error) {
	timeout := time.Duration(min(max(in.TimeoutSeconds, -1), maxTimeoutSeconds)) * time.Second
	bound, cancel := lifecycle.Bind(ctx)
	defer cancel()
	out, err := a.core.WaitUpdates(bound, actor, in.BoardID, kanban.WaitInput{
		PullInput: kanban.PullInput{Since: in.Since, Limit: in.Limit}, Timeout: timeout,
	})
	if lifecycle.Interrupted(ctx, err) {
		return nil, lifecycle.ErrShuttingDown
	}
	return out, err
}
