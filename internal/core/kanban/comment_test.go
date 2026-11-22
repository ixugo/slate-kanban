package kanban_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"slate-kanban/internal/core/kanban"
)

// TestAddComment 验证人写评论：正文去掉首尾空白，记录作者类型和名称、所属看板和任务，
// 并追加 comment.added 事件；评论按写入顺序读出。
func TestAddComment(t *testing.T) {
	ctx := context.Background()
	fixed := time.Date(2026, 5, 6, 7, 8, 9, 0, time.UTC)
	e := newCardEnv(t, kanban.WithClock(func() time.Time { return fixed }))
	c := e.card(t, colTodo, "A")

	got, err := e.core.AddComment(ctx, testActor, c.ID, "  请在周五前完成\n")
	if err != nil {
		t.Fatalf("写评论失败: %v", err)
	}
	if got.ID == 0 || got.Body != "请在周五前完成" || got.Author != "admin" || got.AuthorKind != "human" ||
		got.BoardID != e.board.ID || got.CardID != c.ID || !got.CreatedAt.Equal(fixed) {
		t.Fatalf("评论的字段不对: %+v", got)
	}
	want := fmt.Sprintf(`{"author":"admin","author_kind":"human","comment_id":%d,"key":"TST-1","snippet":"请在周五前完成"}`, got.ID)
	if ev := e.lastEvent(t); ev.Type != kanban.EventCommentAdded || ev.CardID != c.ID || ev.Payload != want {
		t.Fatalf("事件不对: %+v，期望内容 %s", ev, want)
	}

	if _, err := e.core.AddComment(ctx, testActor, c.ID, "第二条"); err != nil {
		t.Fatalf("写第二条评论失败: %v", err)
	}
	list, err := e.db.Comment().ListByCard(ctx, c.ID)
	if err != nil || len(list) != 2 || list[0].Body != "请在周五前完成" || list[1].Body != "第二条" {
		t.Fatalf("评论列表不对: %+v %v", list, err)
	}
	if after := e.stored(t, c.ID); after.Version != 1 {
		t.Fatalf("写评论不应该修改任务: %+v", after)
	}
}

// TestAddCommentByAgent 验证 agent 可以评论任何任务，不需要持有；事件和评论里记录的来源是 agent。
func TestAddCommentByAgent(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	c := e.card(t, colTodo, "A")

	got, err := e.core.AddComment(ctx, testAgent, c.ID, "我看过了")
	if err != nil || got.AuthorKind != "agent" || got.Author != "claude" {
		t.Fatalf("agent 应该能评论: %+v %v", got, err)
	}
	if ev := e.lastEvent(t); ev.ActorKind != "agent" || ev.Actor != "claude" {
		t.Fatalf("事件应该记录 agent: %+v", ev)
	}
}

// TestCommentSnippet 验证事件里的摘要按字符截取：超过 200 个中文字符只留前 200 个，不会截出乱码；
// 评论正文本身保持完整。
func TestCommentSnippet(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	c := e.card(t, colTodo, "A")
	body := strings.Repeat("字", 250)

	got, err := e.core.AddComment(ctx, testActor, c.ID, body)
	if err != nil || got.Body != body {
		t.Fatalf("正文应该完整保存: %v", err)
	}
	ev := e.lastEvent(t)
	if !strings.Contains(ev.Payload, `"snippet":"`+strings.Repeat("字", 200)+`"`) || strings.Contains(ev.Payload, strings.Repeat("字", 201)) {
		t.Fatalf("摘要应该正好 200 个字符: %s", ev.Payload)
	}
}

