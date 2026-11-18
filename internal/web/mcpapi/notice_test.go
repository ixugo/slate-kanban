package mcpapi_test

import (
	"strings"
	"testing"
)

// noticeOf 取工具结果里的 agent_notice，没有时返回空串。
func noticeOf(data map[string]any) string {
	s, _ := data["agent_notice"].(string)
	return s
}

// TestCardToolsCarryUnclearTaskNotice 验证读取、领取任务卡片时结果里带着"任务不清楚先评论反馈"的提示，
// 并且卡片原来的字段（id、描述、评论、列指令）还在原位，没有被嵌套到别处。
func TestCardToolsCarryUnclearTaskNotice(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	c := s.card(t, b, b.doing, "写文档")
	sess := s.agent(t, "claude")

	got := ok(t, sess, "get_card", map[string]any{"card_id": c.ID}).Data
	if notice := noticeOf(got); !strings.Contains(notice, "comment_card") || !strings.Contains(notice, "不要直接开始") {
		t.Errorf("get_card 应该带提示: %v", got)
	}
	if got["id"] != float64(c.ID) || got["description"] != "描述：写文档" || got["column_instruction"] == "" {
		t.Errorf("卡片字段应该保持平铺: %v", got)
	}

	claimed := ok(t, sess, "claim_card", map[string]any{"card_id": c.ID}).Data
	if noticeOf(claimed) == "" || claimed["claimed_by"] != "claude" {
		t.Errorf("claim_card 应该带提示和领取信息: %v", claimed)
	}
}

// TestNextCardNotice 验证 next_card 领到卡片时带提示，没有可做的卡片时不带。
func TestNextCardNotice(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	s.card(t, b, b.doing, "写文档")
	sess := s.agent(t, "claude")

	got := ok(t, sess, "next_card", map[string]any{"board_id": b.ID}).Data
	if noticeOf(got) == "" {
		t.Errorf("领到卡片时应该带提示: %v", got)
	}
	empty := ok(t, s.agent(t, "other"), "next_card", map[string]any{"board_id": b.ID}).Data
	if empty["card"] != nil || noticeOf(empty) != "" {
		t.Errorf("没有卡片时 card 为 null 且没有提示: %v", empty)
	}
}

// TestAdminGetCardAlsoCarriesNotice 验证管理视图读卡片也带提示：它和 agent 视图共用同一个读取工具。
func TestAdminGetCardAlsoCarriesNotice(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	c := s.card(t, b, b.doing, "写文档")
	got := ok(t, s.human(t), "get_card", map[string]any{"card_id": c.ID}).Data
	if noticeOf(got) == "" {
		t.Errorf("管理视图读卡片也应该带提示: %v", got)
	}
}

// TestUnclearTaskReturnsToTodo 验证提示里教的做法走得通：agent 评论提问，再用 move_card 把卡片移出领取列，
// 之后 next_card 不会马上又领到同一张卡；只 release_card 的话卡片还在领取列里，会被立刻重新领到。
func TestUnclearTaskReturnsToTodo(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	c := s.card(t, b, b.doing, "需求不清楚的卡")
	sess := s.agent(t, "claude")

	ok(t, sess, "next_card", map[string]any{"board_id": b.ID})
	ok(t, sess, "comment_card", map[string]any{"card_id": c.ID, "body": "验收标准是什么？我的理解是……"})
	moved := ok(t, sess, "move_card", map[string]any{"card_id": c.ID, "column_id": b.todo}).Data
	if moved["column_id"] != float64(b.todo) {
		t.Fatalf("卡片应该回到待办: %v", moved)
	}
	if next := ok(t, sess, "next_card", map[string]any{"board_id": b.ID}).Data; next["card"] != nil {
		t.Errorf("卡片已经移出领取列，不应该再被领到: %v", next)
	}
}

// TestReleaseOnlyLoopsBack 说明为什么提示里不能只教 release_card：放手后卡片还在领取列，马上又会被领到。
func TestReleaseOnlyLoopsBack(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	c := s.card(t, b, b.doing, "需求不清楚的卡")
	sess := s.agent(t, "claude")

	ok(t, sess, "next_card", map[string]any{"board_id": b.ID})
	ok(t, sess, "release_card", map[string]any{"card_id": c.ID})
	again := ok(t, sess, "next_card", map[string]any{"board_id": b.ID}).Data
	if card, _ := again["card"].(map[string]any); card == nil || card["id"] != float64(c.ID) {
		t.Errorf("只放手的话应该又领到同一张卡: %v", again)
	}
}
