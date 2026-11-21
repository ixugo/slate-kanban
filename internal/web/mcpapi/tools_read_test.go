package mcpapi_test

import (
	"fmt"
	"strings"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// TestListBoards 验证看板列表：两个视图都能读，按创建顺序返回，带标识和名称。
func TestListBoards(t *testing.T) {
	s := newStack(t)
	s.seed(t, "ALPHA")
	s.seed(t, "BETA")
	for name, sess := range map[string]func() []map[string]any{
		"agent": func() []map[string]any {
			return items(t, ok(t, s.agent(t, "claude"), "list_boards", nil).Data, "boards")
		},
		"人": func() []map[string]any { return items(t, ok(t, s.human(t), "list_boards", nil).Data, "boards") },
	} {
		boards := sess()
		if len(boards) != 2 || boards[0]["slug"] != "ALPHA" || boards[1]["slug"] != "BETA" || boards[0]["name"] != "看板 ALPHA" {
			t.Errorf("%s：看板列表不对: %v", name, boards)
		}
	}
}

// TestListBoardsEmpty 验证没有看板时返回空数组而不是 null，agent 不用处理 null。
func TestListBoardsEmpty(t *testing.T) {
	s := newStack(t)
	if got := items(t, ok(t, s.agent(t, "claude"), "list_boards", nil).Data, "boards"); len(got) != 0 {
		t.Errorf("没有看板应该返回空数组: %v", got)
	}
}

// TestGetBoard 验证看板详情：列带着说明和给 agent 的指令、领取列标记，规则也在里面；看板不存在是工具错误。
func TestGetBoard(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	sess := s.agent(t, "claude")
	got := ok(t, sess, "get_board", map[string]any{"board_id": b.ID}).Data
	cols := items(t, got, "columns")
	if len(cols) != 5 || len(items(t, got, "rules")) == 0 {
		t.Fatalf("应该有 5 列和默认规则: %v", got)
	}
	doing := cols[2]
	if doing["name"] != "进行中" || !strings.Contains(fmt.Sprint(doing["instruction"]), "finish_card") {
		t.Errorf("进行中列应该是 agent 领取列并带着指令: %v", doing)
	}
	prompt := fmt.Sprint(got["watch_prompt"])
	if !strings.Contains(prompt, "「看板 SLATE」") || strings.Contains(prompt, "%s") || !strings.Contains(prompt, "slate-kanban skill") {
		t.Errorf("watch_prompt 应该带着填好的看板名并指向 skill: %q", prompt)
	}
	missing := call(t, sess, "get_board", map[string]any{"board_id": 9999})
	if !missing.IsError || !strings.HasPrefix(missing.Text, "ErrBoardNotFound:") {
		t.Errorf("看板不存在应该是工具错误: %+v", missing)
	}
}

// TestListCardsSummaries 验证卡片列表只给摘要：没有描述和评论，有编号、所在列、派生状态；
// 同时支持筛选语法，写错的筛选词是带说明的工具错误。
func TestListCardsSummaries(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	s.card(t, b, b.doing, "做登录")
	s.card(t, b, b.todo, "写文档")
	sess := s.agent(t, "claude")

	all := ok(t, sess, "list_cards", map[string]any{"board_id": b.ID}).Data
	cards := items(t, all, "cards")
	if len(cards) != 2 || all["total"] != float64(2) || all["truncated"] != false {
		t.Fatalf("应该有两张卡片: %v", all)
	}
	first := cards[0] // 待办列在进行中列的左边，所以先出现的是"写文档"
	for _, banned := range []string{"description", "comments"} {
		if _, has := first[banned]; has {
			t.Errorf("摘要里不应该有 %s: %v", banned, first)
		}
	}
	if first["title"] != "写文档" || first["key"] != "SLATE-2" || first["column"] != "待办" || first["done"] != false {
		t.Errorf("摘要字段不对: %v", first)
	}

	doing := items(t, ok(t, sess, "list_cards", map[string]any{"board_id": b.ID, "query": "col:进行中"}).Data, "cards")
	if len(doing) != 1 || doing[0]["title"] != "做登录" {
		t.Errorf("按列筛选的结果不对: %v", doing)
	}
	bad := call(t, sess, "list_cards", map[string]any{"board_id": b.ID, "query": "is:foo"})
	if !bad.IsError || !strings.Contains(bad.Text, "is:foo") {
		t.Errorf("写错的筛选词应该是带说明的工具错误: %+v", bad)
	}
}

// TestListCardsLimit 验证数量限制：不指定时默认 10 张、最多 30 张，指定了按指定的，并如实告诉调用方被截断了。
func TestListCardsLimit(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	for i := range 55 {
		s.card(t, b, b.todo, fmt.Sprintf("卡片 %d", i))
	}
	sess := s.agent(t, "claude")
	def := ok(t, sess, "list_cards", map[string]any{"board_id": b.ID}).Data
	if len(items(t, def, "cards")) != 10 || def["total"] != float64(55) || def["truncated"] != true {
		t.Errorf("默认应该返回 10 张并标记被截断: total=%v truncated=%v", def["total"], def["truncated"])
	}
	most := ok(t, sess, "list_cards", map[string]any{"board_id": b.ID, "limit": 100}).Data
	if len(items(t, most, "cards")) != 30 || most["truncated"] != true {
		t.Errorf("limit 超过上限应该只返回 30 张: %v", most["truncated"])
	}
	few := ok(t, sess, "list_cards", map[string]any{"board_id": b.ID, "limit": 3}).Data
	if len(items(t, few, "cards")) != 3 || few["truncated"] != true {
		t.Errorf("limit 为 3 应该返回 3 张: %v", few["truncated"])
	}
	if neg := call(t, sess, "list_cards", map[string]any{"board_id": b.ID, "limit": -1}); !neg.IsError {
		t.Errorf("负数的 limit 应该是工具错误: %+v", neg)
	}
}

// TestGetCard 验证卡片详情：全文、评论、所在列的指令、依赖的卡片都在里面；卡片不存在是工具错误。
func TestGetCard(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	dep := s.card(t, b, b.todo, "先做这个")
	card, err := s.core.CreateCard(t.Context(), humanActor, b.ID, kanban.CreateCardInput{
		Title: "做登录", Description: "需要支持扫码", ColumnID: b.doing, WaitsOn: []int64{dep.ID},
	})
	if err != nil {
		t.Fatalf("创建卡片失败: %v", err)
	}
	if _, err := s.core.AddComment(t.Context(), humanActor, card.ID, "注意兼容旧版"); err != nil {
		t.Fatalf("写评论失败: %v", err)
	}
	sess := s.agent(t, "claude")
	got := ok(t, sess, "get_card", map[string]any{"card_id": card.ID}).Data
	if got["description"] != "需要支持扫码" || got["waiting"] != true || !strings.Contains(fmt.Sprint(got["column_instruction"]), "finish_card") {
		t.Errorf("详情字段不对: %v", got)
	}
	comments, deps := items(t, got, "comments"), items(t, got, "dependencies")
	if len(comments) != 1 || comments[0]["body"] != "注意兼容旧版" || len(deps) != 1 || deps[0]["title"] != "先做这个" {
		t.Errorf("评论或依赖不对: %v %v", comments, deps)
	}
	missing := call(t, sess, "get_card", map[string]any{"card_id": 9999})
	if !missing.IsError || !strings.HasPrefix(missing.Text, "ErrCardNotFound:") {
		t.Errorf("卡片不存在应该是工具错误: %+v", missing)
	}
}

// TestListEvents 验证活动流水只在管理视图里：返回起点之后的全部事件、最新序号。
func TestListEvents(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	s.card(t, b, b.todo, "写文档")
	got := ok(t, s.human(t), "list_events", map[string]any{"board_id": b.ID}).Data
	events := items(t, got, "events")
	if len(events) < 2 || events[len(events)-1]["type"] != "card.created" || got["latest"] != events[len(events)-1]["id"] {
		t.Errorf("事件列表不对: %v", got)
	}
	after := ok(t, s.human(t), "list_events", map[string]any{"board_id": b.ID, "after": got["latest"]}).Data
	if len(items(t, after, "events")) != 0 {
		t.Errorf("起点是最新序号时不应该再有事件: %v", after)
	}
}
