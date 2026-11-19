package kanbanapi_test

import (
	"net/http"
	"net/url"
	"slices"
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

// activityPage 是活动流接口的返回结构。
type activityPage struct {
	Events []struct {
		ID        int64  `json:"id"`
		CardID    int64  `json:"card_id"`
		Type      string `json:"type"`
		ActorKind string `json:"actor_kind"`
		Actor     string `json:"actor"`
	} `json:"events"`
	HasMore bool  `json:"has_more"`
	Latest  int64 `json:"latest"`
	Actors  []struct {
		Kind string `json:"kind"`
		Name string `json:"name"`
	} `json:"actors"`
}

// activityTypes 取出一页活动里每条事件的类型，顺序和接口返回一致。
func activityTypes(p activityPage) []string {
	out := make([]string, len(p.Events))
	for i, e := range p.Events {
		out[i] = e.Type
	}
	return out
}

// TestActivity 验证活动流接口：最新的在前，能按类型、操作者类型、操作者名称筛选，
// 用 before 翻页，并带回最新序号和操作者列表。
func TestActivity(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "ACT")
	c := s.newCard(t, b, b.Columns[1].ID, "活动一")
	s.human(t, http.MethodPost, "/cards/"+id(c.ID)+"/comments", map[string]string{"body": "留言"})
	s.human(t, http.MethodPost, "/cards/"+id(c.ID)+"/move", map[string]any{"column_id": b.Columns[2].ID})
	base := "/boards/" + id(b.ID) + "/activity"

	all := decode[activityPage](t, s.human(t, http.MethodGet, base, nil))
	if len(all.Events) < 4 || all.HasMore || all.Latest != all.Events[0].ID {
		t.Fatalf("不带条件应该一次返回全部、最新的在前: %+v", all)
	}
	for i := 1; i < len(all.Events); i++ {
		if all.Events[i].ID >= all.Events[i-1].ID {
			t.Fatalf("事件必须从新到旧: %+v", all.Events)
		}
	}
	if len(all.Actors) < 2 {
		t.Fatalf("操作者列表应该包含人和自动化: %+v", all.Actors)
	}

	paged := decode[activityPage](t, s.human(t, http.MethodGet, base+"?limit=2", nil))
	if len(paged.Events) != 2 || !paged.HasMore || paged.Events[0].ID != all.Events[0].ID {
		t.Fatalf("limit=2 应该返回最新的两条并提示还有更多: %+v", paged)
	}
	older := decode[activityPage](t, s.human(t, http.MethodGet, base+"?limit=2&before="+id(paged.Events[1].ID), nil))
	if len(older.Events) != 2 || older.Events[0].ID != all.Events[2].ID {
		t.Fatalf("before 应该接着上一页往后翻: %+v", older)
	}

	moved := decode[activityPage](t, s.human(t, http.MethodGet, base+"?types=card.moved", nil))
	if got := activityTypes(moved); len(got) != 1 || got[0] != "card.moved" {
		t.Fatalf("按类型筛选不对: %v", got)
	}
	two := decode[activityPage](t, s.human(t, http.MethodGet, base+"?types=card.moved,comment.added", nil))
	if got := activityTypes(two); len(got) != 2 {
		t.Fatalf("多个类型应该取并集: %v", got)
	}
	system := decode[activityPage](t, s.human(t, http.MethodGet, base+"?actor_kind=system", nil))
	if len(system.Events) == 0 {
		t.Fatalf("移动进开始列应该触发了自动化: %+v", all.Events)
	}
	for _, e := range system.Events {
		if e.ActorKind != "system" {
			t.Fatalf("混进了别的操作者: %+v", e)
		}
	}
	name := system.Events[0].Actor
	byName := decode[activityPage](t, s.human(t, http.MethodGet, base+"?actor="+url.QueryEscape(name), nil))
	if len(byName.Events) != len(system.Events) || byName.Latest != all.Latest || len(byName.Actors) != len(all.Actors) {
		t.Fatalf("按名称筛选只该留下这个操作者的事件，最新序号和操作者列表不应该变: %+v", byName)
	}
	for _, e := range byName.Events {
		if e.Actor != name {
			t.Fatalf("混进了别的操作者: %+v", e)
		}
	}
	if nobody := decode[activityPage](t, s.human(t, http.MethodGet, base+"?actor=nobody", nil)); len(nobody.Events) != 0 {
		t.Fatalf("不存在的操作者应该返回空列表: %+v", nobody)
	}
	if none := decode[activityPage](t, s.human(t, http.MethodGet, base+"?types=card.deleted", nil)); len(none.Events) != 0 || none.HasMore {
		t.Fatalf("没有匹配应该返回空列表: %+v", none)
	}
	other := s.newCard(t, b, b.Columns[1].ID, "活动二")
	own := decode[activityPage](t, s.human(t, http.MethodGet, base+"?card_id="+id(c.ID), nil))
	if len(own.Events) < 3 {
		t.Fatalf("按卡片筛选应该包含建卡、评论、移动: %+v", own.Events)
	}
	for _, e := range own.Events {
		if e.CardID != c.ID {
			t.Fatalf("混进了别的卡片的事件: %+v", e)
		}
	}
	if only := decode[activityPage](t, s.human(t, http.MethodGet, base+"?card_id="+id(other.ID)+"&types=comment.added", nil)); len(only.Events) != 0 {
		t.Fatalf("另一张卡片没有评论: %+v", only)
	}
	wantFail(t, "负数卡片", s.human(t, http.MethodGet, base+"?card_id=-1", nil), http.StatusBadRequest, "ErrInvalidInput")
	if resp := s.human(t, http.MethodGet, base+"?types=card.deleted", nil); !strings.Contains(string(resp.Body), `"events":[]`) {
		t.Fatalf("没有匹配应该返回 \"events\":[]: %s", resp.Body)
	}
}

