package kanban_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
)

// newEmptyBoard 创建一个没有任何列的看板，方便精确控制列的内容。
func newEmptyBoard(t *testing.T) (kanban.Core, kanbandb.DB, *kanban.BoardDetail) {
	t.Helper()
	core, db := newBare(t)
	d := mustCreateBoard(t, core, kanban.CreateBoardInput{Name: "空看板", Slug: "EMPTY", Template: kanban.TemplateEmpty})
	return core, db, d
}

// mustCreateColumn 以人的身份新增列，失败就让测试终止。
func mustCreateColumn(t *testing.T, core kanban.Core, boardID int64, name string) *kanban.Column {
	t.Helper()
	col, err := core.CreateColumn(context.Background(), testActor, boardID, kanban.CreateColumnInput{Name: name})
	if err != nil {
		t.Fatalf("新增列 %s 失败: %v", name, err)
	}
	return col
}

// boardColumns 读出看板当前的列。
func boardColumns(t *testing.T, core kanban.Core, boardID int64) []*kanban.Column {
	t.Helper()
	d, err := core.GetBoard(context.Background(), boardID)
	if err != nil {
		t.Fatalf("读取看板失败: %v", err)
	}
	return d.Columns
}

// TestCreateColumn 验证新增列：加在最右边，排序值是最大值加 1024，字段都写入，
// 并且追加 column.created 事件。
func TestCreateColumn(t *testing.T) {
	ctx := context.Background()
	core, db, d := newEmptyBoard(t)

	first, err := core.CreateColumn(ctx, testActor, d.ID, kanban.CreateColumnInput{
		Name: "  待办  ", Instruction: "指令",
	})
	if err != nil {
		t.Fatalf("新增失败: %v", err)
	}
	if first.Name != "待办" || first.Instruction != "指令" ||
		first.IsDone || first.BoardID != d.ID || first.Sort != 1024 {
		t.Fatalf("第一列的字段不对: %+v", first)
	}
	second, err := core.CreateColumn(ctx, testActor, d.ID, kanban.CreateColumnInput{
		Name: "完成", IsDone: true, NextColumnID: first.ID,
	})
	if err != nil || second.Sort != 2048 || second.NextColumnID != first.ID || !second.IsDone {
		t.Fatalf("第二列的字段不对: %+v %v", second, err)
	}

	got := boardColumns(t, core, d.ID)
	if len(got) != 2 || got[0].ID != first.ID || got[1].ID != second.ID {
		t.Fatalf("读出来的顺序不对: %+v", got)
	}
	events := boardEvents(t, db, d.ID)
	last := events[len(events)-1]
	want := fmt.Sprintf(`{"column_id":%d,"name":"完成"}`, second.ID)
	if last.Type != kanban.EventColumnCreated || last.Payload != want {
		t.Fatalf("事件不对: %+v，期望内容 %s", last, want)
	}
}

