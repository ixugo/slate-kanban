package mcpapi_test

import (
	"context"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"slate-kanban/internal/web/mcpapi"
)

const (
	// fastDelay 是大多数通知测试用的合并窗口，调短了测试才不用干等。
	fastDelay = 30 * time.Millisecond
	// quiet 是确认"没有通知"要等多久：比合并窗口长得多，窗口过了还没来就是不会来。
	quiet = 600 * time.Millisecond
	// patience 是等一条通知最多等多久，比合并窗口宽松很多，慢机器上也不会误报。
	patience = 5 * time.Second
)

// settle 等通知器把已经登记的更新都发完。创建看板、建卡片这些准备工作本身也会触发通知，
// 订阅得太早会收到它们，让"这次改动之后应该收到什么"的断言变得不确定。delay 是这套服务的合并窗口。
func settle(delay time.Duration) {
	time.Sleep(10 * delay)
}

// notice 是客户端收到的一条"看板有更新"通知。
type notice struct {
	URI    string
	Latest int64
}

// watch 以 token 的身份连接服务，并把收到的资源更新通知放进返回的通道。
func (s *stack) watch(t *testing.T, token string) (*mcp.ClientSession, <-chan notice) {
	t.Helper()
	ch := make(chan notice, 64)
	admin := token == s.adminToken()
	sess := s.connect(t, token, admin, &mcp.ClientOptions{
		ResourceUpdatedHandler: func(_ context.Context, r *mcp.ResourceUpdatedNotificationRequest) {
			latest, _ := r.Params.Meta["latest"].(float64)
			ch <- notice{URI: r.Params.URI, Latest: int64(latest)}
		},
	})
	return sess, ch
}

// expectNotice 等一条通知，等不到让测试终止。
func expectNotice(t *testing.T, ch <-chan notice) notice {
	t.Helper()
	select {
	case n := <-ch:
		return n
	case <-time.After(patience):
		t.Fatal("没有收到看板更新通知")
		return notice{}
	}
}

// expectNone 确认在 quiet 这段时间里没有通知到达。
func expectNone(t *testing.T, ch <-chan notice, why string) {
	t.Helper()
	select {
	case n := <-ch:
		t.Errorf("%s：不应该收到通知，实际收到 %+v", why, n)
	case <-time.After(quiet):
	}
}

// TestSubscribeAndNotify 验证从订阅到补读的完整过程：订阅看板后，别人改了看板就收到通知，
// 通知带着最新的事件序号；收到通知后用 pull_updates 读到的正是这次改动。
func TestSubscribeAndNotify(t *testing.T) {
	s := newStack(t, mcpapi.WithNotifyDelay(fastDelay))
	b := s.seed(t, "SLATE")
	sess, got := s.watch(t, s.token(t, "claude"))
	ok(t, sess, "pull_updates", map[string]any{"board_id": b.ID})
	settle(fastDelay)
	if err := sess.Subscribe(t.Context(), &mcp.SubscribeParams{URI: boardURIOf(b.ID)}); err != nil {
		t.Fatalf("订阅失败: %v", err)
	}

	s.card(t, b, b.todo, "写文档")
	latest, err := s.core.LatestSeq(t.Context(), b.ID)
	if err != nil {
		t.Fatalf("读取最新序号失败: %v", err)
	}
	if n := expectNotice(t, got); n.URI != boardURIOf(b.ID) || n.Latest != latest {
		t.Errorf("通知的地址或序号不对: %+v，最新序号 %d", n, latest)
	}
	pulled := ok(t, sess, "pull_updates", map[string]any{"board_id": b.ID}).Data
	if events := types(t, pulled); len(events) != 1 || events[0] != "card.created" {
		t.Errorf("收到通知后应该能补读到这次改动: %v", pulled)
	}
}

