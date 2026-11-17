package kanban_test

import (
	"context"
	"errors"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// TestExportBoardComplete 验证导出含列、规则、卡片视图字段、评论和最新序号。
func TestExportBoardComplete(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	c := e.card(t, colTodo, "导出卡")
	e.mustCreateRule(t, newRuleInput("导出测试规则", e.col(colDoing)))
	if _, err := e.core.AddComment(ctx, testActor, c.ID, "人的备注"); err != nil {
		t.Fatalf("写评论失败: %v", err)
	}
	if _, err := e.core.AddComment(ctx, testAgent, c.ID, "agent 跟进"); err != nil {
		t.Fatalf("写评论失败: %v", err)
	}
	exp, err := e.core.ExportBoard(ctx, testActor, e.board.ID)
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	if exp.Seq != e.lastEvent(t).ID || len(exp.Rules) == 0 || len(exp.Columns) != 5 {
		t.Fatalf("序号、规则、列不对: seq=%d rules=%d", exp.Seq, len(exp.Rules))
	}
	if len(exp.Cards) != 1 {
		t.Fatalf("应有 1 张卡: %d", len(exp.Cards))
	}
	got := exp.Cards[0]
	if got.Column != e.board.Columns[colTodo].Name {
		t.Fatalf("列名派生不对: %q", got.Column)
	}
	if len(got.Comments) != 2 || got.Comments[0].Body != "人的备注" {
		t.Fatalf("评论未挂上: %+v", got.Comments)
	}
}

// TestExportBoardEmpty 验证空看板导出：卡片是空切片，评论查询仍成功。
func TestExportBoardEmpty(t *testing.T) {
	e := newCardEnv(t)
	exp, err := e.core.ExportBoard(context.Background(), testActor, e.board.ID)
	if err != nil || exp.Cards == nil || len(exp.Cards) != 0 {
		t.Fatalf("空看板: %+v %v", exp, err)
	}
}

// TestExportBoardMissing 验证看板不存在返回 404。
func TestExportBoardMissing(t *testing.T) {
	e := newCardEnv(t)
	if _, err := e.core.ExportBoard(context.Background(), testActor, 9999); !errors.Is(err, kanban.ErrBoardNotFound) {
		t.Fatalf("应 404: %v", err)
	}
}

// TestExportBoardCommentsBatch 验证多张卡的评论都能一次性带出。
func TestExportBoardCommentsBatch(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	a := e.card(t, colTodo, "甲")
	b := e.card(t, colTodo, "乙")
	if _, err := e.core.AddComment(ctx, testActor, a.ID, "A"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.core.AddComment(ctx, testActor, b.ID, "B"); err != nil {
		t.Fatal(err)
	}
	exp, err := e.core.ExportBoard(ctx, testActor, e.board.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := map[int64]int{}
	for _, c := range exp.Cards {
		found[c.ID] = len(c.Comments)
	}
	if found[a.ID] != 1 || found[b.ID] != 1 {
		t.Fatalf("每张卡应各有一条评论: %+v", found)
	}
}