// TestCreateColumnRejects 验证新增列的各种拒绝：同名（不区分大小写）409、下一列不合法 400、
// 输入不合法 400、agent 403、看板不存在 404；失败都不会留下列。
func TestCreateColumnRejects(t *testing.T) {
	ctx := context.Background()
	core, _, d := newEmptyBoard(t)
	other, _ := core.CreateBoard(ctx, testActor, kanban.CreateBoardInput{Name: "别的", Slug: "OTHER", Template: kanban.TemplateSimple})
	mustCreateColumn(t, core, d.ID, "Todo")

	cases := []struct {
		name  string
		actor kanban.Actor
		board int64
		in    kanban.CreateColumnInput
		want  error
	}{
		{"同名", testActor, d.ID, kanban.CreateColumnInput{Name: "Todo"}, kanban.ErrColumnNameTaken},
		{"同名不区分大小写", testActor, d.ID, kanban.CreateColumnInput{Name: " tODO "}, kanban.ErrColumnNameTaken},
		{"下一列不存在", testActor, d.ID, kanban.CreateColumnInput{Name: "A", NextColumnID: 9999}, kanban.ErrInvalidInput},
		{"下一列属于别的看板", testActor, d.ID, kanban.CreateColumnInput{Name: "A", NextColumnID: other.Columns[0].ID}, kanban.ErrInvalidInput},
		{"名称为空", testActor, d.ID, kanban.CreateColumnInput{Name: "  "}, kanban.ErrInvalidInput},
		{"名称有换行", testActor, d.ID, kanban.CreateColumnInput{Name: "a\nb"}, kanban.ErrInvalidInput},
		{"名称太长", testActor, d.ID, kanban.CreateColumnInput{Name: strings.Repeat("长", 61)}, kanban.ErrInvalidInput},
		{"指令太长", testActor, d.ID, kanban.CreateColumnInput{Name: "A", Instruction: strings.Repeat("长", 4001)}, kanban.ErrInvalidInput},
		{"agent", testAgent, d.ID, kanban.CreateColumnInput{Name: "A"}, kanban.ErrHumanOnly},
		{"看板不存在", testActor, 9999, kanban.CreateColumnInput{Name: "A"}, kanban.ErrBoardNotFound},
	}
	for _, c := range cases {
		if _, err := core.CreateColumn(ctx, c.actor, c.board, c.in); !errors.Is(err, c.want) {
			t.Errorf("%s：应该得到 %v，实际 %v", c.name, c.want, err)
		}
	}
	if got := boardColumns(t, core, d.ID); len(got) != 1 {
		t.Fatalf("失败不应该留下列，实际 %d 列", len(got))
	}
}

// TestCreateColumnLimit 验证列数上限：并发新增 35 列，恰好 30 个成功，其余返回 ErrColumnLimit，
// 成功的列排序值互不相同。
func TestCreateColumnLimit(t *testing.T) {
	core, _, d := newEmptyBoard(t)
	const total = 35
	var mu sync.Mutex
	ok, limited := 0, 0
	within(t, 30*time.Second, func() {
		var wg sync.WaitGroup
		for i := range total {
			wg.Go(func() {
				_, err := core.CreateColumn(context.Background(), testActor, d.ID, kanban.CreateColumnInput{Name: fmt.Sprintf("列%d", i)})
				mu.Lock()
				defer mu.Unlock()
				switch {
				case err == nil:
					ok++
				case errors.Is(err, kanban.ErrColumnLimit):
					limited++
				default:
					t.Errorf("意外的错误: %v", err)
				}
			})
		}
		wg.Wait()
	})
	if ok != 30 || limited != total-30 {
		t.Fatalf("应该 30 个成功、%d 个被限制，实际 %d、%d", total-30, ok, limited)
	}
	sorts := map[int64]bool{}
	for _, col := range boardColumns(t, core, d.ID) {
		if sorts[col.Sort] {
			t.Fatalf("排序值重复: %d", col.Sort)
		}
		sorts[col.Sort] = true
	}
}

// TestUpdateColumn 验证修改列：只更新出现的字段，事件里记录变化的字段名；
// 下一列可以设置也可以用 0 清除；只改大小写不会撞上自己的名字。
func TestUpdateColumn(t *testing.T) {
	ctx := context.Background()
	core, db, d := newEmptyBoard(t)
	a := mustCreateColumn(t, core, d.ID, "todo")
	b := mustCreateColumn(t, core, d.ID, "doing")
	nextID := b.ID

	got, err := core.UpdateColumn(ctx, testActor, d.ID, a.ID, kanban.UpdateColumnInput{
		Name: strPtr("Todo"), Instruction: strPtr("新指令"), NextColumnID: &nextID,
	})
	if err != nil {
		t.Fatalf("修改失败: %v", err)
	}
	if got.Name != "Todo" || got.Instruction != "新指令" || got.NextColumnID != b.ID {
		t.Fatalf("字段不对: %+v", got)
	}
	read := boardColumns(t, core, d.ID)[0]
	if read.Name != "Todo" || read.NextColumnID != b.ID {
		t.Fatalf("读出来的数据不一致: %+v", read)
	}
	events := boardEvents(t, db, d.ID)
	last := events[len(events)-1]
	wantPayload := fmt.Sprintf(`{"column_id":%d,"fields":["name","instruction","next_column_id"],"name":"Todo"}`, a.ID)
	if last.Type != kanban.EventColumnUpdated || last.Payload != wantPayload {
		t.Fatalf("事件不对: %+v，期望内容 %s", last, wantPayload)
	}

	zero := int64(0)
	cleared, err := core.UpdateColumn(ctx, testActor, d.ID, a.ID, kanban.UpdateColumnInput{NextColumnID: &zero})
	if err != nil || cleared.NextColumnID != 0 {
		t.Fatalf("填 0 应该清除下一列: %+v %v", cleared, err)
	}
}

