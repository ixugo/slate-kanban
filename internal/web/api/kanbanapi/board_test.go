package kanbanapi_test

import (
	"net/http"
	"strings"
	"testing"
)

// TestBoardLifecycle 验证看板的增删改查：空列表是 [] 不是 null；创建返回五列和起步规则；
// 修改只更新出现的字段；删除幂等，删除后读取返回 404。
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
		t.Fatalf("只应该改名称，说明保持不变: %+v", changed)
	}

	for range 2 {
		wantEmptyObject(t, "删除", s.human(t, http.MethodDelete, "/boards/"+id(b.ID), nil))
	}
	wantFail(t, "删除后读取", s.human(t, http.MethodGet, "/boards/"+id(b.ID), nil), http.StatusNotFound, "ErrBoardNotFound")
}

// TestBoardPermissions 验证权限原样来自 Core：agent 不能创建、改名、删除看板，
// 但可以在项目路径为空时写项目路径，写过之后就被锁住。
func TestBoardPermissions(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "PERM")
	path := "/boards/" + id(b.ID)

	wantFail(t, "agent 创建", s.agent(t, "claude", http.MethodPost, "/boards", map[string]string{"name": "x"}), http.StatusForbidden, "ErrHumanOnly")
	wantFail(t, "agent 改名", s.agent(t, "claude", http.MethodPatch, path, map[string]string{"name": "x"}), http.StatusForbidden, "ErrHumanOnly")
	wantFail(t, "agent 删除", s.agent(t, "claude", http.MethodDelete, path, nil), http.StatusForbidden, "ErrHumanOnly")

	set := decode[board](t, s.agent(t, "claude", http.MethodPatch, path, map[string]string{"project_path": "/work/demo"}))
	if set.ProjectPath != "/work/demo" {
		t.Fatalf("agent 应该能写空的项目路径: %+v", set)
	}
	wantFail(t, "agent 改已有路径", s.agent(t, "claude", http.MethodPatch, path, map[string]string{"project_path": "/other"}),
		http.StatusForbidden, "ErrProjectPathLocked")
	if got := decode[board](t, s.human(t, http.MethodGet, path, nil)); got.Name != b.Name {
		t.Fatalf("被拒绝的修改不应该生效: %+v", got)
	}
}

// TestBoardInvalidInput 验证输入错误：名称为空返回 400；路径里的编号不是数字返回 400，
// 数字溢出返回 400，不存在的编号（包括 0 和负数）返回 404。
func TestBoardInvalidInput(t *testing.T) {
	s := newStack(t)
	wantFail(t, "名称为空", s.human(t, http.MethodPost, "/boards", map[string]string{"name": " "}), http.StatusBadRequest, "ErrInvalidInput")
	for path, want := range map[string]int{
		"/boards/abc":                  http.StatusBadRequest,
		"/boards/99999999999999999999": http.StatusBadRequest,
		"/boards/0":                    http.StatusNotFound,
		"/boards/-1":                   http.StatusNotFound,
		"/boards/12345":                http.StatusNotFound,
	} {
		if resp := s.human(t, http.MethodGet, path, nil); resp.Code != want {
			t.Errorf("GET %s 期望 %d，实际 %d %s", path, want, resp.Code, resp.Body)
		}
	}
}

// TestBoardPathNotOverridable 验证路径里的看板编号不会被请求体里的同名字段覆盖：
// 修改看板 a 的请求里夹带看板 b 的编号，结果只有 a 变了。
func TestBoardPathNotOverridable(t *testing.T) {
	s := newStack(t)
	a, b := s.newBoard(t, "AAA"), s.newBoard(t, "BBB")
	body := map[string]any{"name": "改了", "board_id": b.ID, "BoardID": b.ID, "id": b.ID}
	if got := decode[board](t, s.human(t, http.MethodPatch, "/boards/"+id(a.ID), body)); got.ID != a.ID || got.Name != "改了" {
		t.Fatalf("应该修改路径里的看板 a: %+v", got)
	}
	if got := decode[board](t, s.human(t, http.MethodGet, "/boards/"+id(b.ID), nil)); got.Name != b.Name {
		t.Fatalf("看板 b 不应该被改: %+v", got)
	}
}