// TestNotificationsCoalesce 验证通知会合并：窗口内的一连串改动只发一条，序号是最后一次改动的，
// 窗口过后的下一次改动再发新的一条。
func TestNotificationsCoalesce(t *testing.T) {
	const delay = 400 * time.Millisecond
	s := newStack(t, mcpapi.WithNotifyDelay(delay))
	b := s.seed(t, "SLATE")
	sess, got := s.watch(t, s.token(t, "claude"))
	time.Sleep(2 * delay)
	if err := sess.Subscribe(t.Context(), &mcp.SubscribeParams{URI: boardURIOf(b.ID)}); err != nil {
		t.Fatalf("订阅失败: %v", err)
	}

	for i := range 10 {
		s.card(t, b, b.todo, "卡片"+string(rune('A'+i)))
	}
	latest, err := s.core.LatestSeq(t.Context(), b.ID)
	if err != nil {
		t.Fatalf("读取最新序号失败: %v", err)
	}
	if n := expectNotice(t, got); n.Latest != latest {
		t.Errorf("合并后的通知应该带最后一次改动的序号 %d，实际 %d", latest, n.Latest)
	}
	expectNone(t, got, "一连串改动只应该发一条通知")

	s.card(t, b, b.todo, "窗口之后")
	if n := expectNotice(t, got); n.Latest <= latest {
		t.Errorf("窗口之后的改动应该再发一条新的通知: %+v", n)
	}
}

// TestNotifyOnlyThatBoard 验证通知只发给订阅了这个看板的会话：没订阅的、订阅了别的看板的、取消订阅了的，
// 都不会收到；管理视图（另一个 Server）的订阅同样有效。
func TestNotifyOnlyThatBoard(t *testing.T) {
	s := newStack(t, mcpapi.WithNotifyDelay(fastDelay))
	b, other := s.seed(t, "SLATE"), s.seed(t, "OTHER")
	subscribed, subscribedCh := s.watch(t, s.adminToken())
	elsewhere, elsewhereCh := s.watch(t, s.token(t, "codex"))
	_, silentCh := s.watch(t, s.token(t, "claude"))
	left, leftCh := s.watch(t, s.token(t, "gemini"))
	settle(fastDelay)
	for _, c := range []struct {
		sess *mcp.ClientSession
		id   int64
	}{{subscribed, b.ID}, {elsewhere, other.ID}, {left, b.ID}} {
		if err := c.sess.Subscribe(t.Context(), &mcp.SubscribeParams{URI: boardURIOf(c.id)}); err != nil {
			t.Fatalf("订阅失败: %v", err)
		}
	}
	if err := left.Unsubscribe(t.Context(), &mcp.UnsubscribeParams{URI: boardURIOf(b.ID)}); err != nil {
		t.Fatalf("取消订阅失败: %v", err)
	}

	s.card(t, b, b.todo, "写文档")
	if n := expectNotice(t, subscribedCh); n.URI != boardURIOf(b.ID) {
		t.Errorf("管理视图的订阅应该收到通知: %+v", n)
	}
	expectNone(t, elsewhereCh, "订阅了别的看板")
	expectNone(t, silentCh, "没有订阅")
	expectNone(t, leftCh, "已经取消订阅")
}

// TestNotifyBoardDeleted 验证看板被删除时订阅者也会收到通知（序号是 0），之后再读这个资源得到"不存在"。
func TestNotifyBoardDeleted(t *testing.T) {
	s := newStack(t, mcpapi.WithNotifyDelay(fastDelay))
	b := s.seed(t, "SLATE")
	sess, got := s.watch(t, s.token(t, "claude"))
	settle(fastDelay)
	if err := sess.Subscribe(t.Context(), &mcp.SubscribeParams{URI: boardURIOf(b.ID)}); err != nil {
		t.Fatalf("订阅失败: %v", err)
	}
	if err := s.core.DeleteBoard(t.Context(), humanActor, b.ID); err != nil {
		t.Fatalf("删除看板失败: %v", err)
	}
	if n := expectNotice(t, got); n.URI != boardURIOf(b.ID) || n.Latest != 0 {
		t.Errorf("删除看板应该通知订阅者，序号为 0: %+v", n)
	}
	if _, err := sess.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: boardURIOf(b.ID)}); err == nil {
		t.Error("看板已经删除，读取资源应该失败")
	}
}

// TestNotifierStopsWithCleanup 验证调用清理函数后通知器停止工作：之后的改动不再发通知，
// 写入本身也不受影响；清理函数可以重复调用。
func TestNotifierStopsWithCleanup(t *testing.T) {
	s := newStack(t, mcpapi.WithNotifyDelay(fastDelay))
	b := s.seed(t, "SLATE")
	sess, got := s.watch(t, s.token(t, "claude"))
	settle(fastDelay)
	if err := sess.Subscribe(t.Context(), &mcp.SubscribeParams{URI: boardURIOf(b.ID)}); err != nil {
		t.Fatalf("订阅失败: %v", err)
	}
	s.stopNotifier()
	s.stopNotifier()

	s.card(t, b, b.todo, "停止之后")
	expectNone(t, got, "通知器已经停止")
}
