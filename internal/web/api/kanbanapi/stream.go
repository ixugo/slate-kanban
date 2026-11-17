package kanbanapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ixugo/goddd/pkg/reason"
	"github.com/ixugo/goddd/pkg/web"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/lifecycle"
)

const (
	// streamHeartbeat 是实时流发保活注释行的间隔：既防止代理因为空闲断开连接，也能借写失败发现客户端已经走了。
	streamHeartbeat = 20 * time.Second
	// streamMaxAge 是单次实时连接的最长时间。到点服务端主动结束，客户端带着最后的事件序号重连，
	// 这样被删除的令牌最多再多读 10 分钟，半开的连接也不会一直占着资源。
	streamMaxAge = 10 * time.Minute
)

// 实时流里的事件名，含义见设计文档 api.rest.go.md。
const (
	sseReady = "ready" // 连上之后的第一条，带当前最新序号
	sseBoard = "board" // 一条看板事件
	sseReset = "reset" // 续传起点不可用，客户端要重新取快照
	sseError = "error" // 流开始之后出错，比如看板被删了
)

// stream 处理实时流请求 GET /boards/:board_id/stream。
// 起点的确定和看板是否存在的检查都在写响应头之前完成，所以这些错误还能返回正常的 JSON 错误；
// 响应头写出去之后出的错只能用 error 事件通知客户端。服务开始关闭时流直接结束，不发事件，
// 客户端带着最后收到的序号重连就能接上。
func (a API) stream(c *gin.Context) {
	boardID, err := strconv.ParseInt(c.Param("board_id"), 10, 64)
	if err != nil {
		web.Fail(c, reason.ErrBadRequest.WithMsg("看板编号不合法"))
		return
	}
	after, hasAfter, err := streamCursor(c)
	if err != nil {
		web.Fail(c, err)
		return
	}
	ctx := c.Request.Context()
	latest, err := a.core.LatestSeq(ctx, boardID)
	if err != nil {
		web.Fail(c, err)
		return
	}
	s := &streamSession{
		core: a.core, w: sseWriter{c: c}, boardID: boardID, from: latest,
		heartbeat: a.heartbeat, maxAge: a.maxAge,
	}
	if hasAfter {
		s.from = after
	}
	s.w.start()
	if s.send(sseReady, "", map[string]int64{"latest": latest}) != nil {
		return
	}
	if s.from > latest {
		// 客户端记着的序号比服务端的还大，说明服务端的数据回退过（比如从备份恢复），数据已经对不上了
		s.finish(ctx, sseReset, map[string]int64{"latest": latest})
		return
	}
	// 长时间的循环用绑定后的上下文：服务开始关闭时流也跟着结束，不会拖住关闭
	bound, cancel := lifecycle.Bind(ctx)
	defer cancel()
	s.run(bound)
}

// streamCursor 读出续传位置：查询参数 after 优先，其次是请求头 Last-Event-ID。
// 两个都没给时 ok 为 false，表示从当前最新开始；给了但不是不小于 0 的整数返回 400。
func streamCursor(c *gin.Context) (seq int64, ok bool, err error) {
	raw := c.Query("after")
	if raw == "" {
		raw = c.GetHeader("Last-Event-ID")
	}
	if raw == "" {
		return 0, false, nil
	}
	seq, err = strconv.ParseInt(raw, 10, 64)
	if err != nil || seq < 0 {
		return 0, false, reason.ErrBadRequest.WithMsg("after 或 Last-Event-ID 必须是不小于 0 的整数")
	}
	return seq, true, nil
}

// sseWriter 把内容按 text/event-stream 的格式写给客户端。
type sseWriter struct {
	c *gin.Context
}

// start 写响应头。X-Accel-Buffering 让 nginx 不要缓冲，否则事件会被攒着不发。
func (w sseWriter) start() {
	h := w.c.Writer.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("X-Accel-Buffering", "no")
	w.c.Status(http.StatusOK)
}

// event 写一条事件，数据是单行 JSON；id 为空就不写 id 行。写完不刷新，由调用方决定什么时候 flush。
func (w sseWriter) event(name, id string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	var b strings.Builder
	if id != "" {
		b.WriteString("id: " + id + "\n")
	}
	b.WriteString("event: " + name + "\ndata: ")
	b.Write(raw)
	b.WriteString("\n\n")
	_, err = io.WriteString(w.c.Writer, b.String())
	return err
}

