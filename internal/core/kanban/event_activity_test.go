package kanban_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// codexActor 是测试里的 agent。
var codexActor = kanban.Actor{Kind: kanban.ActorAgent, Name: "codex"}

// activityEnv 造一份有三类操作者的活动：人建卡和移动，agent 评论，规则自动写开始时间。
// 返回环境，以及它产生的全部事件（按发生顺序）。
func activityEnv(t *testing.T) (*cardEnv, []*kanban.Event) {
	t.Helper()
	e := newCardEnv(t)
	e.enterRule(t, "开始计时", e.col(colDoing), startedAction)
	c := e.card(t, colTodo, "A")
	if _, err := e.core.AddComment(context.Background(), codexActor, c.ID, "收到"); err != nil {
		t.Fatalf("agent 评论失败: %v", err)
	}
	e.moveTo(t, testActor, c.ID, colDoing)
	return e, e.events(t)
}

// activityIDs 取出一页活动的事件序号。
func activityIDs(page *kanban.ActivityPage) []int64 {
	return eventSeqs(page.Events)
}

// reversed 返回倒序的副本。
func reversed(seqs []int64) []int64 {
	out := slices.Clone(seqs)
	slices.Reverse(out)
	return out
}

// TestListActivityPaging 验证翻页：从新到旧，用上一页最后一条的序号翻下一页，
// 每页满额，最后一页 has_more 为假，所有页拼起来恰好是全部事件的倒序，没有重复和遗漏。
func TestListActivityPaging(t *testing.T) {
	ctx := context.Background()
	e, all := activityEnv(t)

	var got []int64
	before := int64(0)
	pages := 0
	for ; ; pages++ {
		if pages > len(all) {
			t.Fatal("翻页没有结束")
		}
		page, err := e.core.ListActivity(ctx, e.board.ID, kanban.ActivityQuery{Before: before, Limit: 2})
		if err != nil {
			t.Fatalf("读活动流失败: %v", err)
		}
		if page.Latest != all[len(all)-1].ID {
			t.Fatalf("最新序号不对: %d", page.Latest)
		}
		got = append(got, activityIDs(page)...)
		if !page.HasMore {
			break
		}
		if len(page.Events) != 2 {
			t.Fatalf("还有更多时这一页应该是满的: %d", len(page.Events))
		}
		before = page.Events[len(page.Events)-1].ID
	}
	if !slices.Equal(got, reversed(eventSeqs(all))) {
		t.Fatalf("所有页拼起来应该是全部事件的倒序: %v vs %v", got, reversed(eventSeqs(all)))
	}
	// 最后一页刚好装满时不能再多翻一页空的：页数必须是总数除以页大小向上取整。
	if want := (len(all) + 1) / 2; pages+1 != want {
		t.Fatalf("应该翻 %d 页，实际 %d 页", want, pages+1)
	}
}

// TestListActivityHasMoreBoundary 验证 has_more 的边界：剩下的事件刚好等于页大小时为假，多一条时为真。
func TestListActivityHasMoreBoundary(t *testing.T) {
	ctx := context.Background()
	e, all := activityEnv(t)
	exact, err := e.core.ListActivity(ctx, e.board.ID, kanban.ActivityQuery{Limit: len(all)})
	if err != nil || exact.HasMore || len(exact.Events) != len(all) {
		t.Fatalf("页大小刚好等于总数时不应该还有更多: %+v %v", exact, err)
	}
	short, err := e.core.ListActivity(ctx, e.board.ID, kanban.ActivityQuery{Limit: len(all) - 1})
	if err != nil || !short.HasMore || len(short.Events) != len(all)-1 {
		t.Fatalf("少取一条时应该还有更多: %+v %v", short, err)
	}
}

