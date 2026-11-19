package kanban

import (
	"context"
	"time"

	"github.com/ixugo/goddd/pkg/orm"
)

// Storer 是看板领域的数据持久化聚合接口，每个实体一个访问器。
// Begin 开启事务；事务里必须使用各访问器的 WithTx 副本，
// 不能直接用聚合上的访问器，否则在只有一个连接的 SQLite 上会把自己卡死。
type Storer interface {
	Begin() (orm.Tx, error)
	Board() BoardStorer
	Column() ColumnStorer
	Card() CardStorer
	Comment() CommentStorer
	Rule() RuleStorer
	Event() EventStorer
	Cursor() CursorStorer
	Attachment() AttachmentStorer
	Tag() TagStorer
}

// Core 是看板领域的业务入口，是个很小的值类型，可以直接按值传递。
// 人和 agent 共用它的全部方法，区别只在调用时传入的 Actor。
type Core struct {
	store      Storer
	now        func() time.Time
	hub        *Hub           // 按指针共享，Core 被复制后仍然是同一个唤醒器
	keepEvents int            // 每个看板保留的事件条数
	pruneEvery int64          // 事件序号是它的整数倍时顺带清理旧事件
	resources  ResourceStorer // 图片等资源文件的存放处，没配置时不能上传图片
}

// Option 修改 Core 的可选行为。
type Option func(*Core)

// WithClock 替换 Core 使用的时钟。测试里用它固定时间，生产环境不需要设置。
func WithClock(now func() time.Time) Option {
	return func(c *Core) { c.now = now }
}

// WithResources 设置图片等资源文件的存放处。不设置时上传和读取图片都会报错。
func WithResources(r ResourceStorer) Option {
	return func(c *Core) { c.resources = r }
}

// NewCore 创建看板领域的业务入口，默认使用系统时钟。
func NewCore(store Storer, opts ...Option) Core {
	c := Core{store: store, now: time.Now, hub: NewHub(), keepEvents: eventKeep, pruneEvery: eventPruneEvery}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

// Subscribe 订阅某个看板的更新信号，用法和注意事项见 Hub.Subscribe。
// 等待更新的接口先订阅再查库，这样查库和等待之间发生的更新不会丢。
func (c Core) Subscribe(boardID int64) (<-chan struct{}, func()) {
	return c.hub.Subscribe(boardID)
}

// Observe 注册一个长期观察者，每次有看板提交更新就会被调用，例如 MCP 层据此发资源更新通知。
// 只能在程序启动、开始处理请求之前调用；key 重复会 panic。
func (c Core) Observe(key string, fn func(context.Context, Published) error) {
	c.hub.Register(key, fn)
}
