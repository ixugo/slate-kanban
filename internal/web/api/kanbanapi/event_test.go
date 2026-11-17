package kanbanapi_test

import (
	"net/http"
	"strings"
	"testing"
)

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
	if len(page.Events) != 2 || !page.HasMore || page.Events[0].Payload == nil {
		t.Fatalf("limit=2 应该返回两条: %+v", page)
	}
	if resp := s.human(t, http.MethodGet, base+"?after="+id(page.Latest), nil); !strings.Contains(string(resp.Body), `"events":[]`) {
		t.Fatalf("没有新事件应该返回 \"events\":[]: %s", resp.Body)
	}
	wantFail(t, "负数 after", s.human(t, http.MethodGet, base+"?after=-1", nil), http.StatusBadRequest, "ErrInvalidInput")
}

// TestPullWaitHumanOnly 验证拉取和阻塞等待只给 agent（MCP）；REST 人身份被拒绝。
func TestPullWaitHumanOnly(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "PUL")
	pull := "/boards/" + id(b.ID) + "/updates/pull"
	wait := "/boards/" + id(b.ID) + "/updates/wait"
	wantFail(t, "人拉取", s.human(t, http.MethodPost, pull, nil), http.StatusForbidden, "ErrAgentOnly")
	wantFail(t, "人等待", s.human(t, http.MethodPost, wait, nil), http.StatusForbidden, "ErrAgentOnly")
}
