package mcpapi_test

import (
	"strings"
	"testing"
)

func TestFinishReviewedMerge(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	c := s.card(t, b, b.review, "已审查的改动")
	reviewer := s.agent(t, "codex")
	other := s.agent(t, "cursor")
	admin := s.human(t)
	id := map[string]any{"card_id": c.ID}

	ok(t, reviewer, "claim_card", map[string]any{"card_id": c.ID, "engineer_id": b.Engineers[2].ID})
	updated := ok(t, reviewer, "update_card", map[string]any{"card_id": c.ID, "branch": "slate/" + strings.ToLower(c.Key) + "-reviewed"}).Data
	input := map[string]any{
		"card_id": c.ID, "summary": "审查通过", "version": updated["version"],
		"merged_commits": []string{"abc123", "def456"},
	}

	fails(t, call(t, other, "finish_card", input), "ErrNotHolder")
	fails(t, call(t, admin, "finish_card", input), "ErrAgentOnly")
	input["column_id"] = b.doing
	fails(t, call(t, reviewer, "finish_card", input), "ErrInvalidInput")
	delete(input, "column_id")
	input["merged_commits"] = []string{"abc1234"}
	fails(t, call(t, reviewer, "finish_card", input), "ErrInvalidInput")
	input["merged_commits"] = []string{"abc123", "def456"}

	input["version"] = 1
	fails(t, call(t, reviewer, "finish_card", input), "ErrCardVersionConflict")
	input["version"] = updated["version"]
	done := ok(t, reviewer, "finish_card", input).Data
	if done["done"] != true || done["claimed_by"] != "" {
		t.Fatalf("合并记录应随完成流转保存: %v", done)
	}
	full := ok(t, admin, "get_card", id).Data
	hashes, _ := full["merged_commits"].([]any)
	if len(hashes) != 2 || hashes[0] != "abc123" || hashes[1] != "def456" {
		t.Fatalf("人视图应读到合并提交列表: %v", full)
	}
	comments := items(t, full, "comments")
	if len(comments) != 1 || !strings.Contains(comments[0]["body"].(string), "`abc123`, `def456`") {
		t.Fatalf("合并提交应写进审查结论，失败请求不留评论: %v", comments)
	}
	ok(t, admin, "move_card", map[string]any{"card_id": c.ID, "column_id": b.doing})
	ok(t, admin, "move_card", map[string]any{"card_id": c.ID, "column_id": b.done})
	if got := ok(t, admin, "get_card", id).Data; got["merged_commits"] != nil {
		t.Fatalf("返工再手动完成不能沿用旧合并标记: %v", got)
	}
}
