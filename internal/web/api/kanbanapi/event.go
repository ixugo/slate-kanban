package kanbanapi

import (
	"context"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/lifecycle"
)

// maxTimeoutSeconds 保留 REST 阻塞等待的 50 秒上限。
const maxTimeoutSeconds = 50

// eventRoutes 注册快照、事件、agent 拉取更新和实时流的路由。
func (a API) eventRoutes(r gin.IRouter) {
	r.GET("/boards/:board_id/snapshot", route(a.snapshot))
	r.GET("/boards/:board_id/events", route(a.listEvents))
	r.GET("/boards/:board_id/activity", route(a.activity))
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

// activityInput 是读活动流的请求。before 是翻页起点（只看比它更早的事件），limit 是一页最多几条；
// types 是用逗号分隔的事件类型，exclude_types 是不要返回的事件类型（同样用逗号分隔），actor_kind 和 actor 是操作者类型和名称，card_id 只看某个任务的事件，不填都表示不限。
type activityInput struct {
	boardPath
	Before       int64  `form:"before" json:"-"`
	Limit        int    `form:"limit" json:"-"`
	Types        string `form:"types" json:"-"`
	ExcludeTypes string `form:"exclude_types" json:"-"`
	ActorKind    string `form:"actor_kind" json:"-"`
	Actor        string `form:"actor" json:"-"`
	CardID       int64  `form:"card_id" json:"-"`
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

// snapshot 一次返回整块看板：看板、列、规则、全部任务和读取之前的最新事件序号。
func (a API) snapshot(ctx context.Context, _ kanban.Actor, in *boardPath) (*kanban.Snapshot, error) {
	return a.core.BoardSnapshot(ctx, in.BoardID)
}

// listEvents 返回起点序号之后的事件，不做过滤，网页的活动列表和断线补发用。
func (a API) listEvents(ctx context.Context, _ kanban.Actor, in *listEventsInput) (*kanban.EventPage, error) {
	return a.core.ListEvents(ctx, in.BoardID, in.After, in.Limit)
}

// activity 返回看板的活动流：从新到旧，可按事件类型、操作者和任务筛选，网页的活动抽屉和任务详情用。
// types 和 exclude_types 里出现空项（比如多写了逗号）会被当成不认识的类型，返回 400。
func (a API) activity(ctx context.Context, _ kanban.Actor, in *activityInput) (*kanban.ActivityPage, error) {
	q := kanban.ActivityQuery{Before: in.Before, Limit: in.Limit, ActivityFilter: kanban.ActivityFilter{
		ActorKind: kanban.ActorKind(in.ActorKind),
		Actor:     in.Actor,
		CardID:    in.CardID,
	}}
	if in.Types != "" {
		q.Types = strings.Split(in.Types, ",")
	}
	if in.ExcludeTypes != "" {
		q.ExcludeTypes = strings.Split(in.ExcludeTypes, ",")
	}
	return a.core.ListActivity(ctx, in.BoardID, q)
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
