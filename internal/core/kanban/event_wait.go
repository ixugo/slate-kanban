package kanban

import (
	"context"
	"time"
)

const (
	// defaultWaitTimeout 阻塞等待更新时，没指定超时的默认等待时间。
	defaultWaitTimeout = 30 * time.Second
	// maxWaitTimeout 阻塞等待的最长时间。常见的反向代理和客户端会在一分钟左右切断空闲连接，
	// 所以上限定在 50 秒，超时后调用方立刻再等一次即可。
	maxWaitTimeout = 50 * time.Second
)

// WaitInput 是 agent 阻塞等待更新的参数：拉取参数加一个最长等待时间。
type WaitInput struct {
	PullInput
	// Timeout 最长等多久，0 取默认值，超过上限取上限。接口层负责把请求里的秒数换算成它。
	Timeout time.Duration
}

// normalizeWaitTimeout 把调用方给的等待时间换成实际使用的时间：0 取默认值，超过上限取上限，负数返回 400。
func normalizeWaitTimeout(d time.Duration) (time.Duration, error) {
	switch {
	case d < 0:
		return 0, invalidf("timeout 不能是负数")
	case d == 0:
		return defaultWaitTimeout, nil
	}
	return min(d, maxWaitTimeout), nil
}

// WaitUpdates 和 PullUpdates 一样返回 agent 还没读到的事件，区别是现在没有值得通知的事件时会阻塞等待，
// 直到有新事件、超时或者请求被取消。agent 用它代替定时轮询：一直挂着，有事立刻醒。
// 需要立刻告诉调用方的情况不等待：第一次读这个看板（first）和离线太久事件被清理（truncated）。
// 超时不是错误，返回空事件并标记 TimedOut，调用方带上返回的进度立刻再等一次即可。
// 请求被取消返回取消错误；等待期间看板被删除返回 404。
func (c Core) WaitUpdates(ctx context.Context, actor Actor, boardID int64, in WaitInput) (*Updates, error) {
	if err := requireAgent(actor); err != nil {
		return nil, err
	}
	limit, err := normalizeLimit(in.Limit, defaultPullLimit, maxPullLimit)
	if err != nil {
		return nil, err
	}
	timeout, err := normalizeWaitTimeout(in.Timeout)
	if err != nil {
		return nil, err
	}
	// 必须先订阅再查库：查库之后、开始等待之前发生的更新，信号会留在通道里，不会丢。
	wake, cancel := c.Subscribe(boardID)
	defer cancel()
	out, err := c.PullUpdates(ctx, actor, boardID, in.PullInput)
	if err != nil || out.First || out.Truncated || len(out.Events) > 0 {
		return out, err
	}
	w := updateWaiter{core: c, agent: actor.Name, boardID: boardID, limit: limit, wake: wake}
	return w.wait(ctx, out, timeout)
}

// updateWaiter 是一次阻塞等待的固定上下文：谁在等、等哪个看板、一页最多读多少、从哪里收更新信号。
type updateWaiter struct {
	core    Core
	agent   string
	boardID int64
	limit   int
	wake    <-chan struct{}
}

// wait 在已经订阅的前提下反复"等信号、读事件"，直到读到值得通知的事件、超时或请求被取消。
// out 是刚才那次没有值得通知事件的结果，下一次从它的进度之后读，不用重读已经过滤掉的事件。
// 收到信号但事件全被过滤掉（例如是 agent 自己的操作）时继续等；上一页之后还有事件时不等信号，直接读下一页。
func (w updateWaiter) wait(ctx context.Context, out *Updates, timeout time.Duration) (*Updates, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !out.HasMore {
			select {
			case <-w.wake:
			case <-timer.C:
				out.TimedOut = true
				return out, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		if _, err := w.core.store.Board().GetByID(ctx, w.boardID); err != nil {
			return nil, mapNotFound(err, ErrBoardNotFound, "读取看板")
		}
		next, err := w.core.scanUpdates(ctx, w.agent, w.boardID, out.Cursor, w.limit)
		if err != nil {
			return nil, err
		}
		if len(next.Events) > 0 || next.Truncated {
			return next, nil
		}
		out = next
	}
}
