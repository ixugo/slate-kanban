package kanban_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// minimalTemplateJSON 构造一份合法的最小项目模板 JSON，供多个测试复用。
func minimalTemplateJSON(t *testing.T, name string) []byte {
	t.Helper()
	doc := map[string]any{
		"format":    kanban.ProjectTemplateFormat,
		"version":   kanban.ProjectTemplateVersion,
		"name":      name,
		"tags":      []any{},
		"engineers": []any{},
		"columns": []any{
			map[string]any{"name": "待办", "instruction": "", "is_done": false, "next_column": -1},
		},
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("编码模板 JSON 失败: %v", err)
	}
	return b
}

// TestValidateProjectTemplateJSON_rejectsIllegalNextColumn 非法 next_column 必须拒绝。
func TestValidateProjectTemplateJSON_rejectsIllegalNextColumn(t *testing.T) {
	doc := map[string]any{
		"format": kanban.ProjectTemplateFormat, "version": kanban.ProjectTemplateVersion, "name": "X",
		"columns": []any{map[string]any{"name": "A", "instruction": "", "is_done": false, "next_column": 3}},
	}
	raw, _ := json.Marshal(doc)
	if _, err := kanban.ParseProjectTemplateJSON(raw); err == nil {
		t.Fatal("next_column 越界应该失败")
	}
}

// TestValidateProjectTemplateJSON_rejectsReservedBuiltinName 内置模板名不能作为自定义模板名。
func TestValidateProjectTemplateJSON_rejectsReservedBuiltinName(t *testing.T) {
	for _, name := range []string{kanban.TemplateDev, kanban.TemplateSimple, kanban.TemplateEmpty} {
		raw := minimalTemplateJSON(t, name)
		if _, err := kanban.ParseProjectTemplateJSON(raw); err == nil {
			t.Fatalf("保留名 %q 应该被拒绝", name)
		}
	}
}

// TestValidateProjectTemplateJSON_allowsCaseVariantOfBuiltin 内置名区分大小写，Dev 可以。
func TestValidateProjectTemplateJSON_allowsCaseVariantOfBuiltin(t *testing.T) {
	raw := minimalTemplateJSON(t, "Dev")
	if _, err := kanban.ParseProjectTemplateJSON(raw); err != nil {
		t.Fatalf("Dev 与 dev 区分大小写，应合法: %v", err)
	}
}

// TestValidateProjectTemplateJSON_rejectsEmptyBody 空内容必须拒绝。
func TestValidateProjectTemplateJSON_rejectsEmptyBody(t *testing.T) {
	if _, err := kanban.ParseProjectTemplateJSON(nil); err == nil {
		t.Fatal("空文件应该失败")
	}
	if _, err := kanban.ParseProjectTemplateJSON([]byte("   ")); err == nil {
		t.Fatal("只有空白应该失败")
	}
}

// TestValidateProjectTemplateJSON_rejectsOversize 超过体积上限必须拒绝。
func TestValidateProjectTemplateJSON_rejectsOversize(t *testing.T) {
	huge := minimalTemplateJSON(t, "大")
	huge = append(huge, make([]byte, kanban.MaxProjectTemplateImportBytes)...)
	if _, err := kanban.ParseProjectTemplateJSON(huge); err == nil {
		t.Fatal("超大文件应该失败")
	}
}

// TestImportProjectTemplate_rejectsDuplicateWithoutReplace 重名且未要求覆盖应返回 409。
func TestImportProjectTemplate_rejectsDuplicateWithoutReplace(t *testing.T) {
	core, _ := newBare(t)
	ctx := context.Background()
	raw := minimalTemplateJSON(t, "团队流")
	if _, err := core.ImportProjectTemplate(ctx, testActor, raw, false); err != nil {
		t.Fatalf("首次导入: %v", err)
	}
	if _, err := core.ImportProjectTemplate(ctx, testActor, raw, false); !errors.Is(err, kanban.ErrProjectTemplateNameTaken) {
		t.Fatalf("重名应 ErrProjectTemplateNameTaken，实际 %v", err)
	}
}

