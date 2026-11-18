package mcpapi_test

import (
	"strings"
	"testing"
)

// titles 按位置顺序返回某一列的卡片标题，用逗号连接。
func (s *stack) titles(t *testing.T, boardID, column int64) string {
	t.Helper()
	snap, err := s.core.BoardSnapshot(t.Context(), boardID)
	if err != nil {
		t.Fatalf("读取看板快照失败: %v", err)
	}
	var names []string
	for _, c := range snap.Cards {
		if c.ColumnID == column {
			names = append(names, c.Title)
		}
	}
	return strings.Join(names, ",")
}

// TestAgentPlacementDefaultsToTop 验证 agent 通过 MCP 创建、移动、交接卡片时默认放到目标列最顶部，
// 带 before_id / after_id 时以参数为准。
func TestAgentPlacementDefaultsToTop(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "TOP")
	s.card(t, b, b.review, "旧一")
	s.card(t, b, b.review, "旧二")
	mine := s.card(t, b, b.doing, "我的")
	sess := s.agent(t, "claude")

	created := ok(t, sess, "create_card", map[string]any{"board_id": b.ID, "title": "新建", "column_id": b.todo})
	if created.Data["column_id"] != float64(b.todo) {
		t.Fatalf("新建的卡片应该在待办列: %v", created.Data)
	}
	ok(t, sess, "next_card", map[string]any{"board_id": b.ID})
	ok(t, sess, "finish_card", map[string]any{"card_id": mine.ID, "summary": "做完了"})
	if got := s.titles(t, b.ID, b.review); got != "我的,旧一,旧二" {
		t.Fatalf("交接应该放到审查列最顶部，实际: %s", got)
	}
	if got := s.titles(t, b.ID, b.todo); got != "新建" {
		t.Fatalf("创建的卡片应该放到列最顶部，实际: %s", got)
	}
}

// TestAgentFinishWithPosition 验证交接时用 after_id 指定位置，不再默认放顶部。
func TestAgentFinishWithPosition(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "POS")
	first := s.card(t, b, b.review, "旧一")
	s.card(t, b, b.review, "旧二")
	s.card(t, b, b.doing, "我的")
	sess := s.agent(t, "claude")

	next := ok(t, sess, "next_card", map[string]any{"board_id": b.ID}).Data
	id := next["card"].(map[string]any)["id"]
	ok(t, sess, "finish_card", map[string]any{"card_id": id, "summary": "做完了", "after_id": first.ID})
	if got := s.titles(t, b.ID, b.review); !strings.HasPrefix(got, "旧一,我的") {
		t.Fatalf("应该放在旧一后面，实际: %s", got)
	}
}
