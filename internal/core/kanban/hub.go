package kanban

import (
	"context"
	"sync"

	"github.com/ixugo/goddd/pkg/event"
)

// Published 是"某个看板有更新"的广播内容。它只是个唤醒信号，不携带只此一份的数据：
// 订阅者被唤醒后要自己去查事件表，所以信号丢了也不会丢数据。
// Seq 是这次提交产生的最大事件序号；删除看板时没有事件，Seq 为 0。
type Published struct {
	BoardID int64
	Seq     int64
}

// Hub 在进程内把"某个看板有更新"的信号分发给两类接收者：
// 一类是正在阻塞等待的请求（Subscribe），另一类是长期存在的观察者（Register），
// 例如 MCP 层要据此给已订阅的客户端发资源更新通知。
type Hub struct {
	mu   sync.Mutex
	subs map[int64]map[*subscription]struct{}
	bus  *event.Bus[Published]
}

// subscription 是一个等待者。通道容量为 1：信号只表示"有更新"，
// 多次更新合并成一次唤醒，等待者醒来后一次查库就能取回全部新事件。
type subscription struct {
	ch chan struct{}
}

// NewHub 创建一个空的唤醒器。
func NewHub() *Hub {
	return &Hub{
		subs: make(map[int64]map[*subscription]struct{}),
		bus:  event.NewBus[Published](),
	}
}

// Subscribe 订阅某个看板的更新信号，返回信号通道和取消函数。
// 等待结束后必须调用取消函数，否则订阅者会一直留在内存里；取消函数可以重复调用。
func (h *Hub) Subscribe(boardID int64) (<-chan struct{}, func()) {
	s := &subscription{ch: make(chan struct{}, 1)}
	h.mu.Lock()
	if h.subs[boardID] == nil {
		h.subs[boardID] = make(map[*subscription]struct{})
	}
	h.subs[boardID][s] = struct{}{}
	h.mu.Unlock()

	cancel := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.subs[boardID], s)
		if len(h.subs[boardID]) == 0 {
			delete(h.subs, boardID)
		}
	}
	return s.ch, cancel
}

// Register 注册一个长期观察者。key 重复会 panic，防止悄悄覆盖；
// 应该只在程序启动、开始处理请求之前调用。
func (h *Hub) Register(key string, fn func(context.Context, Published) error) {
	h.bus.Register(key, fn)
}

// publish 先唤醒所有等待者，再通知观察者。唤醒用非阻塞发送，
// 某个等待者没来得及取走上一次信号时，这一次直接合并，不会拖慢提交请求的流程。
func (h *Hub) publish(ctx context.Context, p Published) error {
	h.mu.Lock()
	for s := range h.subs[p.BoardID] {
		select {
		case s.ch <- struct{}{}:
		default:
		}
	}
	h.mu.Unlock()
	return h.bus.Notify(ctx, p)
}