// TestImportProjectTemplate_replaceReplacesContent 覆盖导入应替换旧内容。
func TestImportProjectTemplate_replaceReplacesContent(t *testing.T) {
	core, _ := newBare(t)
	ctx := context.Background()
	name := "可覆盖"
	first := minimalTemplateJSON(t, name)
	if _, err := core.ImportProjectTemplate(ctx, testActor, first, false); err != nil {
		t.Fatalf("首次: %v", err)
	}
	second := map[string]any{
		"format": kanban.ProjectTemplateFormat, "version": kanban.ProjectTemplateVersion, "name": name,
		"columns": []any{map[string]any{"name": "新列", "instruction": "说明", "is_done": true, "next_column": -1}},
	}
	raw2, _ := json.Marshal(second)
	got, err := core.ImportProjectTemplate(ctx, testActor, raw2, true)
	if err != nil {
		t.Fatalf("覆盖: %v", err)
	}
	if len(got.Columns) != 1 || got.Columns[0].Name != "新列" {
		t.Fatalf("覆盖后内容不对: %+v", got.Columns)
	}
	list, err := core.ListProjectTemplates(ctx, testActor)
	if err != nil || len(list) != 2 {
		t.Fatalf("列表应只有内置模板和覆盖后的模板: %v %+v", err, list)
	}
}

// TestImportProjectTemplate_concurrentReplace 并发覆盖不应留下半成品或丢名。
func TestImportProjectTemplate_concurrentReplace(t *testing.T) {
	core, _ := newBare(t)
	ctx := context.Background()
	name := "并发名"
	raw := minimalTemplateJSON(t, name)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := core.ImportProjectTemplate(ctx, testActor, raw, true)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("并发覆盖不应失败: %v", err)
		}
	}
	list, err := core.ListProjectTemplates(ctx, testActor)
	if err != nil || len(list) != 2 || list[1].Name != name {
		t.Fatalf("应只剩内置模板和一个同名模板: %v %+v", err, list)
	}
}

// TestExportProjectTemplateFromBoard_omitsBoardFields 从看板导出不含本机路径和数据库 id。
func TestExportProjectTemplateFromBoard_omitsBoardFields(t *testing.T) {
	core, _ := newBare(t)
	ctx := context.Background()
	d := mustCreateBoard(t, core, kanban.CreateBoardInput{Name: "导出源", Slug: "SRC", ProjectPath: "/secret/path"})
	file, err := core.ExportProjectTemplateFromBoard(ctx, testActor, d.ID)
	if err != nil {
		t.Fatalf("导出: %v", err)
	}
	if file.Name != "导出源" {
		t.Fatalf("默认名应是看板名，得到 %q", file.Name)
	}
	text, _ := json.Marshal(file)
	s := string(text)
	for _, bad := range []string{`"project_path"`, `"/secret/path"`, `"board_id"`} {
		if strings.Contains(s, bad) {
			t.Fatalf("不应含 %s", bad)
		}
	}
	if len(file.Columns) == 0 {
		t.Fatal("dev 看板应有列")
	}
	for i, c := range file.Columns {
		if c.NextColumn < -1 || c.NextColumn >= len(file.Columns) {
			t.Fatalf("列 %d next_column 不合法: %d", i, c.NextColumn)
		}
	}
}