// TestAddCommentRejects 验证拒绝情况：空白正文、过长、含空字符、非法 UTF-8、任务不存在、
// 自动化和身份无效的调用；失败时不写评论也不写事件。
func TestAddCommentRejects(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	c := e.card(t, colTodo, "A")
	events := len(e.events(t))

	for name, body := range map[string]string{
		"空正文":      "",
		"只有空白":     " \n\t ",
		"超过上限":     strings.Repeat("a", 20001),
		"含空字符":     "a\x00b",
		"非法 UTF-8": "a\xffb",
	} {
		if _, err := e.core.AddComment(ctx, testActor, c.ID, body); !errors.Is(err, kanban.ErrInvalidInput) {
			t.Errorf("%s：应该得到 ErrInvalidInput，实际 %v", name, err)
		}
	}
	if _, err := e.core.AddComment(ctx, testActor, c.ID, strings.Repeat("a", 20000)); err != nil {
		t.Errorf("刚好 20000 个字符应该成功: %v", err)
	}
	if _, err := e.core.AddComment(ctx, testActor, 9999, "x"); !errors.Is(err, kanban.ErrCardNotFound) {
		t.Errorf("任务不存在应该得到 ErrCardNotFound，实际 %v", err)
	}
	if _, err := e.core.AddComment(ctx, kanban.Actor{Kind: kanban.ActorSystem, Name: "automation"}, c.ID, "x"); !errors.Is(err, kanban.ErrHumanOnly) {
		t.Errorf("自动化应该得到 ErrHumanOnly，实际 %v", err)
	}
	if _, err := e.core.AddComment(ctx, kanban.Actor{}, c.ID, "x"); !errors.Is(err, kanban.ErrInvalidActor) {
		t.Errorf("身份无效应该得到 ErrInvalidActor，实际 %v", err)
	}
	if after := len(e.events(t)); after != events+1 { // 只有刚好 20000 个字符那条成功
		t.Fatalf("失败的调用不应该写事件，事件数 %d → %d", events, after)
	}
}

// TestDeleteComment 验证删除评论：评论没了，写 comment.deleted 事件；重复删除和删除不存在的评论都成功且不写事件。
func TestDeleteComment(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	c := e.card(t, colTodo, "A")
	keep, _ := e.core.AddComment(ctx, testActor, c.ID, "保留")
	gone, err := e.core.AddComment(ctx, testAgent, c.ID, "删除")
	if err != nil {
		t.Fatalf("准备评论失败: %v", err)
	}

	if err := e.core.DeleteComment(ctx, testActor, gone.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	list, err := e.db.Comment().ListByCard(ctx, c.ID)
	if err != nil || len(list) != 1 || list[0].ID != keep.ID {
		t.Fatalf("应该只剩保留的那条: %+v %v", list, err)
	}
	want := fmt.Sprintf(`{"comment_id":%d,"key":"TST-1"}`, gone.ID)
	ev := e.lastEvent(t)
	if ev.Type != kanban.EventCommentDeleted || ev.CardID != c.ID || ev.Payload != want {
		t.Fatalf("事件不对: %+v，期望内容 %s", ev, want)
	}

	events := len(e.events(t))
	for _, id := range []int64{gone.ID, 9999} {
		if err := e.core.DeleteComment(ctx, testActor, id); err != nil {
			t.Fatalf("删除 %d 应该成功: %v", id, err)
		}
	}
	if after := len(e.events(t)); after != events {
		t.Fatalf("幂等的删除不应该写事件，事件数 %d → %d", events, after)
	}
}

// TestDeleteCommentHumanOnly 验证只有人能删评论：agent、自动化、身份无效都被拒绝，评论保留。
func TestDeleteCommentHumanOnly(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	c := e.card(t, colTodo, "A")
	cm, err := e.core.AddComment(ctx, testAgent, c.ID, "agent 的评论")
	if err != nil {
		t.Fatalf("准备评论失败: %v", err)
	}

	for name, tc := range map[string]struct {
		actor kanban.Actor
		want  error
	}{
		"agent": {testAgent, kanban.ErrHumanOnly},
		"自动化":   {kanban.Actor{Kind: kanban.ActorSystem, Name: "automation"}, kanban.ErrHumanOnly},
		"身份无效":  {kanban.Actor{}, kanban.ErrInvalidActor},
	} {
		if err := e.core.DeleteComment(ctx, tc.actor, cm.ID); !errors.Is(err, tc.want) {
			t.Errorf("%s：期望 %v，实际 %v", name, tc.want, err)
		}
	}
	if list, _ := e.db.Comment().ListByCard(ctx, c.ID); len(list) != 1 {
		t.Fatalf("被拒绝的删除不应该删评论: %+v", list)
	}
}
