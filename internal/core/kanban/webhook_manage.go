package kanban

import (
	"context"

	"github.com/ixugo/goddd/pkg/orm"
)

// ListWebhooks 列出看板的全部 webhook，只有人能管理但 agent 也可读配置用于排障。
func (c Core) ListWebhooks(ctx context.Context, actor Actor, boardID int64) ([]WebhookView, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	if _, err := c.store.Board().GetByID(ctx, boardID); err != nil {
		return nil, mapNotFound(err, ErrBoardNotFound, "读取看板")
	}
	items, err := c.store.Webhook().ListByBoard(ctx, boardID)
	if err != nil {
		return nil, wrapDB("读取 webhook", err)
	}
	out := make([]WebhookView, len(items))
	for i, w := range items {
		out[i] = webhookView(w)
	}
	return out, nil
}

// CreateWebhook 新增 webhook，只允许人操作。
func (c Core) CreateWebhook(ctx context.Context, actor Actor, boardID int64, in CreateWebhookInput) (*WebhookView, error) {
	if err := requireHuman(actor); err != nil {
		return nil, err
	}
	var out *WebhookView
	err := c.run(ctx, boardID, func(u *unit) error {
		cols, err := u.column.ListByBoard(ctx, boardID)
		if err != nil {
			return wrapDB("读取列", err)
		}
		items, err := u.webhook.ListByBoard(ctx, boardID)
		if err != nil {
			return wrapDB("读取 webhook", err)
		}
		if len(items) >= MaxWebhooksPerBoard {
			return ErrWebhookLimit
		}
		now := u.now()
		enabled := true
		if in.Enabled != nil {
			enabled = *in.Enabled
		}
		w := &Webhook{
			BoardID: boardID, Name: in.Name, Type: in.Type, URL: in.URL, Secret: in.Secret, Enabled: enabled,
			StrictFilters: in.StrictFilters, StrictFilterModes: in.StrictFilterModes, EventTypes: in.EventTypes, ActorKinds: in.ActorKinds, MoveColumnIDs: in.MoveColumnIDs,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := normalizeWebhook(w, cols); err != nil {
			return err
		}
		if err := u.webhook.Create(ctx, w); err != nil {
			return wrapDB("创建 webhook", err)
		}
		view := webhookView(w)
		out = &view
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateWebhook 修改 webhook，只允许人。
func (c Core) UpdateWebhook(ctx context.Context, actor Actor, webhookID int64, in UpdateWebhookInput) (*WebhookView, error) {
	if err := requireHuman(actor); err != nil {
		return nil, err
	}
	found, err := c.store.Webhook().GetByID(ctx, webhookID)
	if err != nil {
		return nil, mapNotFound(err, ErrWebhookNotFound, "读取 webhook")
	}
	var out *WebhookView
	err = c.run(ctx, found.BoardID, func(u *unit) error {
		w, err := u.webhook.GetByID(ctx, webhookID)
		if err != nil {
			return mapNotFound(err, ErrWebhookNotFound, "读取 webhook")
		}
		before := *w
		in.apply(w)
		cols, err := u.column.ListByBoard(ctx, w.BoardID)
		if err != nil {
			return wrapDB("读取列", err)
		}
		if err := normalizeWebhook(w, cols); err != nil {
			return err
		}
		if sameWebhook(&before, w) {
			view := webhookView(w)
			out = &view
			return nil
		}
		w.UpdatedAt = u.now()
		if err := u.webhook.Save(ctx, w); err != nil {
			return wrapDB("保存 webhook", err)
		}
		view := webhookView(w)
		out = &view
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// sameWebhook 判断两条 webhook 可配置内容是否一致。
func sameWebhook(a, b *Webhook) bool {
	return a.Name == b.Name && a.Type == b.Type && a.URL == b.URL && a.Secret == b.Secret &&
		a.Enabled == b.Enabled && a.StrictFilters == b.StrictFilters &&
		sameOptionalBool(a.StrictFilterModes.EventTypes, b.StrictFilterModes.EventTypes) &&
		sameOptionalBool(a.StrictFilterModes.ActorKinds, b.StrictFilterModes.ActorKinds) &&
		sameOptionalBool(a.StrictFilterModes.MoveColumnIDs, b.StrictFilterModes.MoveColumnIDs) &&
		sliceEqual(a.EventTypes, b.EventTypes) && sliceEqual(a.ActorKinds, b.ActorKinds) && int64SliceEqual(a.MoveColumnIDs, b.MoveColumnIDs)
}

func sameOptionalBool(a, b *bool) bool {
	return a == b || a != nil && b != nil && *a == *b
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func int64SliceEqual(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// DeleteWebhook 删除 webhook，只允许人，幂等。
func (c Core) DeleteWebhook(ctx context.Context, actor Actor, webhookID int64) error {
	if err := requireHuman(actor); err != nil {
		return err
	}
	found, err := c.store.Webhook().GetByID(ctx, webhookID)
	if orm.IsErrRecordNotFound(err) {
		return nil
	}
	if err != nil {
		return wrapDB("读取 webhook", err)
	}
	return c.run(ctx, found.BoardID, func(u *unit) error {
		if _, err := u.webhook.GetByID(ctx, webhookID); orm.IsErrRecordNotFound(err) {
			return nil
		} else if err != nil {
			return wrapDB("读取 webhook", err)
		}
		return u.webhook.Delete(ctx, webhookID)
	})
}

// TestWebhook 发送一条测试消息，只有人能调用。
func (c Core) TestWebhook(ctx context.Context, actor Actor, webhookID int64) (*TestWebhookResult, error) {
	if err := requireHuman(actor); err != nil {
		return nil, err
	}
	w, err := c.store.Webhook().GetByID(ctx, webhookID)
	if err != nil {
		return nil, mapNotFound(err, ErrWebhookNotFound, "读取 webhook")
	}
	board, err := c.store.Board().GetByID(ctx, w.BoardID)
	if err != nil {
		return nil, mapNotFound(err, ErrBoardNotFound, "读取看板")
	}
	text := "【" + board.Name + "】这是 Slate 看板的活动通知测试消息。\n如果你看到这条消息，说明 webhook 配置正确。"
	if err := SendWebhookText(ctx, w, text); err != nil {
		return &TestWebhookResult{OK: false, Message: err.Error()}, nil
	}
	return &TestWebhookResult{OK: true, Message: "发送成功"}, nil
}
