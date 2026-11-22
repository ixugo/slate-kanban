package kanbandb_test

import (
	"context"
	"sync"
	"testing"

	"github.com/ixugo/goddd/pkg/orm"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/kanbantest"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
)

// addEvents 给看板连续追加 n 条事件，返回它们的序号。
func addEvents(t *testing.T, store kanban.EventStorer, boardID int64, n int) []int64 {
	t.Helper()
	seqs := make([]int64, 0, n)
	for range n {
		e := &kanban.Event{BoardID: boardID, Type: "card.created", ActorKind: "human", Actor: "admin", Payload: "{}"}
		if err := store.Create(context.Background(), e); err != nil {
			t.Fatalf("追加事件失败: %v", err)
		}
		seqs = append(seqs, e.ID)
	}
	return seqs
}

// TestEventSeqIsIncreasing 验证事件序号单调递增，并且最新序号和最小序号读得正确；
// 没有事件的看板两个值都是 0。
func TestEventSeqIsIncreasing(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Event()

		if latest, err := store.LatestSeq(ctx, 1); err != nil || latest != 0 {
			t.Fatalf("没有事件时最新序号应该是 0，实际 %d, %v", latest, err)
		}
		if min, err := store.MinSeq(ctx, 1); err != nil || min != 0 {
			t.Fatalf("没有事件时最小序号应该是 0，实际 %d, %v", min, err)
		}

		seqs := addEvents(t, store, 1, 3)
		addEvents(t, store, 2, 2)
		for i := 1; i < len(seqs); i++ {
			if seqs[i] <= seqs[i-1] {
				t.Fatalf("序号必须递增: %v", seqs)
			}
		}
		if latest, _ := store.LatestSeq(ctx, 1); latest != seqs[2] {
			t.Fatalf("最新序号应该是 %d，实际 %d", seqs[2], latest)
		}
		if min, _ := store.MinSeq(ctx, 1); min != seqs[0] {
			t.Fatalf("最小序号应该是 %d，实际 %d", seqs[0], min)
		}
	})
}

// TestEventListAfter 验证只返回本看板里序号更大的事件，升序排列，并且遵守条数上限。
func TestEventListAfter(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Event()
		seqs := addEvents(t, store, 1, 5)
		addEvents(t, store, 2, 3)

		items, err := store.ListAfter(ctx, 1, seqs[1], 10)
		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		if len(items) != 3 || items[0].ID != seqs[2] || items[2].ID != seqs[4] {
			t.Fatalf("应该返回序号 %v 之后的 3 条，实际 %d 条: %+v", seqs[1], len(items), items)
		}

		limited, _ := store.ListAfter(ctx, 1, 0, 2)
		if len(limited) != 2 || limited[0].ID != seqs[0] {
			t.Fatalf("条数上限没有生效: %+v", limited)
		}
	})
}

// TestEventListByCard 验证按任务查询，最新的在前，并遵守条数上限。
func TestEventListByCard(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Event()
		for range 3 {
			if err := store.Create(ctx, &kanban.Event{BoardID: 1, CardID: 7, Type: "card.updated", Payload: "{}"}); err != nil {
				t.Fatalf("追加事件失败: %v", err)
			}
		}
		_ = store.Create(ctx, &kanban.Event{BoardID: 1, CardID: 8, Type: "card.updated", Payload: "{}"})

		items, err := store.ListByCard(ctx, 7, 2)
		if err != nil || len(items) != 2 {
			t.Fatalf("应该返回 2 条，实际 %d, %v", len(items), err)
		}
		if items[0].ID <= items[1].ID {
			t.Fatalf("应该是最新的在前: %d, %d", items[0].ID, items[1].ID)
		}
	})
}

