package kanbanapi_test

import (
	"net/http"
	"strings"
	"testing"
)

// cardList 是卡片列表的响应。
type cardList struct {
	Cards     []card `json:"cards"`
	Total     int    `json:"total"`
	Truncated bool   `json:"truncated"`
}

// TestCardLifecycle 验证卡片的增改读删：创建返回编号；修改只更新出现的字段，旧版本号返回 409；
// 详情里评论是 [] 不是 null；删除幂等，删除后读取返回 404。
func TestCardLifecycle(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "CRD")
	todo := b.Columns[1].ID
	made := decode[card](t, s.human(t, http.MethodPost, "/boards/"+id(b.ID)+"/cards",
		map[string]any{"title": "写文档", "description": "要写清楚", "column_id": todo, "priority": 2, "tags": []string{"docs"}}))
	if made.ID == 0 || made.Key != "CRD-1" || made.ColumnID != todo || made.Priority != 2 || made.Version == 0 {
		t.Fatalf("创建的卡片不对: %+v", made)
	}

	path := "/cards/" + id(made.ID)
	detail := decode[map[string]any](t, s.human(t, http.MethodGet, path, nil))
	if comments, ok := detail["comments"].([]any); !ok || len(comments) != 0 {
		t.Fatalf("详情里的评论应该是 []: %v", detail["comments"])
	}

	patched := decode[card](t, s.human(t, http.MethodPatch, path, map[string]any{"title": "写好文档", "version": made.Version}))
	if patched.Title != "写好文档" || patched.Description != "要写清楚" || patched.Priority != 2 {
		t.Fatalf("只应该改标题: %+v", patched)
	}
	wantFail(t, "旧版本号", s.human(t, http.MethodPatch, path, map[string]any{"title": "x", "version": made.Version}),
		http.StatusConflict, "ErrCardVersionConflict")

	for range 2 {
		wantEmptyObject(t, "删除", s.human(t, http.MethodDelete, path, nil))
	}
	wantFail(t, "删除后读取", s.human(t, http.MethodGet, path, nil), http.StatusNotFound, "ErrCardNotFound")
}

// TestCardList 验证列表：空看板返回 "cards":[]；筛选词生效，limit 截断时给出总数和 truncated；
// 负数 limit 返回 400。
func TestCardList(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "LST")
	base := "/boards/" + id(b.ID) + "/cards"
	if resp := s.human(t, http.MethodGet, base, nil); !strings.Contains(string(resp.Body), `"cards":[]`) {
		t.Fatalf("空看板应该返回 \"cards\":[]: %s", resp.Body)
	}
	for _, title := range []string{"alpha one", "alpha two", "beta three"} {
		s.newCard(t, b, b.Columns[1].ID, title)
	}

	if got := decode[cardList](t, s.human(t, http.MethodGet, base+"?query=alpha", nil)); got.Total != 2 || len(got.Cards) != 2 || got.Truncated {
		t.Fatalf("筛选 alpha 应该命中 2 张: %+v", got)
	}
	if got := decode[cardList](t, s.human(t, http.MethodGet, base+"?limit=1", nil)); got.Total != 3 || len(got.Cards) != 1 || !got.Truncated {
		t.Fatalf("limit=1 应该只返回 1 张并标记截断: %+v", got)
	}
	wantFail(t, "负数 limit", s.human(t, http.MethodGet, base+"?limit=-1", nil), http.StatusBadRequest, "ErrInvalidInput")
	wantFail(t, "看板不存在", s.human(t, http.MethodGet, "/boards/9999/cards", nil), http.StatusNotFound, "ErrBoardNotFound")
}

// TestCardMoveAndComments 验证移动和评论：移到另一列后列变了；评论作者取自令牌身份；
// 详情里能看到评论；只有人能删评论；评论为空返回 400。
func TestCardMoveAndComments(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "MVC")
	c := s.newCard(t, b, b.Columns[1].ID, "移动我")
	path := "/cards/" + id(c.ID)

	moved := decode[card](t, s.human(t, http.MethodPost, path+"/move", map[string]any{"column_id": b.Columns[3].ID}))
	if moved.ColumnID != b.Columns[3].ID {
		t.Fatalf("卡片应该移到第四列: %+v", moved)
	}

	human := decode[comment](t, s.human(t, http.MethodPost, path+"/comments", map[string]string{"body": "人的评论"}))
	robot := decode[comment](t, s.agent(t, "claude", http.MethodPost, path+"/comments", map[string]string{"body": "agent 的评论"}))
	if human.Author != "human" || robot.Author != "human" {
		t.Fatalf("REST 评论作者固定为人: %+v %+v", human, robot)
	}
	wantFail(t, "空评论", s.human(t, http.MethodPost, path+"/comments", map[string]string{"body": "  "}), http.StatusBadRequest, "ErrInvalidInput")
	if got := decode[cardDetail](t, s.human(t, http.MethodGet, path, nil)); len(got.Comments) != 2 {
		t.Fatalf("详情里应该有两条评论: %+v", got.Comments)
	}

	for range 2 {
		wantEmptyObject(t, "人删评论", s.human(t, http.MethodDelete, "/comments/"+id(human.ID), nil))
	}
	if got := decode[cardDetail](t, s.human(t, http.MethodGet, path, nil)); len(got.Comments) != 1 {
		t.Fatalf("应该还剩一条评论: %+v", got.Comments)
	}
}

// TestCardPathNotOverridable 验证路径里的卡片编号不会被请求体覆盖：
// 修改卡片 a 的请求里夹带卡片 b 的编号，结果只有 a 变了。
func TestCardPathNotOverridable(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "OVR")
	a, other := s.newCard(t, b, b.Columns[1].ID, "甲"), s.newCard(t, b, b.Columns[1].ID, "乙")
	body := map[string]any{"title": "改了甲", "card_id": other.ID, "CardID": other.ID, "id": other.ID}
	if got := decode[card](t, s.human(t, http.MethodPatch, "/cards/"+id(a.ID), body)); got.ID != a.ID || got.Title != "改了甲" {
		t.Fatalf("应该修改路径里的卡片 a: %+v", got)
	}
	if got := decode[cardDetail](t, s.human(t, http.MethodGet, "/cards/"+id(other.ID), nil)); got.Title != "乙" {
		t.Fatalf("卡片 b 不应该被改: %+v", got)
	}
}
