package kanban_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// eventSeqs 返回一组事件的序号。
func eventSeqs(events []*kanban.Event) []int64 {
	seqs := make([]int64, len(events))
	for i, ev := range events {
		seqs[i] = ev.ID
	}
	return seqs
}

// TestListEvents 验证补发：只返回序号大于 afterSeq 的事件，按序号升序，只包含本看板的，
// 带着最新序号；limit 截断时 has_more 为真。
func TestListEvents(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	other := newCardEnvOn(t, e.core, e.db, "OTH")
	e.card(t, colTodo, "A")
	other.card(t, colTodo, "别的看板的")
	e.card(t, colTodo, "B")
	all := e.events(t)

	page, err := e.core.ListEvents(ctx, e.board.ID, 0, 0)
	if err != nil || !slices.Equal(eventSeqs(page.Events), eventSeqs(all)) {
		t.Fatalf("应该返回本看板的全部事件: %+v %v", page, err)
	}
	if page.Latest != all[len(all)-1].ID || page.HasMore || page.Truncated {
		t.Fatalf("最新序号、更多、截断标记不对: %+v", page)
	}

	after, err := e.core.ListEvents(ctx, e.board.ID, all[len(all)-2].ID, 0)
	if err != nil || len(after.Events) != 1 || after.Events[0].ID != all[len(all)-1].ID {
		t.Fatalf("只应该返回 afterSeq 之后的事件: %+v %v", after, err)
	}
	limited, err := e.core.ListEvents(ctx, e.board.ID, 0, 2)
	if err != nil || len(limited.Events) != 2 || !limited.HasMore {
		t.Fatalf("limit 截断时 has_more 应该为真: %+v %v", limited, err)
	}
	empty, err := e.core.ListEvents(ctx, e.board.ID, page.Latest, 0)
	if err != nil || empty.Events == nil || len(empty.Events) != 0 {
		t.Fatalf("没有新事件应该返回空数组: %+v %v", empty, err)
	}
}

// TestListEventsInvalid 验证参数错误：负数的 afterSeq 和 limit 返回 400，看板不存在返回 404。
func TestListEventsInvalid(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	if _, err := e.core.ListEvents(ctx, e.board.ID, -1, 0); !errors.Is(err, kanban.ErrInvalidInput) {
		t.Errorf("负数 afterSeq 应该返回 400: %v", err)
	}
	if _, err := e.core.ListEvents(ctx, e.board.ID, 0, -1); !errors.Is(err, kanban.ErrInvalidInput) {
		t.Errorf("负数 limit 应该返回 400: %v", err)
	}
	if _, err := e.core.ListEvents(ctx, 9999, 0, 0); !errors.Is(err, kanban.ErrBoardNotFound) {
		t.Errorf("看板不存在应该返回 404: %v", err)
	}
}

// TestListEventsTruncated 验证保留范围之前的起点：事件被清理后，从很早的序号读会标记 truncated，
// 并且从保留范围的开头读起；起点刚好在保留范围前一个序号时没有缺口，不标记。
func TestListEventsTruncated(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t, kanban.WithRetentionForTest(5, 1))
	for i := range 8 {
		e.card(t, colTodo, string(rune('A'+i)))
	}
	kept := e.events(t)
	if len(kept) != 5 {
		t.Fatalf("前提不成立：应该只保留 5 条，实际 %d", len(kept))
	}

	page, err := e.core.ListEvents(ctx, e.board.ID, 0, 0)
	if err != nil || !page.Truncated || !slices.Equal(eventSeqs(page.Events), eventSeqs(kept)) {
		t.Fatalf("从很早的序号读应该标记 truncated 并返回保留的全部: %+v %v", page, err)
	}
	edge, err := e.core.ListEvents(ctx, e.board.ID, kept[0].ID-1, 0)
	if err != nil || edge.Truncated || len(edge.Events) != 5 {
		t.Fatalf("起点刚好在保留范围前一个序号，没有缺口: %+v %v", edge, err)
	}
}

// TestLatestSeq 验证最新序号：等于补发接口返回的最新序号，新增事件之后变大，
// 不受别的看板的事件影响；看板不存在返回 404。
func TestLatestSeq(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	other := newCardEnvOn(t, e.core, e.db, "OTH")
	page, err := e.core.ListEvents(ctx, e.board.ID, 0, 0)
	if err != nil {
		t.Fatalf("读取事件失败: %v", err)
	}
	before, err := e.core.LatestSeq(ctx, e.board.ID)
	if err != nil || before != page.Latest || before == 0 {
		t.Fatalf("应该等于补发接口的最新序号 %d: %d %v", page.Latest, before, err)
	}

	other.card(t, colTodo, "别的看板的")
	if same, err := e.core.LatestSeq(ctx, e.board.ID); err != nil || same != before {
		t.Fatalf("别的看板的事件不应该影响本看板: %d %v", same, err)
	}
	e.card(t, colTodo, "A")
	if after, err := e.core.LatestSeq(ctx, e.board.ID); err != nil || after <= before {
		t.Fatalf("新增事件之后序号应该变大: %d %v", after, err)
	}
	if _, err := e.core.LatestSeq(ctx, e.board.ID+9999); !errors.Is(err, kanban.ErrBoardNotFound) {
		t.Errorf("看板不存在应该返回 404: %v", err)
	}
}