// TestUpdateColumnNoChange 验证提交的值和原来相同时，不写事件。
func TestUpdateColumnNoChange(t *testing.T) {
	ctx := context.Background()
	core, db, d := newEmptyBoard(t)
	col := mustCreateColumn(t, core, d.ID, "待办")
	before := len(boardEvents(t, db, d.ID))

	for _, in := range []kanban.UpdateColumnInput{{}, {Name: strPtr("待办")}} {
		if _, err := core.UpdateColumn(ctx, testActor, d.ID, col.ID, in); err != nil {
			t.Fatalf("没有变化也应该成功: %v", err)
		}
	}
	if after := len(boardEvents(t, db, d.ID)); after != before {
		t.Fatalf("没有变化不应该写事件，事件数 %d → %d", before, after)
	}
}

// TestUpdateColumnRejects 验证修改列的各种拒绝：改成别的列的名字 409、下一列指向自己或别的看板 400、
// 列不存在或属于别的看板 404、agent 403。
func TestUpdateColumnRejects(t *testing.T) {
	ctx := context.Background()
	core, _, d := newEmptyBoard(t)
	other, _ := core.CreateBoard(ctx, testActor, kanban.CreateBoardInput{Name: "别的", Slug: "OTHER", Template: kanban.TemplateSimple})
	a := mustCreateColumn(t, core, d.ID, "A")
	mustCreateColumn(t, core, d.ID, "B")
	self, foreign := a.ID, other.Columns[0].ID

	cases := []struct {
		name  string
		actor kanban.Actor
		board int64
		col   int64
		in    kanban.UpdateColumnInput
		want  error
	}{
		{"改成别的列的名字", testActor, d.ID, a.ID, kanban.UpdateColumnInput{Name: strPtr("b")}, kanban.ErrColumnNameTaken},
		{"下一列指向自己", testActor, d.ID, a.ID, kanban.UpdateColumnInput{NextColumnID: &self}, kanban.ErrInvalidInput},
		{"下一列属于别的看板", testActor, d.ID, a.ID, kanban.UpdateColumnInput{NextColumnID: &foreign}, kanban.ErrInvalidInput},
		{"名称为空", testActor, d.ID, a.ID, kanban.UpdateColumnInput{Name: strPtr(" ")}, kanban.ErrInvalidInput},
		{"列不存在", testActor, d.ID, 9999, kanban.UpdateColumnInput{Name: strPtr("x")}, kanban.ErrColumnNotFound},
		{"列属于别的看板", testActor, d.ID, foreign, kanban.UpdateColumnInput{Name: strPtr("x")}, kanban.ErrColumnNotFound},
		{"agent", testAgent, d.ID, a.ID, kanban.UpdateColumnInput{Name: strPtr("x")}, kanban.ErrHumanOnly},
		{"看板不存在", testActor, 9999, a.ID, kanban.UpdateColumnInput{Name: strPtr("x")}, kanban.ErrBoardNotFound},
	}
	for _, c := range cases {
		if _, err := core.UpdateColumn(ctx, c.actor, c.board, c.col, c.in); !errors.Is(err, c.want) {
			t.Errorf("%s：应该得到 %v，实际 %v", c.name, c.want, err)
		}
	}
	if got := boardColumns(t, core, d.ID); got[0].Name != "A" {
		t.Fatalf("失败的修改不应该生效: %+v", got[0])
	}
}

