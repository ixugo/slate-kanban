package kanbanapi

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/lifecycle"
)

// maxTimeoutSeconds 是请求里等待秒数的上限，只用来防止换算成纳秒时溢出；
// 真正的等待上限由 Core 决定，超过的会被它压到上限。
const maxTimeoutSeconds = 3600

// eventRoutes 注册快照、事件、agent 拉取更新和实时流的路由。
func (a API) eventRoutes(r gin.IRouter) {
	r.GET("/boards/:board_id/snapshot", route(a.snapshot))
	r.GET("/boards/:board_id/events", route(a.listEvents))
	r.POST("/boards/:board_id/updates/pull", route(a.pullUpdates))
	r.POST("/boards/:board_id/updates/wait", route(a.waitUpdates))
	r.GET("/boards/:board_id/stream", a.stream)
	r.GET("/boards/:board_id/export", a.export)
}

// listEventsInput 是读事件列表的请求，查询参数 after 是起点序号，limit 是最多返回几条。
type listEventsInput struct {
	boardPath
	After int64 `form:"after" json:"-"`
	Limit int   `form:"limit" json:"-"`
}

// pullUpdatesInput 是 agent 拉取更新的请求。
type pullUpdatesInput struct {
	boardPath
	kanban.PullInput
}

// waitUpdatesInput 是 agent 阻塞等待更新的请求，TimeoutSeconds 为 0 时用 Core 的默认等待时长。
type waitUpdatesInput struct {
	boardPath
	kanban.PullInput
	TimeoutSeconds int `json:"timeout_seconds"`
}

// snapshot 一次返回整块看板：看板、列、规则、全部卡片和读取之前的最新事件序号。
func (a API) snapshot(ctx context.Context, _ kanban.Actor, in *boardPath) (*kanban.Snapshot, error) {
	return a.core.BoardSnapshot(ctx, in.BoardID)
}

// listEvents 返回起点序号之后的事件，不做过滤，网页的活动列表和断线补发用。
func (a API) listEvents(ctx context.Context, _ kanban.Actor, in *listEventsInput) (*kanban.EventPage, error) {
	return a.core.ListEvents(ctx, in.BoardID, in.After, in.Limit)
}

// pullUpdates 返回 agent 还没看过的更新，并按请求里的 since 确认上一批，只有 agent 能做。
func (a API) pullUpdates(ctx context.Context, actor kanban.Actor, in *pullUpdatesInput) (*kanban.Updates, error) {
	return a.core.PullUpdates(ctx, actor, in.BoardID, in.PullInput)
}

// waitUpdates 和 pullUpdates 一样，但没有新更新时会等到有更新或超时，只有 agent 能做。
// 等待期间服务开始关闭会被打断，返回 503，客户端稍后重试即可。
func (a API) waitUpdates(ctx context.Context, actor kanban.Actor, in *waitUpdatesInput) (*kanban.Updates, error) {
	timeout := time.Duration(min(in.TimeoutSeconds, maxTimeoutSeconds)) * time.Second
	bound, cancel := lifecycle.Bind(ctx)
	defer cancel()
	out, err := a.core.WaitUpdates(bound, actor, in.BoardID, kanban.WaitInput{PullInput: in.PullInput, Timeout: timeout})
	if lifecycle.Interrupted(ctx, err) {
		return nil, lifecycle.ErrShuttingDown
	}
	return out, err
}
