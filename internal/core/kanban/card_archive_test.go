package kanban_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"slate-kanban/internal/core/kanban"
)

func TestArchiveFreezesTaskAndKeepsDependencies(t *testing.T) {
	ctx := t.Context()
	e := newClaimEnv(t)
	source := e.card(t, colDoing, "归档任务")
	if _, err := e.core.ClaimCard(ctx, codex, source.ID, kanban.ClaimCardInput{}); err != nil {
		t.Fatal(err)
	}
	comment, err := e.core.AddComment(ctx, testActor, source.ID, "保留此评论")
	if err != nil {
		t.Fatal(err)
	}
	waiting, err := e.core.CreateCard(ctx, testActor, e.board.ID, kanban.CreateCardInput{
		Title: "依赖归档任务", WaitsOn: []int64{source.ID},
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := e.core.ArchiveCard(ctx, testActor, source.ID)
	if err != nil || got.ArchivedAt == nil || got.ClaimedBy != "" || got.ClaimExpiresAt != nil || got.ColumnID != source.ColumnID {
		t.Fatalf("归档状态不对: %+v %v", got, err)
	}
	count := len(e.events(t))
	again, err := e.core.ArchiveCard(ctx, testActor, source.ID)
	if err != nil || again.Version != got.Version || len(e.events(t)) != count {
		t.Fatalf("重复归档写入了变化: %+v %v", again, err)
	}
	_, updateErr := e.core.UpdateCard(ctx, testActor, source.ID, kanban.UpdateCardInput{Title: new("不能修改")})
	_, claimErr := e.core.ClaimCard(ctx, codex, source.ID, kanban.ClaimCardInput{})
	_, addErr := e.core.AddComment(ctx, codex, source.ID, "不能写评论")
	_, moveErr := e.core.MoveCard(ctx, testActor, source.ID, kanban.MoveCardInput{ColumnID: e.col(colDone)})
	_, finishErr := e.core.FinishCard(ctx, testActor, source.ID, kanban.FinishCardInput{Summary: "不能交接"})
	for name, err := range map[string]error{
		"update": updateErr, "claim": claimErr, "comment": addErr, "move": moveErr, "finish": finishErr,
		"delete":         e.core.DeleteCard(ctx, testActor, source.ID),
		"delete comment": e.core.DeleteComment(ctx, testActor, comment.ID),
	} {
		if !errors.Is(err, kanban.ErrCardArchived) {
			t.Errorf("%s: %v", name, err)
		}
	}
	next, err := e.core.NextCard(ctx, codex, e.board.ID, kanban.NextCardInput{ColumnID: e.col(colDoing)})
	if err != nil || next != nil {
		t.Fatalf("不应领取到归档任务: %+v %v", next, err)
	}
	snapshot, err := e.core.BoardSnapshot(ctx, e.board.ID)
	if err != nil || len(snapshot.Cards) != 1 || snapshot.Cards[0].ID != waiting.ID || !snapshot.Cards[0].Waiting {
		t.Fatalf("快照或依赖状态不对: %+v %v", snapshot, err)
	}
	detail, err := e.core.GetCardDetailByNumber(ctx, e.board.ID, source.Number)
	if err != nil || detail.ID != source.ID || len(detail.Comments) != 1 || detail.Comments[0].ID != comment.ID {
		t.Fatalf("应能读取归档任务及评论: %+v %v", detail, err)
	}
}

func TestArchivedCardsPageUsesArchiveOrder(t *testing.T) {
	ctx := t.Context()
	e := newClaimEnv(t)
	for i := range 21 {
		card := e.card(t, colIdea, fmt.Sprintf("归档 %d", i))
		if _, err := e.core.ArchiveCard(ctx, testActor, card.ID); err != nil {
			t.Fatal(err)
		}
	}
	active := e.card(t, colTodo, "活跃任务")
	first, err := e.core.ListArchivedCards(ctx, e.board.ID, 0)
	if err != nil || !first.HasMore || len(first.Cards) != 20 {
		t.Fatalf("第一页不对: %+v %v", first, err)
	}
	for i, card := range first.Cards {
		if card.Number != int64(21-i) {
			t.Fatalf("同一归档时间应按任务主键倒序: %+v", card)
		}
	}
	last, err := e.core.ListArchivedCards(ctx, e.board.ID, first.Cards[19].ID)
	if err != nil || last.HasMore || len(last.Cards) != 1 || last.Cards[0].Number != 1 {
		t.Fatalf("最后一页不对: %+v %v", last, err)
	}
	if _, err := e.core.ListArchivedCards(ctx, e.board.ID, active.ID); !errors.Is(err, kanban.ErrCardNotFound) {
		t.Fatalf("普通任务不应作为归档游标: %v", err)
	}
}

func TestForkCopiesOnlyContinuationFields(t *testing.T) {
	ctx := t.Context()
	e := newCardEnv(t)
	dep := e.card(t, colDone, "已完成的依赖")
	source, err := e.core.CreateCard(ctx, testActor, e.board.ID, kanban.CreateCardInput{
		ColumnID: e.col(colDoing), Title: "继续此任务", Description: "原描述", Priority: 3,
		TagIDs: []int64{e.board.Tags[0].ID}, WaitsOn: []int64{dep.ID},
		URL: "https://example.com/task", HumanOnly: true, Branch: "source",
		DueAt: "2026-11-24T08:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.core.AddComment(ctx, testActor, source.ID, "原评论"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.core.ArchiveCard(ctx, testActor, source.ID); err != nil {
		t.Fatal(err)
	}
	e.mustCreateRule(t, kanban.CreateRuleInput{
		Name: "移走新建任务", Trigger: kanban.TriggerCardEntered, ColumnID: e.col(colIdea),
		Actions: []kanban.RuleAction{{Type: kanban.ActionMoveToColumn, ColumnID: e.col(colTodo)}},
	})
	got, err := e.core.ForkCard(ctx, testActor, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID == source.ID || got.ColumnID != e.col(colIdea) || got.Title != source.Title || got.Priority != source.Priority ||
		got.HumanOnly != source.HumanOnly || got.URL != source.URL || !slices.Equal(got.TagIDs, source.TagIDs) || !slices.Equal(got.WaitsOn, source.WaitsOn) ||
		got.Description != fmt.Sprintf("接着任务 #%d 继续", source.Number) || got.Branch != "" || got.DueAt != nil || got.ArchivedAt != nil {
		t.Fatalf("Fork 的字段不对: %+v", got)
	}
	detail, err := e.core.GetCardDetail(ctx, got.ID)
	if err != nil || len(detail.Comments) != 0 {
		t.Fatalf("Fork 不应复制评论: %+v %v", detail, err)
	}
	if _, err := e.core.UpdateColumn(ctx, testActor, e.board.ID, e.col(colIdea), kanban.UpdateColumnInput{
		Name: new("其他列"), Instruction: new("不作为收件箱"),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.core.ForkCard(ctx, testActor, source.ID); !errors.Is(err, kanban.ErrInboxNotFound) {
		t.Fatalf("不应放到无收件箱看板的任意列: %v", err)
	}
}

func TestArchiveRetainsDeletedRelations(t *testing.T) {
	ctx := t.Context()
	e := newClaimEnv(t)
	tag := e.board.Tags[0]
	dep := e.card(t, colDone, "依赖原名")
	makeCard := func(title string) *kanban.Card {
		t.Helper()
		card, err := e.core.CreateCard(ctx, testActor, e.board.ID, kanban.CreateCardInput{
			ColumnID: e.col(colDoing), Title: title, TagIDs: []int64{tag.ID}, WaitsOn: []int64{dep.ID},
		})
		if err != nil {
			t.Fatal(err)
		}
		return card
	}
	target := makeCard("归档任务")
	active := makeCard("活跃任务")
	tagOnly, err := e.core.CreateCard(ctx, testActor, e.board.ID, kanban.CreateCardInput{Title: "仅有标签", TagIDs: []int64{tag.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.core.ArchiveCard(ctx, testActor, tagOnly.ID); err != nil {
		t.Fatal(err)
	}
	archived, err := e.core.ArchiveCard(ctx, testActor, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.core.UpdateBoard(ctx, testActor, e.board.ID, kanban.UpdateBoardInput{
		Tags: new([]kanban.TagInput{{ID: tag.ID, Text: "已更名标签", Color: "red-3"}}),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.core.UpdateCard(ctx, testActor, dep.ID, kanban.UpdateCardInput{Title: new("依赖现名")}); err != nil {
		t.Fatal(err)
	}
	detail, err := e.core.GetCardDetail(ctx, target.ID)
	if err != nil || len(detail.Dependencies) != 1 || detail.Dependencies[0].Title != dep.Title || detail.Dependencies[0].Missing {
		t.Fatalf("重命名应保留历史依赖文案: %+v %v", detail, err)
	}
	if _, err := e.core.ForkCard(ctx, testActor, target.ID); err != nil {
		t.Fatalf("关联存在时仍可复制：%v", err)
	}
	if err := e.core.DeleteCard(ctx, testActor, dep.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.core.ForkCard(ctx, testActor, target.ID); !errors.Is(err, kanban.ErrForkRelationUnavailable) {
		t.Fatalf("缺少依赖时应拒绝不完整的 Fork：%v", err)
	}
	if _, err := e.core.UpdateBoard(ctx, testActor, e.board.ID, kanban.UpdateBoardInput{Tags: new([]kanban.TagInput{})}); err != nil {
		t.Fatal(err)
	}

	detail, err = e.core.GetCardDetail(ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Version != archived.Version || !detail.UpdatedAt.Equal(archived.UpdatedAt) ||
		!slices.Equal(detail.TagIDs, []int64{tag.ID}) || !slices.Equal(detail.WaitsOn, []int64{dep.ID}) {
		t.Fatalf("归档内容或版本被改写：%+v", detail.Card)
	}
	if len(detail.ArchivedTags) != 1 || detail.ArchivedTags[0].Text != tag.Text || detail.ArchivedTags[0].Color != tag.Color {
		t.Fatalf("归档标签不完整：%+v", detail.ArchivedTags)
	}
	if len(detail.Dependencies) != 1 || detail.Dependencies[0].Key != dep.Key ||
		detail.Dependencies[0].Title != dep.Title || !detail.Dependencies[0].Done || !detail.Dependencies[0].Missing {
		t.Fatalf("删除不应清空历史依赖：%+v", detail.Dependencies)
	}
	updated, err := e.core.GetCard(ctx, active.ID)
	if err != nil || len(updated.TagIDs) != 0 || len(updated.WaitsOn) != 0 {
		t.Fatalf("活跃任务应继续正常清理：%+v %v", updated, err)
	}
	if _, err := e.core.ForkCard(ctx, testActor, tagOnly.ID); !errors.Is(err, kanban.ErrForkRelationUnavailable) {
		t.Fatalf("缺少标签时应拒绝不完整的 Fork：%v", err)
	}
	inbox, err := e.core.ListCards(ctx, e.board.ID, kanban.ListCardsInput{Query: "col:收件箱"})
	if err != nil || inbox.Total != 1 {
		t.Fatalf("只应写入唯一一次成功的 Fork：%+v %v", inbox, err)
	}
}

func makeCardWithLegacyRelations(t *testing.T, e *cardEnv, tagIDs, waitsOn []int64) *kanban.Card {
	t.Helper()
	card := e.card(t, colDoing, "旧归档")
	card.ArchivedAt = new(claimNow)
	card.TagIDs, card.WaitsOn = tagIDs, waitsOn
	if err := e.db.Card().Save(t.Context(), card); err != nil {
		t.Fatal(err)
	}
	return card
}

func TestLegacyArchiveUsesSurvivingRelations(t *testing.T) {
	ctx := t.Context()
	e := newClaimEnv(t)
	tag := e.board.Tags[0]
	dep := e.card(t, colDone, "旧归档仍可读取此依赖")
	card := makeCardWithLegacyRelations(t, e, []int64{tag.ID}, []int64{dep.ID})
	detail, err := e.core.GetCardDetail(ctx, card.ID)
	if err != nil || len(detail.ArchivedTags) != 1 || detail.ArchivedTags[0].ID != tag.ID ||
		len(detail.Dependencies) != 1 || detail.Dependencies[0].ID != dep.ID {
		t.Fatalf("无快照的旧数据应回退到仍存在的关联：%+v %v", detail, err)
	}
}
