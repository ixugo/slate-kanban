package kanban_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// TestGetCardDetail 验证详情：派生状态、所在列给 agent 的指令、依赖（按依赖列表的顺序、带是否完成）、
// 评论（按时间顺序、带作者）都齐全。
func TestGetCardDetail(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	instruction := "先写失败的测试，再实现，最后在评论里汇报"
	if _, err := e.core.UpdateColumn(ctx, testActor, e.board.ID, e.col(colDoing), kanban.UpdateColumnInput{Instruction: &instruction}); err != nil {
		t.Fatalf("设置列指令失败: %v", err)
	}
	finished := e.card(t, colDone, "已完成的前置")
	pending := e.card(t, colTodo, "没完成的前置")
	main := e.card(t, colDoing, "主任务")
	deps := []int64{finished.ID, pending.ID}
	if _, err := e.core.UpdateCard(ctx, testActor, main.ID, kanban.UpdateCardInput{WaitsOn: &deps}); err != nil {
		t.Fatalf("设置依赖失败: %v", err)
	}
	e.hold(t, main.ID, testAgent.Name)
	for _, c := range []struct {
		actor kanban.Actor
		body  string
	}{{testActor, "请先看看接口文档"}, {testAgent, "收到，开始做"}} {
		if _, err := e.core.AddComment(ctx, c.actor, main.ID, c.body); err != nil {
			t.Fatalf("写评论失败: %v", err)
		}
	}

	d, err := e.core.GetCardDetail(ctx, main.ID)
	if err != nil {
		t.Fatalf("读取详情失败: %v", err)
	}
	if d.Key != "TST-3" || d.Column != "进行中" || d.Done || !d.Waiting || !d.ClaimActive || d.ColumnInstruction != instruction {
		t.Fatalf("任务与列的信息不对: %+v 指令=%q", d.CardView, d.ColumnInstruction)
	}
	if len(d.Dependencies) != 2 ||
		d.Dependencies[0].Key != "TST-1" || !d.Dependencies[0].Done || d.Dependencies[0].Column != "完成" ||
		d.Dependencies[1].Key != "TST-2" || d.Dependencies[1].Done || d.Dependencies[1].Column != "待办" {
		t.Fatalf("依赖不对: %+v", d.Dependencies)
	}
	if len(d.Comments) != 2 || d.Comments[0].Author != "admin" || d.Comments[1].Author != "claude" || d.Comments[1].AuthorKind != "agent" {
		t.Fatalf("评论不对: %+v", d.Comments)
	}
}

// TestGetCardDetailEmptyAndDangling 验证没有依赖和评论时返回空数组；依赖指向已经不存在的任务时跳过，也不算等待。
func TestGetCardDetailEmptyAndDangling(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	c := e.card(t, colTodo, "孤零零")

	d, err := e.core.GetCardDetail(ctx, c.ID)
	if err != nil || d.Dependencies == nil || d.Comments == nil || len(d.Dependencies) != 0 || len(d.Comments) != 0 || d.Waiting {
		t.Fatalf("应该是空数组且不在等待: %+v %v", d, err)
	}

	stored := e.stored(t, c.ID)
	stored.WaitsOn = []int64{9999}
	if err := e.db.Card().Save(ctx, stored); err != nil {
		t.Fatalf("制造悬空依赖失败: %v", err)
	}
	d, err = e.core.GetCardDetail(ctx, c.ID)
	if err != nil || len(d.Dependencies) != 0 || d.Waiting {
		t.Fatalf("悬空依赖应该被跳过且不算等待: %+v %v", d, err)
	}
}

// TestGetCardDetailJSON 验证序列化后的结构：任务字段、派生状态、列指令、依赖、评论都在最外层，
// 网页和 MCP 都依赖这个结构。
func TestGetCardDetailJSON(t *testing.T) {
	e := newCardEnv(t)
	c := e.card(t, colTodo, "A")

	d, err := e.core.GetCardDetail(context.Background(), c.ID)
	if err != nil {
		t.Fatalf("读取详情失败: %v", err)
	}
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	for _, key := range []string{"id", "key", "title", "version", "column", "done", "waiting", "claim_active", "column_instruction", "dependencies", "comments"} {
		if _, ok := m[key]; !ok {
			t.Errorf("JSON 最外层缺少 %s: %s", key, raw)
		}
	}
	if m["dependencies"] == nil || m["comments"] == nil {
		t.Errorf("依赖和评论应该是 [] 而不是 null: %s", raw)
	}
}

// TestGetCardDetailNotFound 验证任务不存在返回 404。
func TestGetCardDetailNotFound(t *testing.T) {
	e := newCardEnv(t)
	if _, err := e.core.GetCardDetail(context.Background(), 9999); !errors.Is(err, kanban.ErrCardNotFound) {
		t.Fatalf("应该得到 ErrCardNotFound，实际 %v", err)
	}
}
