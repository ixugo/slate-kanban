package kanbanapi_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"slate-kanban/internal/web/api/kanbanapi"
)

// within 是等一条流消息的最长时间，正常情况下消息几乎是立刻到的。
const within = 3 * time.Second

// sseMsg 是流里的一条消息。
type sseMsg struct {
	id, event, data, comment string
}

// sseConn 是测试用的流客户端：后台协程把读到的消息按顺序放进通道，流结束时关闭通道。
type sseConn struct {
	resp   *http.Response
	msgs   chan sseMsg
	cancel context.CancelFunc
}

// openStream 连接实时流。header 里的键值会作为请求头发出。连接在测试结束时断开。
func openStream(t *testing.T, srv *httptest.Server, token, path string, header map[string]string) *sseConn {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1"+path, http.NoBody)
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("连接实时流失败: %v", err)
	}
	conn := &sseConn{resp: resp, msgs: make(chan sseMsg, 64), cancel: cancel}
	t.Cleanup(func() {
		cancel()
		if err := resp.Body.Close(); err != nil {
			t.Logf("关闭响应体: %v", err)
		}
	})
	go conn.read()
	return conn
}

// read 按 text/event-stream 的格式解析响应体，遇到空行就产出一条消息，读完或出错就关闭通道。
func (c *sseConn) read() {
	defer close(c.msgs)
	var cur sseMsg
	scanner := bufio.NewScanner(c.resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			if cur != (sseMsg{}) {
				c.msgs <- cur
			}
			cur = sseMsg{}
		case strings.HasPrefix(line, "id: "):
			cur.id = strings.TrimPrefix(line, "id: ")
		case strings.HasPrefix(line, "event: "):
			cur.event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			cur.data = strings.TrimPrefix(line, "data: ")
		case strings.HasPrefix(line, ": "):
			cur.comment = strings.TrimPrefix(line, ": ")
		}
	}
}

// next 取下一条消息，等不到或者流已经结束就让测试终止。
func (c *sseConn) next(t *testing.T) sseMsg {
	t.Helper()
	select {
	case msg, ok := <-c.msgs:
		if !ok {
			t.Fatal("流已经结束，但还需要一条消息")
		}
		return msg
	case <-time.After(within):
		t.Fatal("等了很久也没有收到消息")
		return sseMsg{}
	}
}

// wantEnd 要求流在 within 内结束，期间不应该再有消息。
func (c *sseConn) wantEnd(t *testing.T) {
	t.Helper()
	select {
	case msg, ok := <-c.msgs:
		if ok {
			t.Fatalf("流应该结束，却收到了消息: %+v", msg)
		}
	case <-time.After(within):
		t.Fatal("流没有按时结束")
	}
}

// wantQuiet 要求接下来这段时间里没有新消息。
func (c *sseConn) wantQuiet(t *testing.T, d time.Duration) {
	t.Helper()
	select {
	case msg, ok := <-c.msgs:
		t.Fatalf("不应该有新消息，收到 %+v（流是否结束: %v）", msg, !ok)
	case <-time.After(d):
	}
}

// ready 要求下一条是 ready，返回它带的最新序号。
func (c *sseConn) ready(t *testing.T) int64 {
	t.Helper()
	msg := c.next(t)
	var data struct {
		Latest int64 `json:"latest"`
	}
	if msg.event != "ready" || json.Unmarshal([]byte(msg.data), &data) != nil {
		t.Fatalf("第一条应该是 ready: %+v", msg)
	}
	return data.Latest
}

// boardEvent 要求下一条是 board 事件，返回事件序号和类型，并确认 id 行等于事件序号、内容是对象。
func (c *sseConn) boardEvent(t *testing.T) (int64, string) {
	t.Helper()
	msg := c.next(t)
	var ev struct {
		ID      int64          `json:"id"`
		Type    string         `json:"type"`
		Payload map[string]any `json:"payload"`
	}
	if msg.event != "board" || json.Unmarshal([]byte(msg.data), &ev) != nil || msg.id != strconv.FormatInt(ev.ID, 10) || ev.Payload == nil {
		t.Fatalf("应该是 board 事件，id 行等于事件序号，内容是对象: %+v", msg)
	}
	return ev.ID, ev.Type
}

