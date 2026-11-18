package kanbanapi_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestBoardLifecycle(t *testing.T) {
	s := newStack(t)
	if resp := s.human(t, http.MethodGet, "/boards", nil); strings.TrimSpace(string(resp.Body)) != "[]" {
		t.Fatalf("没有看板时应该返回 []: %s", resp.Body)
	}
	b := decode[board](t, s.human(t, http.MethodPost, "/boards",
		map[string]string{"name": "开发", "slug": "dev", "description": "说明", "locale": "zh-CN"}))
	if b.ID == 0 || b.Slug != "DEV" || len(b.Columns) != 5 || len(b.Rules) == 0 {
		t.Fatalf("创建的看板不对: %+v", b)
	}
	if list := decode[[]board](t, s.human(t, http.MethodGet, "/boards", nil)); len(list) != 1 || list[0].ID != b.ID {
		t.Fatalf("列表里应该有这个看板: %+v", list)
	}
	if got := decode[board](t, s.human(t, http.MethodGet, "/boards/"+id(b.ID), nil)); len(got.Columns) != 5 {
		t.Fatalf("读取到的看板不对: %+v", got)
	}
	changed := decode[board](t, s.human(t, http.MethodPatch, "/boards/"+id(b.ID), map[string]string{"name": "新名称"}))
	if changed.Name != "新名称" || changed.Description != "说明" {
		t.Fatalf("只应该改名称: %+v", changed)
	}
	for range 2 {
		wantEmptyObject(t, "删除", s.human(t, http.MethodDelete, "/boards/"+id(b.ID), nil))
	}
	wantFail(t, "删除后读取", s.human(t, http.MethodGet, "/boards/"+id(b.ID), nil), http.StatusNotFound, "ErrBoardNotFound")
}

// TestBoardProjectPath 验证人可以通过 REST 设置项目路径。
func TestBoardProjectPath(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "PERM")
	path := "/boards/" + id(b.ID)
	set := decode[board](t, s.human(t, http.MethodPatch, path, map[string]string{"project_path": "/work/demo"}))
	if set.ProjectPath != "/work/demo" {
		t.Fatalf("应该能写项目路径: %+v", set)
	}
}

// TestBoardLabels 验证通过 REST 维护标签库：新建返回编号，改名同步到卡片，非法颜色返回 400，
// 看板详情和快照里都带着标签库，没有标签库时是空数组。
func TestBoardLabels(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "LAB")
	path := "/boards/" + id(b.ID)
	if got := decode[board](t, s.human(t, http.MethodGet, path, nil)); got.Labels == nil || len(got.Labels) != 0 {
		t.Fatalf("没有标签库应该是空数组: %+v", got.Labels)
	}

	set := decode[board](t, s.human(t, http.MethodPatch, path, map[string]any{"labels": []map[string]string{{"name": "bug", "color": "red-2"}}}))
	if len(set.Labels) != 1 || set.Labels[0]["name"] != "bug" || len(set.Labels[0]["id"].(string)) != 8 {
		t.Fatalf("标签库不对: %+v", set.Labels)
	}
	labelID := set.Labels[0]["id"].(string)
	wantFail(t, "非法颜色", s.human(t, http.MethodPatch, path, map[string]any{"labels": []map[string]string{{"name": "x", "color": "teal-9"}}}),
		http.StatusBadRequest, "ErrInvalidInput")

	snap := s.human(t, http.MethodGet, path+"/snapshot", nil)
	if !strings.Contains(string(snap.Body), `"labels":[{"id":"`+labelID+`"`) {
		t.Fatalf("快照里应该带标签库: %s", snap.Body)
	}
}