// TestEventPrune 验证清理只保留最新的 keep 条，不动别的看板，
// 事件不足 keep 条时什么都不删。
func TestEventPrune(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Event()
		seqs := addEvents(t, store, 1, 6)
		addEvents(t, store, 2, 6)

		if err := store.Prune(ctx, 1, 10); err != nil {
			t.Fatalf("清理失败: %v", err)
		}
		if min, _ := store.MinSeq(ctx, 1); min != seqs[0] {
			t.Fatalf("事件不足保留数量时不应该删除，最小序号 %d", min)
		}

		if err := store.Prune(ctx, 1, 4); err != nil {
			t.Fatalf("清理失败: %v", err)
		}
		if min, _ := store.MinSeq(ctx, 1); min != seqs[2] {
			t.Fatalf("应该保留最新 4 条，最小序号应为 %d，实际 %d", seqs[2], min)
		}
		if latest, _ := store.LatestSeq(ctx, 1); latest != seqs[5] {
			t.Fatalf("最新序号不应该变化，实际 %d", latest)
		}
		other, _ := store.ListAfter(ctx, 2, 0, 100)
		if len(other) != 6 {
			t.Fatalf("别的看板的事件不应该被清理，剩余 %d", len(other))
		}
	})
}

// TestEventDeleteByBoard 验证按看板删除只影响本看板。
func TestEventDeleteByBoard(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Event()
		addEvents(t, store, 1, 3)
		addEvents(t, store, 2, 2)
		if err := store.DeleteByBoard(ctx, 1); err != nil {
			t.Fatalf("删除失败: %v", err)
		}
		if left, _ := store.ListAfter(ctx, 1, 0, 100); len(left) != 0 {
			t.Fatalf("看板 1 的事件应该全部删除，剩余 %d", len(left))
		}
		if left, _ := store.ListAfter(ctx, 2, 0, 100); len(left) != 2 {
			t.Fatalf("看板 2 的事件不应该被删除，剩余 %d", len(left))
		}
	})
}

// TestCursorAdvanceNeverMovesBack 验证游标只增不减：新建、推进、用更小的值调用都符合预期。
func TestCursorAdvanceNeverMovesBack(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Cursor()

		if _, err := store.Get(ctx, 1, "claude"); !orm.IsErrRecordNotFound(err) {
			t.Fatalf("没有记录时应该是找不到，实际: %v", err)
		}
		steps := []struct{ call, want int64 }{
			{5, 5},   // 新建
			{9, 9},   // 推进
			{3, 9},   // 更小的值不会让进度倒退
			{9, 9},   // 相同的值保持不变
			{12, 12}, // 再次推进
		}
		for _, s := range steps {
			got, err := store.Advance(ctx, 1, "claude", s.call)
			if err != nil || got != s.want {
				t.Fatalf("Advance(%d) 应该得到 %d，实际 %d, %v", s.call, s.want, got, err)
			}
		}
	})
}

// TestCursorIsPerBoardAndAgent 验证游标按（看板，agent）各存一份，互不影响。
func TestCursorIsPerBoardAndAgent(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Cursor()
		_, _ = store.Advance(ctx, 1, "claude", 10)
		_, _ = store.Advance(ctx, 1, "cursor", 4)
		_, _ = store.Advance(ctx, 2, "claude", 7)

		for _, c := range []struct {
			board int64
			agent string
			want  int64
		}{{1, "claude", 10}, {1, "cursor", 4}, {2, "claude", 7}} {
			got, err := store.Get(ctx, c.board, c.agent)
			if err != nil || got.LastSeq != c.want {
				t.Fatalf("(%d,%s) 应该是 %d，实际 %+v %v", c.board, c.agent, c.want, got, err)
			}
		}

		if err := store.DeleteByBoard(ctx, 1); err != nil {
			t.Fatalf("删除失败: %v", err)
		}
		if _, err := store.Get(ctx, 1, "claude"); !orm.IsErrRecordNotFound(err) {
			t.Fatalf("看板 1 的游标应该已删除，实际: %v", err)
		}
		if _, err := store.Get(ctx, 2, "claude"); err != nil {
			t.Fatalf("看板 2 的游标不应该被删除: %v", err)
		}
	})
}

// TestCursorAdvanceConcurrent 验证并发推进时，最终结果是所有调用里最大的值。
func TestCursorAdvanceConcurrent(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Cursor()
		var wg sync.WaitGroup
		for i := int64(1); i <= 20; i++ {
			wg.Go(func() {
				if _, err := store.Advance(ctx, 1, "claude", i); err != nil {
					t.Errorf("Advance(%d) 失败: %v", i, err)
				}
			})
		}
		wg.Wait()
		got, err := store.Get(ctx, 1, "claude")
		if err != nil || got.LastSeq != 20 {
			t.Fatalf("并发推进后应该是最大值 20，实际 %+v %v", got, err)
		}
	})
}