// TestActivityInvalid 验证活动流的参数错误都返回 400，看板不存在返回 404。
func TestActivityInvalid(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "ACI")
	base := "/boards/" + id(b.ID) + "/activity"
	for name, query := range map[string]string{
		"负数 before":    "?before=-1",
		"负数 limit":     "?limit=-1",
		"不认识的类型":       "?types=card.exploded",
		"多写了逗号":        "?types=card.moved,",
		"不认识的操作者类型":    "?actor_kind=robot",
		"操作者名称过长":      "?actor=" + strings.Repeat("a", 65),
		"before 不是数字":  "?before=abc",
		"limit 不是数字":   "?limit=abc",
		"类型里夹带 SQL":    "?types=" + url.QueryEscape("card.moved') OR 1=1 --"),
		"操作者类型里夹带 SQL": "?actor_kind=" + url.QueryEscape("human' OR '1'='1"),
	} {
		if resp := s.human(t, http.MethodGet, base+query, nil); resp.Code != http.StatusBadRequest {
			t.Errorf("%s 应该返回 400，实际 %d %s", name, resp.Code, resp.Body)
		}
	}
	wantFail(t, "看板不存在", s.human(t, http.MethodGet, "/boards/9999/activity", nil), http.StatusNotFound, "ErrBoardNotFound")
}

// TestActivityExcludeTypes 验证 exclude_types：改标签写的 card.tags_updated 默认能读到，
// 带 exclude_types 就不返回；不认识的类型返回 400。
func TestActivityExcludeTypes(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "EXC")
	c := s.newCard(t, b, b.Columns[1].ID, "标签事件")
	s.human(t, http.MethodPatch, "/cards/"+id(c.ID), map[string]any{"tags": []string{"Bug"}})
	s.human(t, http.MethodPatch, "/cards/"+id(c.ID), map[string]any{"priority": 2})
	base := "/boards/" + id(b.ID) + "/activity"

	has := func(p activityPage, typ string) bool { return slices.Contains(activityTypes(p), typ) }
	all := decode[activityPage](t, s.human(t, http.MethodGet, base, nil))
	if !has(all, "card.tags_updated") || !has(all, "card.updated") {
		t.Fatalf("不带参数应该同时有标签事件和修改事件: %v", activityTypes(all))
	}
	kept := decode[activityPage](t, s.human(t, http.MethodGet, base+"?exclude_types=card.tags_updated,board.labels_updated", nil))
	if has(kept, "card.tags_updated") || !has(kept, "card.updated") || len(kept.Events) != len(all.Events)-1 {
		t.Fatalf("exclude_types 只应该去掉标签事件: %v", activityTypes(kept))
	}
	wantFail(t, "不认识的排除类型", s.human(t, http.MethodGet, base+"?exclude_types=card.exploded", nil), http.StatusBadRequest, "ErrInvalidInput")
}
