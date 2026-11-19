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

// eventTypes 返回从第 from 条开始的事件类型，用来检查一次操作写了哪些事件、顺序对不对。
func (e *cardEnv) eventTypes(t *testing.T, from int) []string {
	t.Helper()
	var types []string
	for _, ev := range e.events(t)[from:] {
		types = append(types, ev.Type)
	}
	return types
}

// TestFinishCard 验证 agent 完成交接的完整结果：总结作为评论、分支和链接写到卡片上、
// 卡片去了下一列的末尾、领取清除、版本只涨一次，事件按评论、更新、移动的顺序且没有单独的释放事件。
func TestFinishCard(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	e.card(t, colReview, "审查列里已有的卡")
	c := e.card(t, colDoing, "任务")
	if _, err := e.core.ClaimCard(ctx, testAgent, c.ID, kanban.ClaimCardInput{}); err != nil {
		t.Fatalf("领取失败: %v", err)
	}
	events := len(e.events(t))

	d, err := e.core.FinishCard(ctx, testAgent, c.ID, kanban.FinishCardInput{
		Summary: "  已实现并通过测试  ", Branch: "feat/task", URL: "https://example.com/pr/1",
	})
	if err != nil {
		t.Fatalf("完成交接失败: %v", err)
	}
	if d.Column != "审查" || d.ClaimedBy != "" || d.ClaimActive || d.Branch != "feat/task" || d.URL != "https://example.com/pr/1" ||
		d.Version != 3 || d.Position != 2048 || d.UpdatedBy != "claude" {
		t.Fatalf("交接后的卡片不对: %+v", d.CardView.Card)
	}
	if len(d.Comments) != 1 || d.Comments[0].Body != "已实现并通过测试" || d.Comments[0].Author != "claude" {
		t.Fatalf("总结评论不对: %+v", d.Comments)
	}
	if got := strings.Join(e.eventTypes(t, events), ","); got != "comment.added,card.updated,card.moved" {
		t.Fatalf("事件的类型和顺序不对: %s", got)
	}
	want := fmt.Sprintf(`{"from":"进行中","from_id":%d,"key":"TST-2","title":"任务","to":"审查","to_id":%d}`, e.col(colDoing), e.col(colReview))
	if ev := e.lastEvent(t); ev.Type != kanban.EventCardMoved || ev.Actor != "claude" || ev.Payload != want {
		t.Fatalf("移动事件不对: %+v，期望内容 %s", ev, want)
	}
}

// TestFinishCardByHuman 验证人也能完成交接：不必持有，评论的作者是人；人完成 agent 持有的卡片会把它的领取清掉。
func TestFinishCardByHuman(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	held := e.card(t, colDoing, "agent 在做的")
	if _, err := e.core.ClaimCard(ctx, testAgent, held.ID, kanban.ClaimCardInput{}); err != nil {
		t.Fatalf("领取失败: %v", err)
	}
	free := e.card(t, colDoing, "没人领的")

	for _, id := range []int64{held.ID, free.ID} {
		d, err := e.core.FinishCard(ctx, testActor, id, kanban.FinishCardInput{Summary: "我来收尾"})
		if err != nil || d.Column != "审查" || d.ClaimedBy != "" || len(d.Comments) != 1 || d.Comments[0].AuthorKind != "human" {
			t.Fatalf("人完成交接不对: %+v %v", d, err)
		}
	}
}