// TestListActivityFilters 验证筛选：按类型、按操作者类型、按操作者名称，以及叠加；
// 筛选发生在翻页之前，所以每页都是满的；最新序号和操作者列表不受筛选影响。
func TestListActivityFilters(t *testing.T) {
	ctx := context.Background()
	e, all := activityEnv(t)
	byType := func(typ string) []int64 {
		var ids []int64
		for _, ev := range all {
			if ev.Type == typ {
				ids = append(ids, ev.ID)
			}
		}
		return reversed(ids)
	}
	read := func(f kanban.ActivityFilter, limit int) *kanban.ActivityPage {
		t.Helper()
		page, err := e.core.ListActivity(ctx, e.board.ID, kanban.ActivityQuery{Limit: limit, ActivityFilter: f})
		if err != nil {
			t.Fatalf("读活动流失败: %v", err)
		}
		return page
	}

	moved := read(kanban.ActivityFilter{Types: []string{kanban.EventCardMoved}}, 0)
	if !slices.Equal(activityIDs(moved), byType(kanban.EventCardMoved)) || len(moved.Events) != 1 {
		t.Fatalf("按类型筛选不对: %v", activityIDs(moved))
	}
	fired := read(kanban.ActivityFilter{ActorKind: kanban.ActorSystem}, 0)
	if !slices.Equal(activityIDs(fired), byType(kanban.EventAutomationFired)) || len(fired.Events) != 1 {
		t.Fatalf("按操作者类型筛选不对: %v", activityIDs(fired))
	}
	agent := read(kanban.ActivityFilter{ActorKind: kanban.ActorAgent, Actor: "codex"}, 0)
	if !slices.Equal(activityIDs(agent), byType(kanban.EventCommentAdded)) {
		t.Fatalf("按 agent 筛选不对: %v", activityIDs(agent))
	}
	if none := read(kanban.ActivityFilter{Types: []string{kanban.EventCommentAdded}, ActorKind: kanban.ActorHuman}, 0); len(none.Events) != 0 || none.Events == nil || none.HasMore {
		t.Fatalf("没有交集应该返回空数组: %+v", none)
	}

	// 筛选后只有 1 条人的事件之外的另一条，limit=1 时第一页要有更多，翻到第二页才结束。
	humanOnly := read(kanban.ActivityFilter{ActorKind: kanban.ActorHuman}, 1)
	if len(humanOnly.Events) != 1 || !humanOnly.HasMore {
		t.Fatalf("筛选后的第一页应该是满的而且还有更多: %+v", humanOnly)
	}
	next, err := e.core.ListActivity(ctx, e.board.ID, kanban.ActivityQuery{Before: humanOnly.Events[0].ID, Limit: 100, ActivityFilter: kanban.ActivityFilter{ActorKind: kanban.ActorHuman}})
	if err != nil || next.HasMore || len(next.Events) == 0 {
		t.Fatalf("翻到下一页应该拿到剩下的人的事件: %+v %v", next, err)
	}
	for _, ev := range append(humanOnly.Events, next.Events...) {
		if ev.ActorKind != string(kanban.ActorHuman) {
			t.Fatalf("混进了别的操作者的事件: %+v", ev)
		}
	}
	if moved.Latest != all[len(all)-1].ID {
		t.Fatalf("最新序号不应该受筛选影响: %d", moved.Latest)
	}
}

// TestListActivityByCard 验证按卡片筛选：只返回这张卡片的事件（建卡、评论、移动），别的卡片和与卡片无关的事件都不出现。
func TestListActivityByCard(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	a := e.card(t, colTodo, "A")
	b := e.card(t, colTodo, "B")
	if _, err := e.core.AddComment(ctx, codexActor, a.ID, "给 A 的留言"); err != nil {
		t.Fatalf("评论失败: %v", err)
	}
	e.moveTo(t, testActor, a.ID, colDoing)

	page, err := e.core.ListActivity(ctx, e.board.ID, kanban.ActivityQuery{ActivityFilter: kanban.ActivityFilter{CardID: a.ID}})
	if err != nil {
		t.Fatalf("读活动流失败: %v", err)
	}
	if len(page.Events) < 3 {
		t.Fatalf("A 至少有建卡、评论、移动三条事件: %+v", page.Events)
	}
	for _, ev := range page.Events {
		if ev.CardID != a.ID {
			t.Fatalf("混进了别的卡片的事件: %+v", ev)
		}
	}
	other, err := e.core.ListActivity(ctx, e.board.ID, kanban.ActivityQuery{ActivityFilter: kanban.ActivityFilter{CardID: b.ID, Types: []string{kanban.EventCommentAdded}}})
	if err != nil || len(other.Events) != 0 {
		t.Fatalf("B 没有评论，叠加类型后应该是空: %+v %v", other, err)
	}
}

