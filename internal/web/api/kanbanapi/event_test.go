package kanbanapi_test

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// updates 是拉取和等待的响应。
type updates struct {
	Events []struct {
		ID      int64          `json:"id"`
		Type    string         `json:"type"`
		Actor   string         `json:"actor"`
		Payload map[string]any `json:"payload"`
	} `json:"events"`
	Cursor   int64 `json:"cursor"`
	Latest   int64 `json:"latest"`
	First    bool  `json:"first"`
	TimedOut bool  `json:"timed_out"`
}

// TestSnapshot 验证快照：一次包含看板、列、规则、全部卡片，并带着不比数据新的事件序号。
func TestSnapshot(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "SNP")
	s.newCard(t, b, b.Columns[1].ID, "快照里的卡片")
	snap := decode[struct {
		board
		Cards []card `json:"cards"`
		Seq   int64  `json:"seq"`
	}](t, s.human(t, http.MethodGet, "/boards/"+id(b.ID)+"/snapshot", nil))
	if snap.ID != b.ID || len(snap.Columns) != 5 || len(snap.Rules) == 0 || len(snap.Cards) != 1 || snap.Seq == 0 {
		t.Fatalf("快照内容不对: %+v", snap)
	}
	wantFail(t, "看板不存在", s.human(t, http.MethodGet, "/boards/9999/snapshot", nil), http.StatusNotFound, "ErrBoardNotFound")
}

// TestListEvents 验证事件列表：事件内容是 JSON 对象；after 和 limit 生效，has_more 正确；
// 没有新事件时是 []；after 为负数返回 400。
func TestListEvents(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "EVT")
	s.newCard(t, b, b.Columns[1].ID, "产生事件一")
	s.newCard(t, b, b.Columns[1].ID, "产生事件二")
	base := "/boards/" + id(b.ID) + "/events"

	page := decode[struct {
		updates
		HasMore bool `json:"has_more"`
	}](t, s.human(t, http.MethodGet, base+"?after=0&limit=2", nil))
	if len(page.Events) != 2 || !page.HasMore || page.Events[0].Payload == nil || page.Latest <= page.Events[1].ID {
		t.Fatalf("limit=2 应该返回两条并标记还有更多，内容是对象: %+v", page)
	}
	if resp := s.human(t, http.MethodGet, base+"?after="+id(page.Latest), nil); !strings.Contains(string(resp.Body), `"events":[]`) {
		t.Fatalf("没有新事件应该返回 \"events\":[]: %s", resp.Body)
	}
	wantFail(t, "负数 after", s.human(t, http.MethodGet, base+"?after=-1", nil), http.StatusBadRequest, "ErrInvalidInput")
}

// TestPullUpdates 验证 agent 拉取：人不能拉；第一次只定下进度，不补发历史；之后能拉到人做的修改，
// 事件内容是对象；带上 since 确认之后不会再拿到同样的事件。
func TestPullUpdates(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "PUL")
	path := "/boards/" + id(b.ID) + "/updates/pull"
	wantFail(t, "人拉取", s.human(t, http.MethodPost, path, nil), http.StatusForbidden, "ErrAgentOnly")

	first := decode[updates](t, s.agent(t, "claude", http.MethodPost, path, nil))
	if !first.First || len(first.Events) != 0 || first.Cursor != first.Latest {
		t.Fatalf("第一次拉取应该只定下进度: %+v", first)
	}
	s.newCard(t, b, b.Columns[1].ID, "离线时建的卡片")
	got := decode[updates](t, s.agent(t, "claude", http.MethodPost, path, map[string]int64{"since": first.Cursor}))
	if len(got.Events) != 1 || got.Events[0].Type != "card.created" || got.Events[0].Actor != "admin" || got.Events[0].Payload == nil {
		t.Fatalf("应该拉到人建卡片的事件: %+v", got)
	}
	again := decode[updates](t, s.agent(t, "claude", http.MethodPost, path, map[string]int64{"since": got.Cursor}))
	if len(again.Events) != 0 || again.Cursor != got.Cursor {
		t.Fatalf("确认之后不应该再拿到同样的事件: %+v", again)
	}
}

// TestWaitUpdates 验证阻塞等待：有新事件时马上返回；没有时到点返回 timed_out，不是错误；
// 人不能等；负数的等待时间返回 400。
func TestWaitUpdates(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "WAI")
	pull := "/boards/" + id(b.ID) + "/updates/pull"
	wait := "/boards/" + id(b.ID) + "/updates/wait"
	wantFail(t, "人等待", s.human(t, http.MethodPost, wait, nil), http.StatusForbidden, "ErrAgentOnly")

	first := decode[updates](t, s.agent(t, "claude", http.MethodPost, pull, nil))
	wantFail(t, "负数等待", s.agent(t, "claude", http.MethodPost, wait, map[string]any{"since": first.Cursor, "timeout_seconds": -1}),
		http.StatusBadRequest, "ErrInvalidInput")

	start := time.Now()
	idle := decode[updates](t, s.agent(t, "claude", http.MethodPost, wait, map[string]any{"since": first.Cursor, "timeout_seconds": 1}))
	if elapsed := time.Since(start); !idle.TimedOut || len(idle.Events) != 0 || elapsed < 900*time.Millisecond || elapsed > 5*time.Second {
		t.Fatalf("没有新事件应该等满约 1 秒后超时返回: %+v 用时 %v", idle, elapsed)
	}

	s.newCard(t, b, b.Columns[1].ID, "新卡片")
	start = time.Now()
	woke := decode[updates](t, s.agent(t, "claude", http.MethodPost, wait, map[string]any{"since": idle.Cursor, "timeout_seconds": 30}))
	if woke.TimedOut || len(woke.Events) != 1 || time.Since(start) > 5*time.Second {
		t.Fatalf("已经有新事件应该马上返回: %+v 用时 %v", woke, time.Since(start))
	}
}
