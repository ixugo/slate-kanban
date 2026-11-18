package mcpapi_test

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"slate-kanban/internal/core/kanban"
)

// agentActor 是测试里直接调用 Core 时使用的 agent 身份，名称和 s.agent(t, "claude") 一致。
var agentActor = kanban.Actor{Kind: kanban.ActorAgent, Name: "claude"}

// types 取出事件列表里每个事件的类型。
func types(t *testing.T, data map[string]any) []string {
	t.Helper()
	var out []string
	for _, ev := range items(t, data, "events") {
		out = append(out, ev["type"].(string))
	}
	return out
}

// TestPullUpdates 验证拉取更新：第一次读不补发历史；之后能读到别人的改动；
// 不确认（不带 since）时重复读到同样的事件，确认之后才前进；自己的操作被过滤掉。
func TestPullUpdates(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	sess := s.agent(t, "claude")

	first := ok(t, sess, "pull_updates", map[string]any{"board_id": b.ID}).Data
	if first["first"] != true || len(items(t, first, "events")) != 0 {
		t.Fatalf("第一次读不应该补发历史: %v", first)
	}

	s.card(t, b, b.todo, "写文档")
	again := ok(t, sess, "pull_updates", map[string]any{"board_id": b.ID}).Data
	if got := types(t, again); len(got) != 1 || got[0] != "card.created" {
		t.Fatalf("应该读到别人创建卡片的事件: %v", again)
	}
	same := ok(t, sess, "pull_updates", map[string]any{"board_id": b.ID}).Data
	if len(items(t, same, "events")) != 1 {
		t.Errorf("没确认之前重新读应该拿到同样的事件: %v", same)
	}
	acked := ok(t, sess, "pull_updates", map[string]any{"board_id": b.ID, "since": again["cursor"]}).Data
	if len(items(t, acked, "events")) != 0 || acked["has_more"] != false {
		t.Errorf("确认之后不应该再有事件: %v", acked)
	}

	if _, err := s.core.CreateCard(t.Context(), agentActor, b.ID, kanban.CreateCardInput{Title: "我自己建的", ColumnID: b.todo}); err != nil {
		t.Fatalf("agent 创建卡片失败: %v", err)
	}
	if own := ok(t, sess, "pull_updates", map[string]any{"board_id": b.ID}).Data; len(items(t, own, "events")) != 0 {
		t.Errorf("自己的操作应该被过滤掉: %v", own)
	}
}

// TestPullUpdatesErrors 验证出错的情况：看板不存在、since 是负数都是带说明的工具错误；
// 管理视图根本没有这个工具（人不拉取更新）。
func TestPullUpdatesErrors(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	sess := s.agent(t, "claude")
	if got := call(t, sess, "pull_updates", map[string]any{"board_id": 9999}); !got.IsError || !strings.HasPrefix(got.Text, "ErrBoardNotFound:") {
		t.Errorf("看板不存在应该是工具错误: %+v", got)
	}
	if got := call(t, sess, "pull_updates", map[string]any{"board_id": b.ID, "since": -1}); !got.IsError {
		t.Errorf("since 是负数应该是工具错误: %+v", got)
	}
	if _, err := s.human(t).CallTool(t.Context(), &mcp.CallToolParams{Name: "pull_updates", Arguments: map[string]any{"board_id": b.ID}}); err == nil {
		t.Error("管理视图不应该有 pull_updates")
	}
}

// TestWaitForUpdatesWakes 验证阻塞等待：没有事件时一直等，别人一改动立刻醒来并带回事件，不用等到超时。
func TestWaitForUpdatesWakes(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	sess := s.agent(t, "claude")
	ok(t, sess, "pull_updates", map[string]any{"board_id": b.ID}) // 先读一次，登记进度

	done := make(chan result, 1)
	start := time.Now()
	go func() {
		res, err := sess.CallTool(t.Context(), &mcp.CallToolParams{Name: "wait_for_updates", Arguments: map[string]any{"board_id": b.ID, "timeout_seconds": 20}})
		if err != nil {
			t.Errorf("等待失败: %v", err)
			done <- result{}
			return
		}
		done <- unpack(t, res)
	}()
	time.Sleep(200 * time.Millisecond)
	s.card(t, b, b.todo, "写文档")
	select {
	case got := <-done:
		if got.IsError || got.Data["timed_out"] != false || len(items(t, got.Data, "events")) != 1 {
			t.Errorf("应该被唤醒并带回事件: %+v", got)
		}
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Errorf("应该在改动之后立刻醒来，实际等了 %v", elapsed)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("别人改动之后，阻塞等待没有醒来")
	}
}

