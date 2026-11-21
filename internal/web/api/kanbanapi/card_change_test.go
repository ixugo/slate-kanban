package kanbanapi_test

import (
	"net/http"
	"testing"
)

// changeRecord 是变更记录接口的返回结构。
type changeRecord struct {
	ID        int64  `json:"id"`
	BoardID   int64  `json:"board_id"`
	CardID    int64  `json:"card_id"`
	Field     string `json:"field"`
	OldValue  string `json:"old_value"`
	NewValue  string `json:"new_value"`
	ActorKind string `json:"actor_kind"`
	Actor     string `json:"actor"`
	CreatedAt string `json:"created_at"`
}

// lastDescriptionChangeID 在看板的事件里找最新一条 card.updated 载荷里的 description_change_id，找不到返回 0。
func lastDescriptionChangeID(t *testing.T, s *stack, boardID int64) int64 {
	t.Helper()
	page := decode[updates](t, s.human(t, http.MethodGet, "/boards/"+id(boardID)+"/events?after=0&limit=200", nil))
	var found int64
	for _, ev := range page.Events {
		if ev.Type != "card.updated" {
			continue
		}
		if n, ok := ev.Payload["description_change_id"].(float64); ok {
			found = int64(n)
		}
	}
	return found
}

// TestCardChangeEndpoint 验证「改描述 → 事件带变更编号 → 按编号读回前后两版」的完整链路：
// 返回的记录内容和修改者都对；用别的卡片、不存在的编号读取都是 404；路径参数不是数字是 400；
// 卡片删除后记录也读不到了。
func TestCardChangeEndpoint(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "CHG")
	todo := b.Columns[1].ID
	made := decode[card](t, s.human(t, http.MethodPost, "/boards/"+id(b.ID)+"/cards",
		map[string]any{"title": "写文档", "description": "第一版\n第二行", "column_id": todo}))
	other := s.newCard(t, b, todo, "别的卡片")
	path := "/cards/" + id(made.ID)

	decode[card](t, s.human(t, http.MethodPatch, path, map[string]any{"description": "第一版\n改过的第二行\n新增一行"}))
	changeID := lastDescriptionChangeID(t, s, b.ID)
	if changeID == 0 {
		t.Fatal("改描述后 card.updated 事件应该带 description_change_id")
	}

	got := decode[changeRecord](t, s.human(t, http.MethodGet, path+"/changes/"+id(changeID), nil))
	if got.ID != changeID || got.BoardID != b.ID || got.CardID != made.ID || got.Field != "description" ||
		got.OldValue != "第一版\n第二行" || got.NewValue != "第一版\n改过的第二行\n新增一行" ||
		got.ActorKind != "human" || got.Actor == "" || got.CreatedAt == "" {
		t.Fatalf("变更记录不对: %+v", got)
	}

	wantFail(t, "用别的卡片读", s.human(t, http.MethodGet, "/cards/"+id(other.ID)+"/changes/"+id(changeID), nil),
		http.StatusNotFound, "ErrCardChangeNotFound")
	wantFail(t, "不存在的编号", s.human(t, http.MethodGet, path+"/changes/9999", nil), http.StatusNotFound, "ErrCardChangeNotFound")
	if resp := s.human(t, http.MethodGet, path+"/changes/abc", nil); resp.Code != http.StatusBadRequest {
		t.Errorf("编号不是数字应该返回 400，实际 %d %s", resp.Code, resp.Body)
	}

	wantEmptyObject(t, "删除卡片", s.human(t, http.MethodDelete, path, nil))
	wantFail(t, "卡片删除后读取", s.human(t, http.MethodGet, path+"/changes/"+id(changeID), nil),
		http.StatusNotFound, "ErrCardChangeNotFound")
}