// addEventAs 给看板追加一条指定类型和操作者的事件，返回它的序号。
func addEventAs(t *testing.T, store kanban.EventStorer, boardID int64, typ string, kind kanban.ActorKind, actor string) int64 {
	t.Helper()
	e := &kanban.Event{BoardID: boardID, Type: typ, ActorKind: string(kind), Actor: actor, Payload: "{}"}
	if err := store.Create(context.Background(), e); err != nil {
		t.Fatalf("追加事件失败: %v", err)
	}
	return e.ID
}

// addCardEvent 给指定任务追加一条事件，返回序号。
func addCardEvent(t *testing.T, store kanban.EventStorer, boardID, cardID int64, typ string) int64 {
	t.Helper()
	e := &kanban.Event{BoardID: boardID, CardID: cardID, Type: typ, ActorKind: string(kanban.ActorHuman), Actor: "human", Payload: "{}"}
	if err := store.Create(context.Background(), e); err != nil {
		t.Fatalf("追加事件失败: %v", err)
	}
	return e.ID
}

// eventIDs 取出一组事件的序号。
func eventIDs(items []*kanban.Event) []int64 {
	ids := make([]int64, len(items))
	for i, e := range items {
		ids[i] = e.ID
	}
	return ids
}

// TestEventListBefore 验证活动流的底层查询：只看本看板，序号严格小于 beforeSeq，从新到旧，
// 遵守条数上限；类型、操作者类型、操作者名称三个条件可以单独用也可以叠加（都满足才算）。
func TestEventListBefore(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Event()
		a := addEventAs(t, store, 1, kanban.EventCardCreated, kanban.ActorHuman, "human")
		b := addEventAs(t, store, 1, kanban.EventCardMoved, kanban.ActorHuman, "human")
		addEventAs(t, store, 2, kanban.EventCardMoved, kanban.ActorHuman, "human") // 别的看板，不能出现
		c := addEventAs(t, store, 1, kanban.EventCommentAdded, kanban.ActorAgent, "codex")
		d := addEventAs(t, store, 1, kanban.EventCardMoved, kanban.ActorAgent, "claude")
		e := addEventAs(t, store, 1, kanban.EventAutomationFired, kanban.ActorSystem, "automation:开始计时")

		list := func(before int64, f kanban.ActivityFilter, limit int) []int64 {
			t.Helper()
			items, err := store.ListBefore(ctx, 1, before, f, limit)
			if err != nil || items == nil {
				t.Fatalf("查询失败或返回了 nil: %v", err)
			}
			return eventIDs(items)
		}
		equal := func(name string, got, want []int64) {
			t.Helper()
			if len(got) != len(want) {
				t.Fatalf("%s: 期望 %v，实际 %v", name, want, got)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("%s: 期望 %v，实际 %v", name, want, got)
				}
			}
		}

		equal("不限条件从最新开始", list(0, kanban.ActivityFilter{}, 10), []int64{e, d, c, b, a})
		equal("条数上限", list(0, kanban.ActivityFilter{}, 2), []int64{e, d})
		equal("序号严格小于 before", list(c, kanban.ActivityFilter{}, 10), []int64{b, a})
		equal("按类型", list(0, kanban.ActivityFilter{Types: []string{kanban.EventCardMoved}}, 10), []int64{d, b})
		equal("多个类型取并集", list(0, kanban.ActivityFilter{Types: []string{kanban.EventCardCreated, kanban.EventCommentAdded}}, 10), []int64{c, a})
		equal("按操作者类型", list(0, kanban.ActivityFilter{ActorKind: kanban.ActorAgent}, 10), []int64{d, c})
		equal("按操作者名称", list(0, kanban.ActivityFilter{Actor: "codex"}, 10), []int64{c})
		equal("三个条件叠加", list(0, kanban.ActivityFilter{Types: []string{kanban.EventCardMoved}, ActorKind: kanban.ActorAgent, Actor: "claude"}, 10), []int64{d})
		equal("类型和操作者没有交集", list(0, kanban.ActivityFilter{Types: []string{kanban.EventCardCreated}, ActorKind: kanban.ActorAgent}, 10), []int64{})
		equal("筛选加翻页", list(d, kanban.ActivityFilter{Types: []string{kanban.EventCardMoved}}, 10), []int64{b})
	})
}