// columnIDs 返回各列的 id。
func columnIDs(cols []*kanban.Column) []int64 {
	ids := make([]int64, len(cols))
	for i, c := range cols {
		ids[i] = c.ID
	}
	return ids
}

// TestReorderColumns 验证重排：排序值按新顺序重写成 1024 的整数倍，读出来的顺序一致，
// 事件里记录完整的 id 列表；顺序没变时不写事件。
func TestReorderColumns(t *testing.T) {
	ctx := context.Background()
	core, db, d := newEmptyBoard(t)
	a, b, c := mustCreateColumn(t, core, d.ID, "A"), mustCreateColumn(t, core, d.ID, "B"), mustCreateColumn(t, core, d.ID, "C")

	want := []int64{c.ID, a.ID, b.ID}
	got, err := core.ReorderColumns(ctx, testActor, d.ID, want)
	if err != nil {
		t.Fatalf("重排失败: %v", err)
	}
	if !slices.Equal(columnIDs(got), want) {
		t.Fatalf("返回的顺序不对: %v", columnIDs(got))
	}
	read := boardColumns(t, core, d.ID)
	if !slices.Equal(columnIDs(read), want) || read[0].Sort != 1024 || read[1].Sort != 2048 || read[2].Sort != 3072 {
		t.Fatalf("读出来的顺序或排序值不对: %+v", read)
	}
	events := boardEvents(t, db, d.ID)
	last := events[len(events)-1]
	if last.Type != kanban.EventColumnReordered || last.Payload != fmt.Sprintf(`{"ids":[%d,%d,%d]}`, c.ID, a.ID, b.ID) {
		t.Fatalf("事件不对: %+v", last)
	}

	before := len(events)
	again, err := core.ReorderColumns(ctx, testActor, d.ID, want)
	if err != nil || !slices.Equal(columnIDs(again), want) {
		t.Fatalf("顺序没变也应该成功并返回当前顺序: %v %v", columnIDs(again), err)
	}
	if after := len(boardEvents(t, db, d.ID)); after != before {
		t.Fatalf("顺序没变不应该写事件，事件数 %d → %d", before, after)
	}
}

// TestReorderColumnsRejectsStaleRequest 验证请求的 id 列表和当前列集合不完全一致时返回 409：
// 少了、多了、重复、混进别的看板的列，都不会改动现有顺序；agent 403。
func TestReorderColumnsRejectsStaleRequest(t *testing.T) {
	ctx := context.Background()
	core, _, d := newEmptyBoard(t)
	other, _ := core.CreateBoard(ctx, testActor, kanban.CreateBoardInput{Name: "别的", Slug: "OTHER", Template: kanban.TemplateSimple})
	a, b := mustCreateColumn(t, core, d.ID, "A"), mustCreateColumn(t, core, d.ID, "B")

	stale := [][]int64{
		{a.ID},
		{a.ID, b.ID, 9999},
		{a.ID, a.ID},
		{a.ID, other.Columns[0].ID},
		{},
		nil,
	}
	for _, ids := range stale {
		if _, err := core.ReorderColumns(ctx, testActor, d.ID, ids); !errors.Is(err, kanban.ErrColumnOrderStale) {
			t.Errorf("%v 应该得到 ErrColumnOrderStale，实际 %v", ids, err)
		}
	}
	if _, err := core.ReorderColumns(ctx, testAgent, d.ID, []int64{b.ID, a.ID}); !errors.Is(err, kanban.ErrHumanOnly) {
		t.Fatalf("agent 应该得到 ErrHumanOnly，实际 %v", err)
	}
	if got := columnIDs(boardColumns(t, core, d.ID)); !slices.Equal(got, []int64{a.ID, b.ID}) {
		t.Fatalf("被拒绝的请求不应该改动顺序: %v", got)
	}
}
