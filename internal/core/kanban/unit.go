package kanban

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/reason"
)

const (
	// eventKeep 每个看板最多保留的事件条数。agent 离线再久，也只能补拉到这个范围内的变化；
	// 更早的变化由它重新读取整块看板来获得，所以不需要无限保留。
	eventKeep = 10000
	// eventPruneEvery 事件序号是这个数的整数倍时，顺带清理一次旧事件。
	// 不是每次写入都清理，是为了让清理的开销摊薄到几百次写入里。
	eventPruneEvery = 500
)

// unit 是一次写事务用到的全部存储和中间状态。每次调用 run 都会新建一个，
// 事务结束就丢弃，只被一个 goroutine 使用，所以内部不需要加锁。
type unit struct {
	core            Core
	board           BoardStorer
	column          ColumnStorer
	card            CardStorer
	cardChange      CardChangeStorer
	comment         CommentStorer
	rule            RuleStorer
	event           EventStorer
	cursor          CursorStorer
	attachment      AttachmentStorer
	tag             TagStorer
	engineer        EngineerStorer
	projectTemplate ProjectTemplateStorer
	webhook         WebhookStorer
	touched         map[int64]int64 // 本事务动过的看板 → 其中最大的事件序号，提交后据此广播
	emitted         []*Event        // 本事务追加的事件，提交后用于 webhook 推送
	after           []func()        // 事务提交成功后要执行的收尾动作，例如删除图片文件
}

// newEvent 是追加一条事件时由调用方提供的内容，操作者、时间和序号由 emit 补全。
type newEvent struct {
	BoardID int64
	CardID  int64 // 和卡片无关的事件填 0
	Type    string
	Payload any // 会被编码成 JSON 对象；nil 表示没有内容，存成空对象
}

// newUnit 为一个事务创建它专用的各类存储，全部是 WithTx 副本，保证都跑在同一个事务连接上。
func newUnit(c Core, tx orm.Tx) *unit {
	return &unit{
		core:            c,
		board:           c.store.Board().WithTx(tx),
		column:          c.store.Column().WithTx(tx),
		card:            c.store.Card().WithTx(tx),
		cardChange:      c.store.CardChange().WithTx(tx),
		comment:         c.store.Comment().WithTx(tx),
		rule:            c.store.Rule().WithTx(tx),
		event:           c.store.Event().WithTx(tx),
		cursor:          c.store.Cursor().WithTx(tx),
		attachment:      c.store.Attachment().WithTx(tx),
		tag:             c.store.Tag().WithTx(tx),
		engineer:        c.store.Engineer().WithTx(tx),
		projectTemplate: c.store.ProjectTemplate().WithTx(tx),
		webhook:         c.store.Webhook().WithTx(tx),
		touched:         make(map[int64]int64),
	}
}

// now 返回当前时间（UTC）。统一从 Core 的时钟取，测试里才能固定时间。
func (u *unit) now() time.Time {
	return u.core.now().UTC()
}

// run 是所有写操作的统一入口：开事务 → 锁看板行 → 执行 fn → 提交 → 广播。
//
// boardID 不为 0 时，事务第一步就锁住这个看板的行，同一看板的写事务因此排队，
// 事件序号的先后就等于提交的先后（PostgreSQL 下这一点才不会乱）；
// 看板不存在时直接返回 ErrBoardNotFound，fn 不会被调用。
// 创建看板时还没有旧行可锁，传 0 即可。
//
// fn 返回错误或 panic 都会回滚，已追加的事件一并撤销。
// 广播发生在提交成功之后，所以收到信号的人一定能查到数据；广播出错只记日志，不影响本次结果。
func (c Core) run(ctx context.Context, boardID int64, fn func(*unit) error) error {
	tx, err := c.store.Begin()
	if err != nil {
		return wrapDB("开启事务", err)
	}
	finished := false
	defer func() {
		if finished {
			return
		}
		if rbErr := tx.Rollback(); rbErr != nil {
			slog.ErrorContext(ctx, "回滚事务失败", "err", rbErr)
		}
	}()

	u := newUnit(c, tx)
	if boardID != 0 {
		if _, err := u.board.Lock(ctx, boardID); err != nil {
			return mapNotFound(err, ErrBoardNotFound, "锁定看板")
		}
	}
	if err := fn(u); err != nil {
		return err
	}
	finished = true // 无论提交成败，事务都已经结束，不需要再回滚
	if err := tx.Commit(); err != nil {
		return wrapDB("提交事务", err)
	}
	for _, fn := range u.after {
		fn()
	}
	if len(u.emitted) > 0 {
		c.scheduleWebhookNotify(ctx, u.emitted)
	}
	c.publish(ctx, u)
	return nil
}

// afterCommit 登记一个收尾动作，事务提交成功后按登记顺序执行；回滚时不会执行。
// 文件的删除不能回滚，所以必须放在提交之后，否则事务失败了文件却已经没了。
func (u *unit) afterCommit(fn func()) {
	u.after = append(u.after, fn)
}

// publish 把本事务动过的每个看板各广播一次，只带最大的事件序号。
func (c Core) publish(ctx context.Context, u *unit) {
	for boardID, seq := range u.touched {
		if err := c.hub.publish(ctx, Published{BoardID: boardID, Seq: seq}); err != nil {
			slog.WarnContext(ctx, "广播看板更新失败", "board_id", boardID, "err", err)
		}
	}
}

// touch 标记某个看板在本事务里有变化，但不产生事件。
// 删除看板用它：事件随看板一起删了，仍然要让等待者醒来发现看板没了。
func (u *unit) touch(boardID int64) {
	u.touched[boardID] = max(u.touched[boardID], 0)
}

// emit 在当前事务里追加一条事件，并记下该看板要广播的最大序号。
// 序号等于事件表主键，由数据库保证递增。每攒够 eventPruneEvery 的整数倍就顺带清理旧事件。
func (u *unit) emit(ctx context.Context, actor Actor, ev newEvent) error {
	payload := "{}"
	if ev.Payload != nil {
		b, err := json.Marshal(ev.Payload)
		if err != nil {
			return reason.ErrJSON.WithCause(err).Withf("编码事件内容 %s", ev.Type)
		}
		payload = string(b)
	}
	e := &Event{
		BoardID: ev.BoardID, CardID: ev.CardID, Type: ev.Type,
		ActorKind: string(actor.Kind), Actor: actor.Name,
		Payload: payload, CreatedAt: u.now(),
	}
	if err := u.event.Create(ctx, e); err != nil {
		return wrapDB("追加事件", err)
	}
	u.emitted = append(u.emitted, e)
	u.touched[ev.BoardID] = max(u.touched[ev.BoardID], e.ID)

	if ev.CardID != 0 && ev.Type != EventCardCreated && ev.Type != EventCardDeleted {
		if err := u.card.IncrUnreadCount(ctx, ev.CardID); err != nil {
			return wrapDB("递增任务未读数", err)
		}
	}

	if u.core.pruneEvery > 0 && e.ID%u.core.pruneEvery == 0 {
		if err := u.event.Prune(ctx, ev.BoardID, u.core.keepEvents); err != nil {
			return wrapDB("清理旧事件", err)
		}
	}
	return nil
}