// TestStreamLive 验证实时推送：连上先收到 ready，之后别人做的修改马上推过来。
func TestStreamLive(t *testing.T) {
	s := newStack(t)
	srv := s.serve(t)
	b := s.newBoard(t, "LIV")
	conn := openStream(t, srv, adminToken, "/boards/"+id(b.ID)+"/stream", nil)
	latest := conn.ready(t)

	c := s.newCard(t, b, b.Columns[1].ID, "新卡片")
	evID, typ := conn.boardEvent(t)
	if typ != "card.created" || evID <= latest {
		t.Fatalf("应该推来建卡片的事件，序号在 ready 之后: %d %s（latest=%d）", evID, typ, latest)
	}
	s.human(t, http.MethodPost, "/cards/"+id(c.ID)+"/comments", map[string]string{"body": "留言"})
	if _, typ := conn.boardEvent(t); typ != "comment.added" {
		t.Fatalf("应该推来评论事件: %s", typ)
	}
	conn.wantQuiet(t, 100*time.Millisecond)
}

// TestStreamHeadersAndAccess 验证响应头和认证：流式响应头齐全；agent 也能读；
// 没有令牌返回 401；把令牌放在 URL 里不会被认可。
func TestStreamHeadersAndAccess(t *testing.T) {
	s := newStack(t)
	srv := s.serve(t)
	b := s.newBoard(t, "HDR")
	path := "/boards/" + id(b.ID) + "/stream"

	conn := openStream(t, srv, s.token(t, "claude"), path, nil)
	h := conn.resp.Header
	if conn.resp.StatusCode != http.StatusOK || !strings.HasPrefix(h.Get("Content-Type"), "text/event-stream") ||
		h.Get("Cache-Control") != "no-cache" || h.Get("X-Accel-Buffering") != "no" {
		t.Fatalf("响应头不对: %d %v", conn.resp.StatusCode, h)
	}
	conn.ready(t)

	if resp := s.Do(t, http.MethodGet, "/api/v1"+path, "", nil); resp.Code != http.StatusUnauthorized {
		t.Errorf("没有令牌应该返回 401: %d", resp.Code)
	}
	if resp := s.Do(t, http.MethodGet, "/api/v1"+path+"?token="+adminToken+"&access_token="+adminToken, "", nil); resp.Code != http.StatusUnauthorized {
		t.Errorf("URL 里的令牌不应该被认可: %d", resp.Code)
	}
}

// TestStreamResume 验证续传：after 之后的事件按顺序补发；Last-Event-ID 请求头同样有效；
// 两个都给时以 after 为准。
func TestStreamResume(t *testing.T) {
	s := newStack(t)
	srv := s.serve(t)
	b := s.newBoard(t, "RES")
	s.newCard(t, b, b.Columns[1].ID, "甲")
	s.newCard(t, b, b.Columns[1].ID, "乙")
	path := "/boards/" + id(b.ID) + "/stream"
	all := decode[struct {
		Events []struct {
			ID int64 `json:"id"`
		} `json:"events"`
	}](t, s.human(t, http.MethodGet, "/boards/"+id(b.ID)+"/events", nil)).Events
	if len(all) != 3 {
		t.Fatalf("准备工作应该产生三条事件: %+v", all)
	}

	byQuery := openStream(t, srv, adminToken, path+"?after="+id(all[0].ID), nil)
	if latest := byQuery.ready(t); latest != all[2].ID {
		t.Fatalf("ready 应该带最新序号 %d: %d", all[2].ID, latest)
	}
	for _, want := range all[1:] {
		if got, _ := byQuery.boardEvent(t); got != want.ID {
			t.Fatalf("补发顺序不对: 期望 %d，实际 %d", want.ID, got)
		}
	}
	byQuery.wantQuiet(t, 100*time.Millisecond)

	byHeader := openStream(t, srv, adminToken, path, map[string]string{"Last-Event-ID": id(all[1].ID)})
	byHeader.ready(t)
	if got, _ := byHeader.boardEvent(t); got != all[2].ID {
		t.Fatalf("Last-Event-ID 之后应该只补发最后一条: %d", got)
	}

	both := openStream(t, srv, adminToken, path+"?after="+id(all[0].ID), map[string]string{"Last-Event-ID": id(all[2].ID)})
	both.ready(t)
	if got, _ := both.boardEvent(t); got != all[1].ID {
		t.Fatalf("两个都给时应该以 after 为准: %d", got)
	}
}

// TestStreamReset 验证需要重取快照的两种情况：客户端的序号比服务端还大（数据回退过），
// 以及起点早于还保留着的事件（中间的已被清理）。两种都先发 ready 再发 reset，然后结束。
func TestStreamReset(t *testing.T) {
	s := newStack(t)
	srv := s.serve(t)
	b := s.newBoard(t, "RST")
	s.newCard(t, b, b.Columns[1].ID, "甲")
	s.newCard(t, b, b.Columns[1].ID, "乙")
	path := "/boards/" + id(b.ID) + "/stream"

	ahead := openStream(t, srv, adminToken, path+"?after=999999", nil)
	latest := ahead.ready(t)
	msg := ahead.next(t)
	if msg.event != "reset" || !strings.Contains(msg.data, `"latest":`+id(latest)) {
		t.Fatalf("序号超前应该发 reset: %+v", msg)
	}
	ahead.wantEnd(t)

	if err := s.db.Event().Prune(t.Context(), b.ID, 1); err != nil {
		t.Fatalf("模拟清理旧事件失败: %v", err)
	}
	stale := openStream(t, srv, adminToken, path+"?after=0", nil)
	stale.ready(t)
	if msg := stale.next(t); msg.event != "reset" {
		t.Fatalf("起点早于保留范围应该发 reset: %+v", msg)
	}
	stale.wantEnd(t)
}

