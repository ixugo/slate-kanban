package kanbanapi_test

import (
	"net/http"
	"slices"
	"testing"
)

// columnIDs 取出一组列的编号。
func columnIDs(cols []column) []int64 {
	ids := make([]int64, len(cols))
	for i, c := range cols {
		ids[i] = c.ID
	}
	return ids
}

// TestColumnLifecycle 验证列的增改排删：新增带上给 agent 的指令；修改只更新出现的字段；
// 排序按请求里的顺序返回；有卡片的列不指定 move_to 删不掉，指定后卡片搬过去；删除幂等。
func TestColumnLifecycle(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "COL")
	base := "/boards/" + id(b.ID) + "/columns"

	made := decode[map[string]any](t, s.human(t, http.MethodPost, base,
		map[string]any{"name": "灰度", "description": "小范围验证", "instruction": "只做验证"}))
	colID := int64(made["id"].(float64))
	if made["name"] != "灰度" || made["instruction"] != "只做验证" {
		t.Fatalf("新增的列不对: %v", made)
	}

	patched := decode[map[string]any](t, s.human(t, http.MethodPatch, base+"/"+id(colID), map[string]string{"name": "灰度发布"}))
	if patched["name"] != "灰度发布" || patched["description"] != "小范围验证" {
		t.Fatalf("只应该改名称，说明保持不变: %v", patched)
	}

	cols := decode[board](t, s.human(t, http.MethodGet, "/boards/"+id(b.ID), nil)).Columns
	reversed := columnIDs(cols)
	slices.Reverse(reversed)
	ordered := decode[[]column](t, s.human(t, http.MethodPut, base+"/order", map[string]any{"ids": reversed}))
	if !slices.Equal(columnIDs(ordered), reversed) {
		t.Fatalf("应该按请求的顺序返回: %v 期望 %v", columnIDs(ordered), reversed)
	}
	wantFail(t, "少了一列", s.human(t, http.MethodPut, base+"/order", map[string]any{"ids": reversed[1:]}), http.StatusConflict, "ErrColumnOrderStale")

	s.newCard(t, b, colID, "留在灰度里")
	wantFail(t, "有卡片不指定去向", s.human(t, http.MethodDelete, base+"/"+id(colID), nil), http.StatusConflict, "ErrColumnNotEmpty")
	target := cols[0].ID
	for range 2 {
		wantEmptyObject(t, "删除", s.human(t, http.MethodDelete, base+"/"+id(colID)+"?move_to="+id(target), nil))
	}
	list := decode[struct {
		Cards []card `json:"cards"`
	}](t, s.human(t, http.MethodGet, "/boards/"+id(b.ID)+"/cards", nil))
	if len(list.Cards) != 1 || list.Cards[0].ColumnID != target {
		t.Fatalf("卡片应该搬到了 move_to 指定的列: %+v", list.Cards)
	}
}

// TestColumnHumanOnly 验证 agent 不能增改排删列，每一项都返回 403。
func TestColumnHumanOnly(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "COL")
	base := "/boards/" + id(b.ID) + "/columns"
	col := id(b.Columns[0].ID)
	checks := map[string]struct {
		method, path string
		body         any
	}{
		"新增": {http.MethodPost, base, map[string]string{"name": "x"}},
		"修改": {http.MethodPatch, base + "/" + col, map[string]string{"name": "x"}},
		"排序": {http.MethodPut, base + "/order", map[string]any{"ids": columnIDs(b.Columns)}},
		"删除": {http.MethodDelete, base + "/" + col, nil},
	}
	for name, c := range checks {
		wantFail(t, "agent "+name, s.agent(t, "claude", c.method, c.path, c.body), http.StatusForbidden, "ErrHumanOnly")
	}
}

// TestColumnPathNotOverridable 验证路径里的列编号不会被查询参数或请求体覆盖：
// 删除列 a 的请求里夹带列 b 的编号，结果只有 a 被删。
func TestColumnPathNotOverridable(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "COL")
	base := "/boards/" + id(b.ID) + "/columns"
	a := decode[column](t, s.human(t, http.MethodPost, base, map[string]string{"name": "甲"}))
	other := decode[column](t, s.human(t, http.MethodPost, base, map[string]string{"name": "乙"}))
	wantEmptyObject(t, "删除", s.human(t, http.MethodDelete, base+"/"+id(a.ID)+"?ColumnID="+id(other.ID)+"&column_id="+id(other.ID), nil))
	cols := columnIDs(decode[board](t, s.human(t, http.MethodGet, "/boards/"+id(b.ID), nil)).Columns)
	if slices.Contains(cols, a.ID) || !slices.Contains(cols, other.ID) {
		t.Fatalf("应该只删掉路径里的列 a: %v", cols)
	}
}