// TestWaitForUpdatesTimesOut 验证超时不是错误：返回 timed_out 为真和空事件，带着可以继续使用的 cursor。
func TestWaitForUpdatesTimesOut(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	sess := s.agent(t, "claude")
	ok(t, sess, "pull_updates", map[string]any{"board_id": b.ID})

	start := time.Now()
	got := ok(t, sess, "wait_for_updates", map[string]any{"board_id": b.ID, "timeout_seconds": 1}).Data
	if got["timed_out"] != true || len(items(t, got, "events")) != 0 {
		t.Errorf("没有事件应该超时返回: %v", got)
	}
	if elapsed := time.Since(start); elapsed < 800*time.Millisecond || elapsed > 4*time.Second {
		t.Errorf("应该等大约 1 秒，实际 %v", elapsed)
	}
}

// TestWaitForUpdatesReturnsImmediatelyWhenFirst 验证第一次读某个看板时不等待：
// 要立刻告诉 agent 这是第一次，让它先去了解现状。
func TestWaitForUpdatesReturnsImmediatelyWhenFirst(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	start := time.Now()
	got := ok(t, s.agent(t, "claude"), "wait_for_updates", map[string]any{"board_id": b.ID, "timeout_seconds": 30}).Data
	if got["first"] != true || time.Since(start) > 3*time.Second {
		t.Errorf("第一次读应该立刻返回 first: %v (%v)", got, time.Since(start))
	}
}

// TestWaitForUpdatesBadTimeout 验证等待秒数是负数是带说明的工具错误，而不是被当成 0 悄悄处理。
func TestWaitForUpdatesBadTimeout(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	got := call(t, s.agent(t, "claude"), "wait_for_updates", map[string]any{"board_id": b.ID, "timeout_seconds": -5})
	if !got.IsError {
		t.Errorf("负数的等待秒数应该是工具错误: %+v", got)
	}
}

// TestWaitForUpdatesHugeTimeoutIsCapped 验证超大的等待秒数按上限处理，而不是换算成纳秒时溢出变成负数被拒绝。
// 这里先让 agent 有事件可读，这样调用立刻返回，不用真的等。
func TestWaitForUpdatesHugeTimeoutIsCapped(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	sess := s.agent(t, "claude")
	ok(t, sess, "pull_updates", map[string]any{"board_id": b.ID})
	s.card(t, b, b.todo, "写文档")

	got := ok(t, sess, "wait_for_updates", map[string]any{"board_id": b.ID, "timeout_seconds": 10_000_000_000}).Data
	if len(items(t, got, "events")) != 1 {
		t.Errorf("超大的等待秒数应该按上限处理，并带回已有的事件: %v", got)
	}
}

// TestWaitCancelledByClient 验证客户端取消一次阻塞等待之后，服务端真正结束了这次等待，
// 而不是白白占着协程和订阅一直等到超时。
func TestWaitCancelledByClient(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	sess := s.agent(t, "claude")
	ok(t, sess, "pull_updates", map[string]any{"board_id": b.ID})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: "wait_for_updates", Arguments: map[string]any{"board_id": b.ID, "timeout_seconds": 30}})
		done <- err
	}()
	eventually(t, "阻塞等待进入 Core", serverWaiting)

	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Error("取消之后调用应该返回错误")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("客户端取消之后，调用没有返回")
	}
	eventually(t, "服务端结束被取消的等待", func() bool { return !serverWaiting() })
}

// serverWaiting 报告当前是否有协程阻塞在 Core 的"等待更新"上。要看协程栈而不是数请求：
// 客户端断开后 HTTP 请求会先返回，处理函数是否还在等，只有协程栈看得出来。
// 本包的测试都不并行，所以栈里出现它就一定是当前测试的。
func serverWaiting() bool {
	buf := make([]byte, 1<<20)
	return strings.Contains(string(buf[:runtime.Stack(buf, true)]), "kanban.updateWaiter.wait")
}

// eventually 在最多 5 秒内反复检查条件，成立就返回，一直不成立让测试终止。
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("等了 5 秒，%s 仍然没有发生", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestWaitInterruptedByShutdown 验证服务开始关闭时，挂着的阻塞等待立刻结束并告诉 agent 稍后重试，
// 不会让关闭一直等到超时。
func TestWaitInterruptedByShutdown(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	sess := s.agent(t, "claude")
	ok(t, sess, "pull_updates", map[string]any{"board_id": b.ID})

	done := make(chan result, 1)
	go func() {
		res, err := sess.CallTool(t.Context(), &mcp.CallToolParams{Name: "wait_for_updates", Arguments: map[string]any{"board_id": b.ID, "timeout_seconds": 30}})
		if err != nil {
			t.Errorf("等待失败: %v", err)
			done <- result{}
			return
		}
		done <- unpack(t, res)
	}()
	time.Sleep(200 * time.Millisecond)
	start := time.Now()
	s.closeNow()
	select {
	case got := <-done:
		if !got.IsError || !strings.HasPrefix(got.Text, "ErrServiceUnavailable:") {
			t.Errorf("服务关闭应该返回 ErrServiceUnavailable: %+v", got)
		}
		if elapsed := time.Since(start); elapsed > 3*time.Second {
			t.Errorf("服务关闭后应该立刻结束，实际等了 %v", elapsed)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("服务关闭之后，阻塞等待没有结束")
	}
}
