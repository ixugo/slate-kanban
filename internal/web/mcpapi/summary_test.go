package mcpapi

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"slate-kanban/internal/core/kanban"
)

// TestSummarize 验证卡片摘要：不含描述，标签和依赖永远是数组（不是 null），派生状态和所在列都带上。
func TestSummarize(t *testing.T) {
	due := time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)
	view := &kanban.CardView{
		Card: &kanban.Card{
			ID: 7, ColumnID: 3, Key: "SLATE-7", Title: "做登录", Description: "很长很长的描述 secret-description",
			Priority: 2, ClaimedBy: "claude", DueAt: &due, Branch: "feat/login", Version: 4,
		},
		Column: "进行中", Waiting: true, ClaimActive: true,
	}
	raw, err := json.Marshal(summarize(view))
	if err != nil {
		t.Fatalf("序列化摘要失败: %v", err)
	}
	text := string(raw)
	if strings.Contains(text, "secret-description") || strings.Contains(text, `"description"`) {
		t.Errorf("摘要不应该包含描述: %s", text)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("摘要不是 JSON 对象: %v", err)
	}
	want := map[string]any{
		"id": float64(7), "key": "SLATE-7", "title": "做登录", "column": "进行中", "column_id": float64(3),
		"priority": float64(2), "waiting": true, "claim_active": true, "claimed_by": "claude",
		"branch": "feat/login", "version": float64(4), "done": false, "human_only": false,
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("字段 %s 期望 %v，实际 %v", k, v, got[k])
		}
	}
	for _, k := range []string{"tags", "waits_on"} {
		if list, ok := got[k].([]any); !ok || len(list) != 0 {
			t.Errorf("%s 应该是空数组而不是 null: %v", k, got[k])
		}
	}
	if got["due_at"] != "2026-11-20T09:00:00Z" || got["started_at"] != nil {
		t.Errorf("时间字段不对: due_at=%v started_at=%v", got["due_at"], got["started_at"])
	}
}

// TestBriefOf 验证写入结果的卡片字段：不含描述，也不含要靠整个看板才能算出的派生状态，
// 标签和依赖永远是数组，版本号和编号都在。
func TestBriefOf(t *testing.T) {
	raw, err := json.Marshal(briefOf(&kanban.Card{ID: 7, Key: "SLATE-7", Title: "做登录", Description: "secret-description", Version: 4}, nil))
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("不是 JSON 对象: %v", err)
	}
	for _, k := range []string{"description", "column", "done", "waiting", "claim_active"} {
		if _, has := got[k]; has {
			t.Errorf("不应该有字段 %s: %s", k, raw)
		}
	}
	if got["id"] != float64(7) || got["key"] != "SLATE-7" || got["version"] != float64(4) {
		t.Errorf("编号和版本号不对: %s", raw)
	}
	for _, k := range []string{"tags", "waits_on"} {
		if list, ok := got[k].([]any); !ok || len(list) != 0 {
			t.Errorf("%s 应该是空数组而不是 null: %v", k, got[k])
		}
	}
}

// TestSummarizeAllNeverNil 验证没有卡片时返回空切片，序列化出来是 [] 而不是 null。
func TestSummarizeAllNeverNil(t *testing.T) {
	raw, err := json.Marshal(summarizeAll(nil))
	if err != nil || string(raw) != "[]" {
		t.Errorf("没有卡片应该序列化成 []: %s %v", raw, err)
	}
}

// TestClampListLimit 验证列表数量：0 用默认值，超过上限压到上限，负数原样交给 Core 去拒绝。
func TestClampListLimit(t *testing.T) {
	for in, want := range map[int]int{0: 50, 1: 1, 50: 50, 200: 200, 201: 200, 1 << 30: 200, -1: -1} {
		if got := clampListLimit(in); got != want {
			t.Errorf("clampListLimit(%d) 期望 %d，实际 %d", in, want, got)
		}
	}
}