// TestListActivityActors 验证操作者列表：人、agent、规则各出现一次，不受筛选条件影响。
func TestListActivityActors(t *testing.T) {
	ctx := context.Background()
	e, _ := activityEnv(t)
	page, err := e.core.ListActivity(ctx, e.board.ID, kanban.ActivityQuery{ActivityFilter: kanban.ActivityFilter{Actor: "codex"}})
	if err != nil {
		t.Fatalf("读活动流失败: %v", err)
	}
	want := []kanban.ActivityActor{
		{Kind: kanban.ActorAgent, Name: "codex"},
		{Kind: kanban.ActorHuman, Name: testActor.Name},
		{Kind: kanban.ActorSystem, Name: "automation:开始计时"},
	}
	if !slices.Equal(page.Actors, want) {
		t.Fatalf("操作者列表不对: %v", page.Actors)
	}
}

// TestListActivityDuplicateTypes 验证类型列表里重复多少次都不会撑爆查询：
// 重复项会被合并，结果和只写一次完全一样。4 万次远超 SQLite 的单条语句参数上限，没合并就会报错。
func TestListActivityDuplicateTypes(t *testing.T) {
	ctx := context.Background()
	e, _ := activityEnv(t)
	once, err := e.core.ListActivity(ctx, e.board.ID, kanban.ActivityQuery{ActivityFilter: kanban.ActivityFilter{Types: []string{kanban.EventCardMoved}}})
	if err != nil || len(once.Events) == 0 {
		t.Fatalf("读活动流失败: %+v %v", once, err)
	}
	many := slices.Repeat([]string{kanban.EventCardMoved}, 40_000)
	repeated, err := e.core.ListActivity(ctx, e.board.ID, kanban.ActivityQuery{ActivityFilter: kanban.ActivityFilter{Types: many}})
	if err != nil || !slices.Equal(activityIDs(once), activityIDs(repeated)) {
		t.Fatalf("重复类型应该和写一次一样: %v %v", activityIDs(repeated), err)
	}
}

// TestListActivityInvalid 验证参数错误一律返回 400，看板不存在返回 404；
// 不认识的类型要报错，而不是悄悄返回一个永远为空的列表。
func TestListActivityInvalid(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	bad := map[string]kanban.ActivityQuery{
		"负数 before":   {Before: -1},
		"负数 limit":    {Limit: -1},
		"负数卡片编号":      {ActivityFilter: kanban.ActivityFilter{CardID: -1}},
		"不认识的类型":      {ActivityFilter: kanban.ActivityFilter{Types: []string{"card.exploded"}}},
		"空字符串类型":      {ActivityFilter: kanban.ActivityFilter{Types: []string{""}}},
		"不认识的操作者类型":   {ActivityFilter: kanban.ActivityFilter{ActorKind: "robot"}},
		"操作者名称过长":     {ActivityFilter: kanban.ActivityFilter{Actor: strings.Repeat("长", 65)}},
		"类型里夹带 SQL":   {ActivityFilter: kanban.ActivityFilter{Types: []string{"card.moved') OR 1=1 --"}}},
		"操作者类型夹带 SQL": {ActivityFilter: kanban.ActivityFilter{ActorKind: "human' OR '1'='1"}},
	}
	for name, q := range bad {
		if _, err := e.core.ListActivity(ctx, e.board.ID, q); !errors.Is(err, kanban.ErrInvalidInput) {
			t.Errorf("%s 应该返回 400: %v", name, err)
		}
	}
	if _, err := e.core.ListActivity(ctx, 9999, kanban.ActivityQuery{}); !errors.Is(err, kanban.ErrBoardNotFound) {
		t.Errorf("看板不存在应该返回 404: %v", err)
	}
	if _, err := e.core.ListActivity(ctx, e.board.ID, kanban.ActivityQuery{Limit: 10_000}); err != nil {
		t.Errorf("limit 超过上限应该被压到上限而不是报错: %v", err)
	}
}

// TestListActivityActorNameBoundary 验证操作者名称刚好等于上限时可以用，名称里的空白会被去掉。
func TestListActivityActorNameBoundary(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	if _, err := e.core.ListActivity(ctx, e.board.ID, kanban.ActivityQuery{ActivityFilter: kanban.ActivityFilter{Actor: strings.Repeat("长", 64)}}); err != nil {
		t.Errorf("刚好 64 个字符应该可以: %v", err)
	}
	e.card(t, colTodo, "A")
	page, err := e.core.ListActivity(ctx, e.board.ID, kanban.ActivityQuery{ActivityFilter: kanban.ActivityFilter{Actor: "  " + testActor.Name + "  "}})
	if err != nil || len(page.Events) == 0 {
		t.Errorf("名称两边的空白应该被去掉: %+v %v", page, err)
	}
}