// TestStreamErrors 验证开始之前的错误都是正常的 JSON 错误响应：看板不存在 404，
// 看板编号、after、Last-Event-ID 不合法 400。
func TestStreamErrors(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "ERR")
	cases := []struct {
		name, path string
		header     map[string]string
		status     int
	}{
		{"看板不存在", "/boards/99999/stream", nil, http.StatusNotFound},
		{"编号不是数字", "/boards/abc/stream", nil, http.StatusBadRequest},
		{"after 不是数字", "/boards/" + id(b.ID) + "/stream?after=abc", nil, http.StatusBadRequest},
		{"after 是负数", "/boards/" + id(b.ID) + "/stream?after=-1", nil, http.StatusBadRequest},
		{"Last-Event-ID 不合法", "/boards/" + id(b.ID) + "/stream", map[string]string{"Last-Event-ID": "x"}, http.StatusBadRequest},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/api/v1"+tc.path, http.NoBody)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		for k, v := range tc.header {
			req.Header.Set(k, v)
		}
		resp := s.Serve(req)
		if resp.Code != tc.status || resp.Reason() == "" {
			t.Errorf("%s：期望 %d 加错误格式，实际 %d %s", tc.name, tc.status, resp.Code, resp.Body)
		}
	}
}

// TestStreamBoardDeleted 验证看板在流连着时被删除：流发出 error 事件，说明看板不存在，然后结束。
func TestStreamBoardDeleted(t *testing.T) {
	s := newStack(t)
	srv := s.serve(t)
	b := s.newBoard(t, "DEL")
	conn := openStream(t, srv, adminToken, "/boards/"+id(b.ID)+"/stream", nil)
	conn.ready(t)

	wantEmptyObject(t, "删除看板", s.human(t, http.MethodDelete, "/boards/"+id(b.ID), nil))
	msg := conn.next(t)
	if msg.event != "error" || !strings.Contains(msg.data, "ErrBoardNotFound") {
		t.Fatalf("应该收到看板不存在的 error 事件: %+v", msg)
	}
	conn.wantEnd(t)
}

// TestStreamHeartbeatAndMaxAge 验证保活和到期：空闲时按间隔收到 ping 注释行；
// 连接超过最长时间，服务端主动结束。这里把间隔缩短，免得真的等 20 秒和 10 分钟。
func TestStreamHeartbeatAndMaxAge(t *testing.T) {
	s := newStackWith(t, func(a kanbanapi.API) kanbanapi.API {
		return kanbanapi.WithStreamTimingForTest(a, 50*time.Millisecond, 600*time.Millisecond)
	})
	srv := s.serve(t)
	b := s.newBoard(t, "HBT")
	conn := openStream(t, srv, adminToken, "/boards/"+id(b.ID)+"/stream", nil)
	conn.ready(t)
	if msg := conn.next(t); msg.comment != "ping" {
		t.Fatalf("空闲时应该收到 ping: %+v", msg)
	}
	deadline := time.After(within) // 只设一个总期限：ping 一直在来，不能每收到一条就重新计时
	for {
		select {
		case msg, ok := <-conn.msgs:
			if !ok {
				return // 到期，服务端主动结束
			}
			if msg.comment != "ping" {
				t.Fatalf("到期之前只应该有 ping: %+v", msg)
			}
		case <-deadline:
			t.Fatal("连接超过最长时间后应该被服务端结束")
		}
	}
}

// TestStreamClientDisconnect 验证客户端断开后，服务端的处理函数会退出，不会一直留着。
func TestStreamClientDisconnect(t *testing.T) {
	s := newStack(t)
	srv := s.serve(t)
	b := s.newBoard(t, "DIS")
	for len(s.finished) > 0 {
		<-s.finished // 清掉建看板那次请求留下的记录
	}
	conn := openStream(t, srv, adminToken, "/boards/"+id(b.ID)+"/stream", nil)
	conn.ready(t)
	conn.cancel()
	select {
	case <-s.finished:
	case <-time.After(within):
		t.Fatal("客户端断开后，处理函数应该退出")
	}
}
