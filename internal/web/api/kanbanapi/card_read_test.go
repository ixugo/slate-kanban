package kanbanapi_test

import (
	"net/http"
	"testing"
)

// TestMarkCardRead 验证已读接口幂等，清零后快照里 unread_count 为 0。
func TestMarkCardRead(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "RD")
	c := s.newCard(t, b, b.Columns[1].ID, "读我")
	path := "/cards/" + id(c.ID)
	_ = s.human(t, http.MethodPost, path+"/comments", map[string]string{"body": "有新动态"})

	snap := decode[map[string]any](t, s.human(t, http.MethodGet, "/boards/"+id(b.ID)+"/snapshot", nil))
	cards := snap["cards"].([]any)
	found := cards[0].(map[string]any)
	if n, _ := found["unread_count"].(float64); n != 1 {
		t.Fatalf("快照未读应为 1: %v", found["unread_count"])
	}

	for range 2 {
		wantEmptyObject(t, "已读", s.human(t, http.MethodPost, path+"/read", nil))
	}

	snap2 := decode[map[string]any](t, s.human(t, http.MethodGet, "/boards/"+id(b.ID)+"/snapshot", nil))
	cards2 := snap2["cards"].([]any)
	got := cards2[0].(map[string]any)
	if n, _ := got["unread_count"].(float64); n != 0 {
		t.Fatalf("已读后未读应为 0: %v", got["unread_count"])
	}
	wantFail(t, "不存在", s.human(t, http.MethodPost, "/cards/9999/read", nil), http.StatusNotFound, "ErrCardNotFound")
}