// TestFinishCardTargetColumn 验证目标列：显式指定可以去任何一列，包括领活列（退回返工由列提示词约定）；
// 别的看板的列返回 400；不指定时用当前列设置的下一列。
func TestFinishCardTargetColumn(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	on, next := true, e.col(colReview)
	if _, err := e.core.UpdateColumn(ctx, testActor, e.board.ID, e.col(colReview), kanban.UpdateColumnInput{AgentPickup: &on}); err != nil {
		t.Fatalf("开启审查列领取失败: %v", err)
	}
	if _, err := e.core.UpdateColumn(ctx, testActor, e.board.ID, e.col(colDoing), kanban.UpdateColumnInput{NextColumnID: &next}); err != nil {
		t.Fatalf("设置下一列失败: %v", err)
	}
	foreign := mustCreateBoard(t, e.core, kanban.CreateBoardInput{Name: "别的看板", Slug: "OTH", Locale: "zh-CN"})
	claim := func(title string) int64 {
		c := e.card(t, colDoing, title)
		if _, err := e.core.ClaimCard(ctx, testAgent, c.ID, kanban.ClaimCardInput{}); err != nil {
			t.Fatalf("领取失败: %v", err)
		}
		return c.ID
	}
	in := func(columnID int64) kanban.FinishCardInput {
		return kanban.FinishCardInput{Summary: "做完了", ColumnID: columnID}
	}

	id := claim("去默认下一列")
	if d, err := e.core.FinishCard(ctx, testAgent, id, in(0)); err != nil || d.Column != "审查" {
		t.Fatalf("默认的下一列是领活列也应该允许: %+v %v", d, err)
	}
	id = claim("显式去完成列")
	if d, err := e.core.FinishCard(ctx, testAgent, id, in(e.col(colDone))); err != nil || d.Column != "完成" || !d.Done {
		t.Fatalf("显式指定非领活列应该成功: %+v %v", d, err)
	}
	id = claim("显式去领活列")
	if d, err := e.core.FinishCard(ctx, testAgent, id, in(e.col(colReview))); err != nil || d.Column != "审查" {
		t.Fatalf("显式指定领活列也应该允许: %+v %v", d, err)
	}
	id = claim("别的看板的列")
	if _, err := e.core.FinishCard(ctx, testAgent, id, in(foreign.Columns[0].ID)); !errors.Is(err, kanban.ErrInvalidInput) {
		t.Fatalf("别的看板的列应该得到 ErrInvalidInput，实际 %v", err)
	}
	if got := e.stored(t, id); got.ColumnID != e.col(colDoing) || got.ClaimedBy != "claude" {
		t.Fatalf("被拒绝的交接不应该改动卡片: %+v", got)
	}
}

// TestFinishCardWithoutNextColumn 验证当前列没有设置下一列也没指定目标列：返回 400，提示怎么办；指定目标列后成功。
func TestFinishCardWithoutNextColumn(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	c := e.card(t, colDone, "已经在完成列")

	_, err := e.core.FinishCard(ctx, testActor, c.ID, kanban.FinishCardInput{Summary: "收尾"})
	if !errors.Is(err, kanban.ErrInvalidInput) || !strings.Contains(err.Error(), "完成") || !strings.Contains(err.Error(), "column_id") {
		t.Fatalf("应该提示没有下一列并说明怎么办，实际 %v", err)
	}
	if list, _ := e.db.Comment().ListByCard(ctx, c.ID); len(list) != 0 {
		t.Fatalf("失败时不应该留下评论: %+v", list)
	}
	d, err := e.core.FinishCard(ctx, testActor, c.ID, kanban.FinishCardInput{Summary: "收尾", ColumnID: e.col(colReview)})
	if err != nil || d.Column != "审查" {
		t.Fatalf("指定目标列应该成功: %+v %v", d, err)
	}
}

// TestFinishCardInSameColumn 验证目标列就是当前列：卡片不动，领取清除，因为没有 card.moved，所以补一条 card.released；
// 总结评论照常写入。
func TestFinishCardInSameColumn(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	c := e.card(t, colReview, "在审查列")
	e.holdUntil(t, c.ID, "claude", claimNow.Add(time.Hour))
	events := len(e.events(t))

	d, err := e.core.FinishCard(ctx, testActor, c.ID, kanban.FinishCardInput{Summary: "审查完", ColumnID: e.col(colReview)})
	if err != nil || d.Column != "审查" || d.ClaimedBy != "" {
		t.Fatalf("原地交接不对: %+v %v", d, err)
	}
	if got := strings.Join(e.eventTypes(t, events), ","); got != "comment.added,card.released" {
		t.Fatalf("事件的类型和顺序不对: %s", got)
	}
}

