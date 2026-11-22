package kanban

import (
	"context"
	"log/slog"
	"sync"
)

// webhookNotifyHook 在事务提交后异步推送事件，测试可替换为同步以便断言。
var webhookNotifyHook = defaultWebhookNotifyHook

// SetWebhookNotifyHookForTest 替换异步推送实现，返回恢复函数。
func SetWebhookNotifyHookForTest(fn func(Core, context.Context, []*Event)) func() {
	prev := webhookNotifyHook
	webhookNotifyHook = fn
	return func() { webhookNotifyHook = prev }
}

// defaultWebhookNotifyHook 在独立 goroutine 里推送，避免阻塞主请求。
func defaultWebhookNotifyHook(c Core, parent context.Context, events []*Event) {
	if len(events) == 0 {
		return
	}
	go c.deliverWebhooks(context.WithoutCancel(parent), events)
}

// scheduleWebhookNotify 登记本事务产生的事件，在提交后触发推送。
func (c Core) scheduleWebhookNotify(ctx context.Context, events []*Event) {
	if len(events) == 0 {
		return
	}
	copied := append([]*Event(nil), events...)
	webhookNotifyHook(c, ctx, copied)
}

// DeliverWebhooksForTest 供测试同步调用推送逻辑。
func (c Core) DeliverWebhooksForTest(ctx context.Context, events []*Event) {
	c.deliverWebhooks(ctx, events)
}

// deliverWebhooks 读取 webhook 并逐条发送匹配的事件；任何失败只记日志。
func (c Core) deliverWebhooks(ctx context.Context, events []*Event) {
	if len(events) == 0 {
		return
	}
	byBoard := groupEventsByBoard(events)
	for boardID, evs := range byBoard {
		c.deliverBoardWebhooks(ctx, boardID, evs)
	}
}

// groupEventsByBoard 按看板分组事件，保持原顺序。
func groupEventsByBoard(events []*Event) map[int64][]*Event {
	out := make(map[int64][]*Event)
	for _, ev := range events {
		out[ev.BoardID] = append(out[ev.BoardID], ev)
	}
	return out
}

// deliverBoardWebhooks 对单个看板推送一批事件。
func (c Core) deliverBoardWebhooks(ctx context.Context, boardID int64, events []*Event) {
	board, err := c.store.Board().GetByID(ctx, boardID)
	if err != nil {
		slog.WarnContext(ctx, "webhook 推送时读取看板失败", "board_id", boardID, "err", err)
		return
	}
	hooks, err := c.store.Webhook().ListByBoard(ctx, boardID)
	if err != nil {
		slog.WarnContext(ctx, "webhook 推送时读取配置失败", "board_id", boardID, "err", err)
		return
	}
	if len(hooks) == 0 {
		return
	}
	var wg sync.WaitGroup
	for _, w := range hooks {
		for _, ev := range events {
			if !webhookMatchesEvent(w, ev) {
				slog.DebugContext(ctx, "webhook 订阅条件不匹配，跳过",
					"board_id", boardID, "webhook_id", w.ID, "event_id", ev.ID,
					"event_type", ev.Type, "actor_kind", ev.ActorKind)
				continue
			}
			wg.Add(1)
			go func(wh *Webhook, event *Event) {
				defer wg.Done()
				text := FormatWebhookMessage(board.Name, event)
				if err := SendWebhookText(ctx, wh, text); err != nil {
					slog.WarnContext(ctx, "webhook 推送失败",
						"board_id", boardID, "webhook_id", wh.ID, "webhook_name", wh.Name,
						"event_id", event.ID, "event_type", event.Type, "url", wh.URL,
						"secret_set", wh.Secret != "", "err", err)
					return
				}
				slog.InfoContext(ctx, "webhook 推送成功",
					"board_id", boardID, "webhook_id", wh.ID, "event_id", event.ID,
					"event_type", event.Type, "actor_kind", event.ActorKind)
			}(w, ev)
		}
	}
	wg.Wait()
}
