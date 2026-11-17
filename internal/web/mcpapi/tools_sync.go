package mcpapi

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/lifecycle"
)

// maxTimeoutSeconds 是入参里等待秒数的上限，只用来防止换算成纳秒时溢出；
// 真正的等待上限由 Core 决定，超过的会被它压到上限。
const maxTimeoutSeconds = 3600

// pullIn 是 pull_updates 的入参。since 用指针：没出现和填 0 是两回事，没出现表示"不确认任何东西"。
type pullIn struct {
	BoardID int64  `json:"board_id" jsonschema:"看板编号，来自 list_boards"`
	Since   *int64 `json:"since,omitempty" jsonschema:"上一次返回的 cursor，表示那一批已经处理完；第一次调用不要填"`
	Limit   int    `json:"limit,omitempty" jsonschema:"一次最多返回几条事件，默认 100，最多 500"`
}

// waitIn 是 wait_for_updates 的入参：拉取的参数加一个最长等待时间。
type waitIn struct {
	pullIn
	TimeoutSeconds int `json:"timeout_seconds,omitempty" jsonschema:"最长等待秒数，默认 30，最多 50；超时不是错误"`
}

// registerSyncTools 注册同步类的工具。拉取更新和阻塞等待只给 agent：它们依赖服务端记录的 agent 读取进度，
// 人在网页上直接看，不需要进度。
func (a API) registerSyncTools(s *mcp.Server) {
	register(s, spec{
		name: "pull_updates", kind: kindWrite,
		desc: "拉取你在这个看板上还没读到的更新（事件列表），不阻塞。读取进度记在服务端：只有下一次调用的 since 才会推进进度，" +
			"所以结果丢了或没处理完，重新调用还能拿到同样的事件，你离线多久都不会漏。你自己的操作、卡片排序和自动化流水已经被过滤掉。" +
			"返回的 cursor 是这一批最后一条事件的序号，处理完之后把它作为下一次的 since。" +
			"has_more 为 true 表示后面还有，立刻再拉一次。" +
			"first 为 true 表示你第一次读这个看板，不补发历史，请用 list_cards 了解现状。" +
			"truncated 为 true 表示你离线太久、中间的事件已被清理，请用 list_cards 重新了解现状。",
	}, a.pullUpdates)
	register(s, spec{
		name: "wait_for_updates", kind: kindWrite,
		desc: "和 pull_updates 一样，但现在没有值得处理的事件时会阻塞等待，直到有更新、超时或服务关闭。" +
			"用它代替定时轮询：挂着等，有事立刻醒。timeout_seconds 默认 30，最长 50；" +
			"超时不是错误，返回 timed_out 为 true 和空事件，带上返回的 cursor 作为 since 立刻再等一次即可。" +
			"第一次读这个看板（first）和离线太久（truncated）会立刻返回，不等待。" +
			"服务关闭时返回 ErrServiceUnavailable，稍后重试。",
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
	timeout := time.Duration(min(in.TimeoutSeconds, maxTimeoutSeconds)) * time.Second
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
