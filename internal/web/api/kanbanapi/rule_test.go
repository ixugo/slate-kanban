package kanbanapi_test

import (
	"net/http"
	"testing"
)

// rule 是规则响应里测试关心的字段。
type rule struct {
	ID       int64            `json:"id"`
	Name     string           `json:"name"`
	Enabled  bool             `json:"enabled"`
	Trigger  string           `json:"trigger"`
	ColumnID int64            `json:"column_id"`
	Actions  []map[string]any `json:"actions"`
}

// TestRuleLifecycle 验证规则的增改删：新增后出现在看板详情里；修改只更新出现的字段；
// 动作传空数组返回 400；删除幂等。
func TestRuleLifecycle(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "RUL")
	done := b.Columns[4].ID
	made := decode[rule](t, s.human(t, http.MethodPost, "/boards/"+id(b.ID)+"/rules", map[string]any{
		"name": "完成时记结束时间", "enabled": true, "trigger": "card_entered", "column_id": done,
		"actions": []map[string]any{{"type": "set_finished_at"}},
	}))
	if made.ID == 0 || !made.Enabled || made.Trigger != "card_entered" || made.ColumnID != done || len(made.Actions) != 1 {
		t.Fatalf("新增的规则不对: %+v", made)
	}
	detail := decode[board](t, s.human(t, http.MethodGet, "/boards/"+id(b.ID), nil))
	found := false
	for _, r := range detail.Rules {
		found = found || int64(r["id"].(float64)) == made.ID
	}
	if !found {
		t.Fatalf("看板详情里应该有新规则: %v", detail.Rules)
	}

	path := "/rules/" + id(made.ID)
	patched := decode[rule](t, s.human(t, http.MethodPatch, path, map[string]any{"enabled": false}))
	if patched.Enabled || patched.Name != made.Name || len(patched.Actions) != 1 {
		t.Fatalf("只应该停用，其他保持不变: %+v", patched)
	}
	wantFail(t, "动作为空", s.human(t, http.MethodPatch, path, map[string]any{"actions": []any{}}), http.StatusBadRequest, "ErrInvalidInput")

	for range 2 {
		wantEmptyObject(t, "删除", s.human(t, http.MethodDelete, path, nil))
	}
}

// TestRuleHumanOnly 验证 agent 不能增改删规则，每一项都返回 403。
func TestRuleHumanOnly(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "RUL")
	made := decode[rule](t, s.human(t, http.MethodPost, "/boards/"+id(b.ID)+"/rules", map[string]any{
		"name": "r", "trigger": "card_created", "actions": []map[string]any{{"type": "add_tag", "tag": "new"}},
	}))
	path := "/rules/" + id(made.ID)
	checks := map[string]struct {
		method, path string
		body         any
	}{
		"新增": {http.MethodPost, "/boards/" + id(b.ID) + "/rules", map[string]any{"name": "x"}},
		"修改": {http.MethodPatch, path, map[string]any{"enabled": false}},
		"删除": {http.MethodDelete, path, nil},
	}
	for name, c := range checks {
		wantFail(t, "agent "+name, s.agent(t, "claude", c.method, c.path, c.body), http.StatusForbidden, "ErrHumanOnly")
	}
}
