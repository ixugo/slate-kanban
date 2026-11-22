package mcpapi

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"slate-kanban/internal/core/kanban"
)

const (
	// defaultNotifyDelay 是通知的合并窗口：被唤醒后等这么久再发，这段时间内同一个看板的多次更新只发一条。
	// 自动化规则连锁触发、人批量改任务时，agent 不会被一串通知淹没，而 200 毫秒对"有更新"的提示来说感觉不到延迟。
	defaultNotifyDelay = 200 * time.Millisecond
	// observerKey 是在 Core 里注册观察者用的名字，同一个 Core 上只能注册一次。
	observerKey = "mcp"
	// sendTimeout 是发一个看板的通知最多花多久。通知依次发给每个订阅的会话，
	// 个别客户端卡住时不能一直拖住后面的看板。
	sendTimeout = 10 * time.Second
)

// notifier 把看板的更新合并后，以资源更新通知的形式发给订阅了这个看板的 MCP 会话。
// 它只是"有更新"的提示：不带数据，也不保证送达，agent 收到后用 pull_updates 取数据；
// 通知丢了，下一次 pull_updates 或 wait_for_updates 照样能读到全部更新。
type notifier struct {
	servers []*mcp.Server // 两个视图各自管自己的订阅，所以每个都要发一遍
	delay   time.Duration

	mu    sync.Mutex
	dirty map[int64]int64 // 有更新还没通知的看板：看板编号 -> 最大事件序号
	wake  chan struct{}   // 容量为 1，表示"有看板脏了"

	ctx    context.Context // 通知器停止时被取消
	cancel context.CancelFunc
	done   chan struct{} // 发送协程退出后关闭
}

// newNotifier 创建通知器并启动它的发送协程。用完要调用 close。
func newNotifier(delay time.Duration, servers ...*mcp.Server) *notifier {
	ctx, cancel := context.WithCancel(context.Background())
	n := &notifier{
		servers: servers, delay: delay, dirty: make(map[int64]int64), wake: make(chan struct{}, 1),
		ctx: ctx, cancel: cancel, done: make(chan struct{}),
	}
	go n.run()
	return n
}

// mark 是注册到 Core 的观察者。Core 在提交写入的请求协程里同步调用它，并且它返回的错误会让写请求失败，
// 所以它只做两件事：把看板和序号记进一个小表、往容量为 1 的通道里非阻塞地放一个信号，然后立刻返回 nil。
// 真正的发送由 run 完成，写请求不会被通知拖慢。
func (n *notifier) mark(_ context.Context, p kanban.Published) error {
	n.mu.Lock()
	if seq, seen := n.dirty[p.BoardID]; !seen || p.Seq > seq {
		n.dirty[p.BoardID] = p.Seq
	}
	n.mu.Unlock()
	select {
	case n.wake <- struct{}{}:
	default:
	}
	return nil
}

// run 是发送协程：等到有看板脏了，再等一个合并窗口，然后一次取走全部脏看板逐个通知。
// 发送期间新来的更新会再放一个信号，下一轮循环马上处理，不会漏。
func (n *notifier) run() {
	defer close(n.done)
	for {
		select {
		case <-n.ctx.Done():
			return
		case <-n.wake:
		}
		select {
		case <-n.ctx.Done():
			return
		case <-time.After(n.delay):
		}
		for boardID, seq := range n.take() {
			n.send(boardID, seq)
		}
	}
}

// take 取走全部脏看板并清空登记表。
func (n *notifier) take() map[int64]int64 {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := n.dirty
	n.dirty = make(map[int64]int64)
	return out
}

// send 给订阅了这个看板的会话发通知，latest 放在 _meta 里，表示这次通知涵盖到哪个事件序号。
// 看板被删除时序号是 0。没有人订阅时 SDK 什么都不发。
func (n *notifier) send(boardID, latest int64) {
	ctx, cancel := context.WithTimeout(n.ctx, sendTimeout)
	defer cancel()
	params := &mcp.ResourceUpdatedNotificationParams{URI: boardURI(boardID), Meta: mcp.Meta{"latest": latest}}
	for _, s := range n.servers {
		if err := s.ResourceUpdated(ctx, params); err != nil {
			slog.Warn("发送看板更新通知失败", "board_id", boardID, "err", err)
		}
	}
}

// close 停止发送协程并等它退出，可以重复调用。没发出去的通知直接丢弃，反正 agent 重连后会用 pull_updates 补读。
func (n *notifier) close() {
	n.cancel()
	<-n.done
}