// comment 写一条注释行，客户端会忽略它，只用来保活和发现断线。
func (w sseWriter) comment(text string) error {
	_, err := io.WriteString(w.c.Writer, ": "+text+"\n\n")
	return err
}

// flush 把已经写的内容立刻送到客户端。
func (w sseWriter) flush() {
	w.c.Writer.Flush()
}

// streamSession 是一次实时连接的状态：往哪里写、读哪个看板、已经发到了哪个序号。
type streamSession struct {
	core      kanban.Core
	w         sseWriter
	boardID   int64
	from      int64 // 已经发给客户端的最大事件序号，下一页从它之后读
	heartbeat time.Duration
	maxAge    time.Duration
}

// send 写一条事件并立刻刷新。
func (s *streamSession) send(name, id string, data any) error {
	if err := s.w.event(name, id, data); err != nil {
		return err
	}
	s.w.flush()
	return nil
}

// finish 发出流结束前的最后一条事件。发不出去说明客户端已经走了，没有别的可做，只记一条调试日志。
func (s *streamSession) finish(ctx context.Context, name string, data any) {
	if err := s.send(name, "", data); err != nil {
		slog.DebugContext(ctx, "实时流的结束事件没能发出去", "event", name, "err", err)
	}
}

// run 是流的主循环：先订阅更新信号，再读事件，读完就等信号。必须先订阅再读，
// 这样读库和等待之间提交的事件，信号会留在订阅里，不会漏。
// 客户端断开、连接到期、写失败、出错或需要重取快照时返回。
func (s *streamSession) run(ctx context.Context) {
	signal, cancel := s.core.Subscribe(s.boardID)
	defer cancel()
	heartbeat := time.NewTicker(s.heartbeat)
	defer heartbeat.Stop()
	expire := time.NewTimer(s.maxAge)
	defer expire.Stop()
	for {
		more, stop := s.sendPage(ctx)
		if stop {
			return
		}
		if more {
			continue
		}
		if !s.wait(ctx, signal, heartbeat, expire) {
			return
		}
	}
}

// sendPage 读起点之后的一页事件发给客户端。more 表示这一页之后还有事件，应该马上再读；
// stop 表示流应该结束。起点早于保留范围发 reset，读库出错发 error，客户端已经断开就什么也不发。
func (s *streamSession) sendPage(ctx context.Context) (more, stop bool) {
	page, err := s.core.ListEvents(ctx, s.boardID, s.from, 0)
	if err != nil {
		if ctx.Err() == nil {
			s.sendError(ctx, err)
		}
		return false, true
	}
	if page.Truncated {
		s.finish(ctx, sseReset, map[string]int64{"latest": page.Latest})
		return false, true
	}
	for _, ev := range page.Events {
		if s.w.event(sseBoard, strconv.FormatInt(ev.ID, 10), ev) != nil {
			return false, true
		}
		s.from = ev.ID
	}
	s.w.flush()
	return page.HasMore, false
}

// sendError 把错误作为 error 事件发出去，内容和 REST 错误响应的 reason、msg 一致。
func (s *streamSession) sendError(ctx context.Context, err error) {
	out := map[string]string{"reason": reason.ErrInternal.GetReason(), "msg": reason.ErrInternal.GetMessage()}
	var info reason.ErrorInfoer
	if errors.As(err, &info) {
		out["reason"], out["msg"] = info.GetReason(), info.GetMessage()
	}
	s.finish(ctx, sseError, out)
}

// wait 等到有更新信号返回 true；等待期间按间隔发保活注释行；
// 连接到期、客户端断开或写保活失败返回 false。
func (s *streamSession) wait(ctx context.Context, signal <-chan struct{}, heartbeat *time.Ticker, expire *time.Timer) bool {
	for {
		select {
		case <-signal:
			return true
		case <-heartbeat.C:
			if s.w.comment("ping") != nil {
				return false
			}
			s.w.flush()
		case <-expire.C:
			return false
		case <-ctx.Done():
			return false
		}
	}
}
