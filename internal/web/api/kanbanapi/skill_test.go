package kanbanapi_test

import (
	"net/http"
	"slices"
	"testing"
)

func TestSkillManagement(t *testing.T) {
	s := newStack(t)
	type skill struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Body        string `json:"body"`
		Builtin     bool   `json:"builtin"`
		URI         string `json:"uri"`
	}
	list := func() []skill { return decode[[]skill](t, s.human(t, http.MethodGet, "/skills", nil)) }
	if !slices.ContainsFunc(list(), func(item skill) bool { return item.Name == "slate-kanban" && item.Builtin }) {
		t.Fatal("内置 Skill 应出现在列表中")
	}
	input := map[string]any{"name": "team-guide", "description": "使用时机：整理团队任务", "body": "# 指南\n\n    保留 Markdown 缩进\n"}
	created := decode[skill](t, s.human(t, http.MethodPost, "/skills", input))
	if created.Name != "team-guide" || created.Builtin || created.Body != input["body"] || created.URI != "skill://team-guide/SKILL.md" {
		t.Fatalf("新增 Skill 不正确: %+v", created)
	}
	if !slices.ContainsFunc(list(), func(item skill) bool {
		return item.Name == "team-guide" && item.Description == input["description"] && item.Body == ""
	}) {
		t.Fatal("列表应返回简介但不返回正文")
	}
	if got := decode[skill](t, s.human(t, http.MethodGet, "/skills/team-guide", nil)); got != created {
		t.Fatalf("读取 Skill 与创建结果不一致: %+v", got)
	}
	wantFail(t, "重名", s.human(t, http.MethodPost, "/skills", input), 409, "ErrSkillNameTaken")
	input["name"] = "slate-kanban"
	wantFail(t, "覆盖内置", s.human(t, http.MethodPost, "/skills", input), 409, "ErrSkillNameTaken")
	wantFail(t, "编辑内置", s.human(t, http.MethodPut, "/skills/slate-kanban", input), 400, "ErrInvalidInput")
	wantFail(t, "删除内置", s.human(t, http.MethodDelete, "/skills/slate-kanban", nil), 400, "ErrInvalidInput")
	input["name"], input["body"] = "team-updated", "更新内容"
	updated := decode[skill](t, s.human(t, http.MethodPut, "/skills/team-guide", input))
	if updated.Name != "team-updated" || updated.Body != "更新内容" {
		t.Fatalf("编辑 Skill 未生效: %+v", updated)
	}
	wantFail(t, "原名称已不存在", s.human(t, http.MethodGet, "/skills/team-guide", nil), 404, "ErrSkillNotFound")
	wantEmptyObject(t, "删除", s.human(t, http.MethodDelete, "/skills/team-updated", nil))
	wantFail(t, "已删除", s.human(t, http.MethodGet, "/skills/team-updated", nil), 404, "ErrSkillNotFound")
}
