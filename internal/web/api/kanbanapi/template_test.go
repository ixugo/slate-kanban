package kanbanapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// TestTemplateImportListDelete 走一遍导入、列表、重名拒绝、覆盖、删除。
func TestTemplateImportListDelete(t *testing.T) {
	s := newStack(t)
	body := map[string]any{
		"format": "slate-project-template", "version": 1, "name": "REST流",
		"columns": []any{map[string]any{"name": "列1", "instruction": "", "is_done": false, "next_column": -1}},
	}
	resp := s.human(t, http.MethodPost, "/templates", body)
	if resp.Code != http.StatusOK {
		t.Fatalf("导入: %d %s", resp.Code, resp.Body)
	}
	list := s.human(t, http.MethodGet, "/templates", nil)
	var items []struct {
		Name string `json:"name"`
	}
	list.JSON(t, &items)
	if len(items) != 1 || items[0].Name != "REST流" {
		t.Fatalf("列表: %+v", items)
	}
	dup := s.human(t, http.MethodPost, "/templates", body)
	wantFail(t, "重名", dup, http.StatusConflict, "ErrProjectTemplateNameTaken")
	replace := s.human(t, http.MethodPost, "/templates?replace=true", body)
	if replace.Code != http.StatusOK {
		t.Fatalf("覆盖: %d", replace.Code)
	}
	var tpl struct {
		ID int64 `json:"id"`
	}
	resp.JSON(t, &tpl)
	del := s.human(t, http.MethodDelete, "/templates/"+id(tpl.ID), nil)
	if del.Code != http.StatusOK {
		t.Fatalf("删除: %d", del.Code)
	}
}

// TestBoardTemplateExport 从看板导出模板 JSON。
func TestBoardTemplateExport(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "TPLX")
	resp := s.human(t, http.MethodGet, "/boards/"+id(b.ID)+"/template-export", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("导出: %d %s", resp.Code, resp.Body)
	}
	var file kanban.ProjectTemplateFile
	resp.JSON(t, &file)
	if file.Format != kanban.ProjectTemplateFormat || file.Name != "看板 TPLX" || len(file.Columns) == 0 {
		t.Fatalf("内容不对: %+v", file)
	}
	text, _ := json.Marshal(file)
	if bytes.Contains(text, []byte("project_path")) {
		t.Fatal("不应含 project_path")
	}
}

// TestTemplateImportRejectsBuiltinName 内置名不能导入。
func TestTemplateImportRejectsBuiltinName(t *testing.T) {
	s := newStack(t)
	body := map[string]any{"format": "slate-project-template", "version": 1, "name": "dev", "columns": []any{}}
	resp := s.human(t, http.MethodPost, "/templates", body)
	wantFail(t, "内置名", resp, http.StatusBadRequest, "ErrInvalidInput")
}