// TestEventListBeforeByCard 验证按任务筛选：只留下这个任务的事件，能和类型叠加、能翻页，0 表示不限。
func TestEventListBeforeByCard(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Event()
		a := addCardEvent(t, store, 1, 7, kanban.EventCardCreated)
		addCardEvent(t, store, 1, 8, kanban.EventCardCreated) // 别的任务
		b := addCardEvent(t, store, 1, 7, kanban.EventCommentAdded)
		addCardEvent(t, store, 2, 7, kanban.EventCommentAdded) // 别的看板，任务编号即使相同也不能出现
		c := addCardEvent(t, store, 1, 7, kanban.EventCardMoved)
		addEventAs(t, store, 1, kanban.EventBoardUpdated, kanban.ActorHuman, "human") // 与任务无关

		ids := func(before int64, f kanban.ActivityFilter, limit int) []int64 {
			t.Helper()
			items, err := store.ListBefore(ctx, 1, before, f, limit)
			if err != nil {
				t.Fatalf("查询失败: %v", err)
			}
			return eventIDs(items)
		}
		same := func(name string, got, want []int64) {
			t.Helper()
			if len(got) != len(want) {
				t.Fatalf("%s: 期望 %v，实际 %v", name, want, got)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("%s: 期望 %v，实际 %v", name, want, got)
				}
			}
		}
		same("只看这个任务", ids(0, kanban.ActivityFilter{CardID: 7}, 10), []int64{c, b, a})
		same("和类型叠加", ids(0, kanban.ActivityFilter{CardID: 7, Types: []string{kanban.EventCommentAdded}}, 10), []int64{b})
		same("筛选加翻页", ids(c, kanban.ActivityFilter{CardID: 7}, 10), []int64{b, a})
		same("没有这个任务的事件", ids(0, kanban.ActivityFilter{CardID: 99}, 10), []int64{})
		if all := ids(0, kanban.ActivityFilter{}, 20); len(all) != 5 {
			t.Fatalf("不限任务时应该看到本看板全部 5 条事件: %v", all)
		}
	})
}

// TestEventActors 验证操作者列表：同一个人只出现一次，按类型再按名称排序，只含本看板，没有事件时是空数组。
func TestEventActors(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Event()
		if got, err := store.Actors(ctx, 1); err != nil || got == nil || len(got) != 0 {
			t.Fatalf("没有事件时应该是空数组: %v %v", got, err)
		}
		addEventAs(t, store, 1, kanban.EventCardCreated, kanban.ActorHuman, "human")
		addEventAs(t, store, 1, kanban.EventCardMoved, kanban.ActorHuman, "human")
		addEventAs(t, store, 1, kanban.EventCommentAdded, kanban.ActorAgent, "codex")
		addEventAs(t, store, 1, kanban.EventCommentAdded, kanban.ActorAgent, "claude")
		addEventAs(t, store, 1, kanban.EventAutomationFired, kanban.ActorSystem, "automation:开始计时")
		addEventAs(t, store, 2, kanban.EventCommentAdded, kanban.ActorAgent, "别的看板的")

		got, err := store.Actors(ctx, 1)
		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		want := []kanban.ActivityActor{
			{Kind: kanban.ActorAgent, Name: "claude"},
			{Kind: kanban.ActorAgent, Name: "codex"},
			{Kind: kanban.ActorHuman, Name: "human"},
			{Kind: kanban.ActorSystem, Name: "automation:开始计时"},
		}
		if len(got) != len(want) {
			t.Fatalf("期望 %v，实际 %v", want, got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("期望 %v，实际 %v", want, got)
			}
		}
	})
}