// TestFinishCardRollsBack 验证原子性：分支或链接不合法时整个交接回滚，
// 总结评论没有写入，卡片还在原来的列并且仍被持有，也没有任何事件。
func TestFinishCardRollsBack(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	c := e.card(t, colDoing, "任务")
	if _, err := e.core.ClaimCard(ctx, testAgent, c.ID, kanban.ClaimCardInput{}); err != nil {
		t.Fatalf("领取失败: %v", err)
	}
	events := len(e.events(t))

	for name, in := range map[string]kanban.FinishCardInput{
		"链接不合法": {Summary: "总结", URL: "ftp://example.com"},
		"分支名过长": {Summary: "总结", Branch: strings.Repeat("b", 201)},
	} {
		if _, err := e.core.FinishCard(ctx, testAgent, c.ID, in); !errors.Is(err, kanban.ErrInvalidInput) {
			t.Errorf("%s：应该得到 ErrInvalidInput，实际 %v", name, err)
		}
	}
	if list, _ := e.db.Comment().ListByCard(ctx, c.ID); len(list) != 0 {
		t.Fatalf("回滚后不应该留下评论: %+v", list)
	}
	if got := e.stored(t, c.ID); got.ColumnID != e.col(colDoing) || got.ClaimedBy != "claude" || got.Version != 2 || got.Branch != "" {
		t.Fatalf("回滚后卡片应该原样: %+v", got)
	}
	if after := len(e.events(t)); after != events {
		t.Fatalf("回滚后不应该有事件，事件数 %d → %d", events, after)
	}
}

// TestFinishCardRejects 验证拒绝情况：总结为空或过长、不是持有者的 agent、卡片不存在、自动化、身份无效。
func TestFinishCardRejects(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	mine := e.card(t, colDoing, "我的")
	if _, err := e.core.ClaimCard(ctx, testAgent, mine.ID, kanban.ClaimCardInput{}); err != nil {
		t.Fatalf("领取失败: %v", err)
	}
	free := e.card(t, colDoing, "没人领")
	ok := kanban.FinishCardInput{Summary: "总结"}

	cases := []struct {
		name  string
		actor kanban.Actor
		id    int64
		in    kanban.FinishCardInput
		want  error
	}{
		{"总结为空", testAgent, mine.ID, kanban.FinishCardInput{}, kanban.ErrInvalidInput},
		{"总结只有空白", testAgent, mine.ID, kanban.FinishCardInput{Summary: " \n "}, kanban.ErrInvalidInput},
		{"总结过长", testAgent, mine.ID, kanban.FinishCardInput{Summary: strings.Repeat("a", 20001)}, kanban.ErrInvalidInput},
		{"别的 agent", codex, mine.ID, ok, kanban.ErrNotHolder},
		{"agent 完成没人领的卡", testAgent, free.ID, ok, kanban.ErrNotHolder},
		{"卡片不存在", testAgent, 9999, ok, kanban.ErrCardNotFound},
		{"自动化", kanban.Actor{Kind: kanban.ActorSystem, Name: "automation"}, mine.ID, ok, kanban.ErrHumanOnly},
		{"身份无效", kanban.Actor{}, mine.ID, ok, kanban.ErrInvalidActor},
	}
	events := len(e.events(t))
	for _, c := range cases {
		if _, err := e.core.FinishCard(ctx, c.actor, c.id, c.in); !errors.Is(err, c.want) {
			t.Errorf("%s：期望 %v，实际 %v", c.name, c.want, err)
		}
	}
	if after := len(e.events(t)); after != events {
		t.Errorf("失败的交接不应该写事件，事件数 %d → %d", events, after)
	}
	if got := e.stored(t, mine.ID); got.ColumnID != e.col(colDoing) || got.ClaimedBy != "claude" {
		t.Errorf("失败的交接不应该改动卡片: %+v", got)
	}
}