// TestCreateBoardFromProjectTemplate_decoupled 用模板建板后改模板不影响看板。
func TestCreateBoardFromProjectTemplate_decoupled(t *testing.T) {
	core, _ := newBare(t)
	ctx := context.Background()
	raw := map[string]any{
		"format": kanban.ProjectTemplateFormat, "version": kanban.ProjectTemplateVersion, "name": "母版",
		"tags":    []any{map[string]any{"text": "甲", "color": "green-2"}},
		"columns": []any{map[string]any{"name": "列甲", "instruction": "", "is_done": false, "next_column": -1}},
	}
	b, _ := json.Marshal(raw)
	tpl, err := core.ImportProjectTemplate(ctx, testActor, b, false)
	if err != nil {
		t.Fatalf("导入: %v", err)
	}
	d, err := core.CreateBoard(ctx, testActor, kanban.CreateBoardInput{Name: "子板", Slug: "CHD", ProjectTemplateID: tpl.ID})
	if err != nil {
		t.Fatalf("建板: %v", err)
	}
	if len(d.Tags) != 1 || d.Tags[0].Text != "甲" || d.Columns[0].Name != "列甲" {
		t.Fatalf("应从模板复制: tags=%+v cols=%+v", d.Tags, columnNames(d.Columns))
	}
	replace := map[string]any{
		"format": kanban.ProjectTemplateFormat, "version": kanban.ProjectTemplateVersion, "name": "母版",
		"tags":    []any{map[string]any{"text": "乙", "color": ""}},
		"columns": []any{map[string]any{"name": "列乙", "instruction": "", "is_done": false, "next_column": -1}},
	}
	r, _ := json.Marshal(replace)
	if _, err := core.ImportProjectTemplate(ctx, testActor, r, true); err != nil {
		t.Fatalf("改模板: %v", err)
	}
	d2, err := core.GetBoard(ctx, d.ID)
	if err != nil {
		t.Fatalf("读板: %v", err)
	}
	if d2.Tags[0].Text != "甲" || d2.Columns[0].Name != "列甲" {
		t.Fatal("看板不应随模板变化")
	}
}

// TestDeleteProjectTemplate_removesRow 删除后列表不含该项。
func TestDeleteProjectTemplate_removesRow(t *testing.T) {
	core, _ := newBare(t)
	ctx := context.Background()
	tpl, err := core.ImportProjectTemplate(ctx, testActor, minimalTemplateJSON(t, "删我"), false)
	if err != nil {
		t.Fatalf("导入: %v", err)
	}
	if err := core.DeleteProjectTemplate(ctx, testActor, tpl.ID); err != nil {
		t.Fatalf("删除: %v", err)
	}
	list, _ := core.ListProjectTemplates(ctx, testActor)
	for _, item := range list {
		if item.ID == tpl.ID {
			t.Fatal("删除后仍在列表")
		}
	}
}

// TestBuiltinProjectTemplateUsesLocalizedDevelopmentFlow 验证内置模板可以用列表 ID 创建带五列和规则的本地化项目。
func TestBuiltinProjectTemplateUsesLocalizedDevelopmentFlow(t *testing.T) {
	core, _ := newBare(t)
	ctx := t.Context()
	list, err := core.ListProjectTemplates(ctx, testActor)
	if err != nil || len(list) != 1 || list[0].Name != kanban.TemplateDev {
		t.Fatalf("应包含内置研发模板: %v %+v", err, list)
	}
	board, err := core.CreateBoard(ctx, testActor, kanban.CreateBoardInput{
		Name: "普通项目", Template: kanban.TemplateEmpty, ProjectTemplateID: list[0].ID, Locale: "zh-CN",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(board.Columns) != 5 || board.Columns[0].Name != "收件箱" || len(board.Rules) != 3 {
		t.Fatalf("研发模板应恢复本地化列和规则: columns=%+v rules=%+v", columnNames(board.Columns), board.Rules)
	}
	boards, err := core.ListBoards(ctx)
	if err != nil || len(boards) != 1 || boards[0].ID != board.ID {
		t.Fatalf("项目列表只含实际创建的项目: %v %+v", err, boards)
	}
	if err := core.DeleteProjectTemplate(ctx, testActor, list[0].ID); err == nil {
		t.Fatal("不允许删除内置模板")
	}
}
