package kanban

import "context"

// 本文件只在测试时编译，把包内的未导出能力暴露给外部测试包 kanban_test。

// PublishForTest 直接向唤醒器广播，用来测试订阅和观察者。
func PublishForTest(h *Hub, p Published) error {
	return h.publish(context.Background(), p)
}

// Unit 和 NewEvent 是事务执行器内部类型的别名，供测试在 fn 里使用。
type (
	Unit     = unit
	NewEvent = newEvent
)

// RunForTest 直接调用事务执行器。
func (c Core) RunForTest(ctx context.Context, boardID int64, fn func(*Unit) error) error {
	return c.run(ctx, boardID, fn)
}

// EmitForTest 在事务里追加一条事件。
func (u *unit) EmitForTest(ctx context.Context, actor Actor, ev NewEvent) error {
	return u.emit(ctx, actor, ev)
}

// TouchForTest 标记看板有变化但不产生事件。
func (u *unit) TouchForTest(boardID int64) {
	u.touch(boardID)
}

// WithRetentionForTest 缩小事件保留条数和清理间隔，让测试不用写上万条事件就能覆盖清理逻辑。
func WithRetentionForTest(keep int, every int64) Option {
	return func(c *Core) {
		c.keepEvents = keep
		c.pruneEvery = every
	}
}

// PlaceCardForTest 在事务里计算任务放进某一列的位置，必要时整列重排。
func (u *unit) PlaceCardForTest(ctx context.Context, cards []*Card, beforeID, afterID int64) (float64, error) {
	pos, _, err := u.placeCard(ctx, cards, beforeID, afterID)
	return pos, err
}
