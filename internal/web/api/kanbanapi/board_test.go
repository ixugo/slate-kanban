package kanbanapi_test

import (
	"net/http"
	"os"
	"path/filepath"
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

// TestCheckProjectPath 使用服务端的实际文件系统验证目录，不创建看板。
func TestCheckProjectPath(t *testing.T) {
	s := newStack(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	foreignPath := `C:\project`
	if filepath.IsAbs(foreignPath) {
		foreignPath = "/project"
	}
	for _, tt := range []struct {
		name, path, reason string
	}{
		{name: "存在的目录", path: "  " + dir + "  "},
		{name: "选填", path: "  "},
		{name: "不存在的目录", path: filepath.Join(dir, "missing"), reason: "ErrProjectPathNotFound"},
		{name: "普通文件", path: file, reason: "ErrProjectPathNotFound"},
		{name: "文件下的子目录", path: filepath.Join(file, "child"), reason: "ErrProjectPathNotFound"},
		{name: "相对路径", path: "relative/dir", reason: "ErrInvalidInput"},
		{name: "其他系统的绝对路径", path: foreignPath, reason: "ErrInvalidInput"},
		{name: "非法字符", path: dir + "\x00", reason: "ErrInvalidInput"},
		{name: "超过长度", path: dir + strings.Repeat("a", 512), reason: "ErrInvalidInput"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resp := s.human(t, http.MethodPost, "/boards/check-project-path", map[string]string{"project_path": tt.path})
			if tt.reason == "" {
				wantEmptyObject(t, tt.name, resp)
			} else {
				wantFail(t, tt.name, resp, http.StatusBadRequest, tt.reason)
			}
		})
	}
	t.Run("无法确认目录", func(t *testing.T) {
		loop := filepath.Join(dir, "loop")
		if err := os.Symlink(loop, loop); err != nil {
			t.Skipf("当前系统无法创建符号链接: %v", err)
		}
		wantFail(t, "无法确认目录", s.human(t, http.MethodPost, "/boards/check-project-path", map[string]string{"project_path": loop}),
			http.StatusBadRequest, "ErrProjectPathCheckFailed")
	})
	if boards := decode[[]board](t, s.human(t, http.MethodGet, "/boards", nil)); len(boards) != 0 {
		t.Fatalf("检查目录不应该创建看板: %+v", boards)
	}
}

// TestBoardProjectPath 验证 REST 创建、更新时都重新检查目录，不依赖网页预先校验。
func TestBoardProjectPath(t *testing.T) {
	s := newStack(t)
	dir := t.TempDir()
	vanished := filepath.Join(dir, "vanished")
	if err := os.Mkdir(vanished, 0o700); err != nil {
		t.Fatal(err)
	}
	wantEmptyObject(t, "目录存在", s.human(t, http.MethodPost, "/boards/check-project-path", map[string]string{"project_path": vanished}))
	if err := os.Remove(vanished); err != nil {
		t.Fatal(err)
	}
	wantFail(t, "校验通过后目录消失，拒绝创建", s.human(t, http.MethodPost, "/boards", map[string]string{"name": "Invalid", "project_path": vanished}),
		http.StatusBadRequest, "ErrProjectPathNotFound")
	if boards := decode[[]board](t, s.human(t, http.MethodGet, "/boards", nil)); len(boards) != 0 {
		t.Fatalf("无效目录不应该创建看板: %+v", boards)
	}
	b := decode[board](t, s.human(t, http.MethodPost, "/boards", map[string]string{"name": "Valid", "project_path": " " + dir + " "}))
	if b.ProjectPath != dir {
		t.Fatalf("创建时应保存整理后的目录: %+v", b)
	}
	path := "/boards/" + id(b.ID)
	wantFail(t, "目录不存在，拒绝更新", s.human(t, http.MethodPatch, path, map[string]string{"name": "Invalid", "project_path": vanished}),
		http.StatusBadRequest, "ErrProjectPathNotFound")
	if got := decode[board](t, s.human(t, http.MethodGet, path, nil)); got.ProjectPath != dir || got.Name != b.Name {
		t.Fatalf("失败的保存不应改变任何字段: %+v", got)
	}
	updatedDir := filepath.Join(dir, "new")
	if err := os.Mkdir(updatedDir, 0o700); err != nil {
		t.Fatal(err)
	}
	set := decode[board](t, s.human(t, http.MethodPatch, path, map[string]string{"project_path": updatedDir}))
	if set.ProjectPath != updatedDir {
		t.Fatalf("应该能修改项目路径: %+v", set)
	}
	if err := os.Remove(updatedDir); err != nil {
		t.Fatal(err)
	}
	wantFail(t, "再次保存已消失的目录", s.human(t, http.MethodPatch, path, map[string]string{"project_path": updatedDir}),
		http.StatusBadRequest, "ErrProjectPathNotFound")
	if got := decode[board](t, s.human(t, http.MethodPatch, path, map[string]string{"name": "Renamed"})); got.ProjectPath != updatedDir || got.Name != "Renamed" {
		t.Fatalf("未提交目录时应保留原路径并允许修改其他字段: %+v", got)
	}
	if got := decode[board](t, s.human(t, http.MethodPatch, path, map[string]string{"project_path": ""})); got.ProjectPath != "" {
		t.Fatalf("应该能清空项目路径: %+v", got)
	}
}

// TestBoardTags 验证通过 REST 维护标签库：新看板自带 6 个标签，整体替换后返回主键，文字可以为空，
// 非法颜色返回 400，任务按 tag_ids 引用，看板详情和快照里都带着标签库。
func TestBoardTags(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "LAB")
	path := "/boards/" + id(b.ID)
	if got := decode[board](t, s.human(t, http.MethodGet, path, nil)); len(got.Tags) != 6 {
		t.Fatalf("新看板应该自带 6 个标签: %+v", got.Tags)
	}

	set := decode[board](t, s.human(t, http.MethodPatch, path, map[string]any{"tags": []map[string]any{{"text": "bug", "color": "red-2"}, {"text": "", "color": "green-3"}}}))
	if len(set.Tags) != 2 || set.Tags[0]["text"] != "bug" || set.Tags[1]["text"] != "" || set.Tags[0]["id"].(float64) == 0 {
		t.Fatalf("标签库不对: %+v", set.Tags)
	}
	tagID := int64(set.Tags[1]["id"].(float64))
	wantFail(t, "非法颜色", s.human(t, http.MethodPatch, path, map[string]any{"tags": []map[string]any{{"text": "x", "color": "teal-9"}}}),
		http.StatusBadRequest, "ErrInvalidInput")

	c := s.newCard(t, b, b.Columns[1].ID, "带标签")
	got := decode[card](t, s.human(t, http.MethodPatch, "/cards/"+id(c.ID), map[string]any{"tag_ids": []int64{tagID}}))
	if len(got.TagIDs) != 1 || got.TagIDs[0] != tagID {
		t.Fatalf("任务应该按 tag_ids 引用标签: %+v", got)
	}

	snap := s.human(t, http.MethodGet, path+"/snapshot", nil)
	if !strings.Contains(string(snap.Body), `"tags":[{"id":`) || !strings.Contains(string(snap.Body), `"tag_ids":[`) {
		t.Fatalf("快照里应该带标签库和任务的 tag_ids: %s", snap.Body)
	}
}
