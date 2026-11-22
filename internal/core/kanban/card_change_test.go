package kanban_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ixugo/goddd/pkg/orm"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/kanbantest"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
)

// changeNow 是变更日志测试里固定的时钟。
var changeNow = time.Date(2026, 11, 21, 10, 30, 0, 0, time.UTC)

// descCard 以人的身份在待办列创建一个带描述的任务，失败就让测试终止。
func (e *cardEnv) descCard(t *testing.T, title, description string) *kanban.Card {
	t.Helper()
	c, err := e.core.CreateCard(context.Background(), testActor, e.board.ID, kanban.CreateCardInput{
		Title: title, Description: description, ColumnID: e.col(colTodo),
	})
	if err != nil {
		t.Fatalf("创建任务 %s 失败: %v", title, err)
	}
	return c
}

// setDescription 以 actor 的身份把任务描述改成 text，失败就让测试终止。
func (e *cardEnv) setDescription(t *testing.T, actor kanban.Actor, cardID int64, text string) {
	t.Helper()
	if _, err := e.core.UpdateCard(context.Background(), actor, cardID, kanban.UpdateCardInput{Description: &text}); err != nil {
		t.Fatalf("修改描述失败: %v", err)
	}
}

// changeIDOf 取出事件载荷里的 description_change_id，没有这个字段返回 0。
func changeIDOf(t *testing.T, ev *kanban.Event) int64 {
	t.Helper()
	var payload struct {
		ID int64 `json:"description_change_id"`
	}
	if err := json.Unmarshal([]byte(ev.Payload), &payload); err != nil {
		t.Fatalf("事件载荷不是合法 JSON: %v", err)
	}
	return payload.ID
}

// TestUpdateDescriptionRecordsChange 验证每次改描述都在变更日志里留一条完整的前后记录：
// 内容、修改者和时间都对，事件载荷里的编号能读回同一条，连改两次得到两条互不覆盖的记录。
func TestUpdateDescriptionRecordsChange(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t, kanban.WithClock(func() time.Time { return changeNow }))
	c := e.descCard(t, "写文档", "第一版")

	e.setDescription(t, testActor, c.ID, "第二版")
	first := changeIDOf(t, e.lastEvent(t))
	if first == 0 {
		t.Fatalf("card.updated 的载荷里应该带 description_change_id: %+v", e.lastEvent(t))
	}
	got, err := e.core.GetCardChange(ctx, c.ID, first)
	if err != nil {
		t.Fatalf("读取变更记录失败: %v", err)
	}
	if got.ID != first || got.BoardID != e.board.ID || got.CardID != c.ID || got.Field != kanban.CardFieldDescription ||
		got.OldValue != "第一版" || got.NewValue != "第二版" || got.ActorKind != "human" || got.Actor != "admin" || !got.CreatedAt.Equal(changeNow) {
		t.Fatalf("变更记录不对: %+v", got)
	}

	e.setDescription(t, testActor, c.ID, "第三版")
	second := changeIDOf(t, e.lastEvent(t))
	if second == 0 || second == first {
		t.Fatalf("第二次修改应该有新的记录编号，实际 %d 和 %d", first, second)
	}
	again, err := e.core.GetCardChange(ctx, c.ID, second)
	if err != nil || again.OldValue != "第二版" || again.NewValue != "第三版" {
		t.Fatalf("第二条记录不对: %+v %v", again, err)
	}
	if kept, err := e.core.GetCardChange(ctx, c.ID, first); err != nil || kept.OldValue != "第一版" || kept.NewValue != "第二版" {
		t.Fatalf("第一条记录不应该被覆盖: %+v %v", kept, err)
	}
}

// TestUpdateDescriptionClearAndFill 验证从有到无、从无到有也各记一条：清空时新值是空串，补写时旧值是空串。
func TestUpdateDescriptionClearAndFill(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	c := e.descCard(t, "空描述", "")

	e.setDescription(t, testActor, c.ID, "补上说明")
	fill, err := e.core.GetCardChange(ctx, c.ID, changeIDOf(t, e.lastEvent(t)))
	if err != nil || fill.OldValue != "" || fill.NewValue != "补上说明" {
		t.Fatalf("补写应该记旧值为空: %+v %v", fill, err)
	}
	e.setDescription(t, testActor, c.ID, "")
	cleared, err := e.core.GetCardChange(ctx, c.ID, changeIDOf(t, e.lastEvent(t)))
	if err != nil || cleared.OldValue != "补上说明" || cleared.NewValue != "" {
		t.Fatalf("清空应该记新值为空: %+v %v", cleared, err)
	}
}

// TestUpdateWithoutDescriptionChangeWritesNoRecord 验证没改描述就不写记录：只改标题、
// 或者提交的描述和原来一样，都不产生变更记录，事件载荷也不带 description_change_id。
func TestUpdateWithoutDescriptionChangeWritesNoRecord(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	c := e.descCard(t, "旧标题", "原描述")

	if _, err := e.core.UpdateCard(ctx, testActor, c.ID, kanban.UpdateCardInput{Title: strPtr("新标题")}); err != nil {
		t.Fatalf("改标题失败: %v", err)
	}
	if id := changeIDOf(t, e.lastEvent(t)); id != 0 {
		t.Fatalf("只改标题不该带变更编号，实际 %d", id)
	}

	before := len(e.events(t))
	e.setDescription(t, testActor, c.ID, "原描述")
	if after := len(e.events(t)); after != before {
		t.Fatalf("描述没变不该写事件，事件数从 %d 变成 %d", before, after)
	}
	if _, err := e.db.CardChange().GetByID(ctx, 1); !orm.IsErrRecordNotFound(err) {
		t.Fatalf("没改描述不该有任何变更记录，实际: %v", err)
	}
}

// TestDescriptionChangeCoexistsWithOtherFields 验证同一次修改既改描述又改别的字段时，
// 记录照常写，载荷里的字段清单仍然列出全部变化的字段。
func TestDescriptionChangeCoexistsWithOtherFields(t *testing.T) {
	e := newCardEnv(t)
	c := e.descCard(t, "旧标题", "旧描述")

	input := kanban.UpdateCardInput{Title: strPtr("新标题"), Description: strPtr("新描述"), Priority: intPtr(2)}
	if _, err := e.core.UpdateCard(context.Background(), testActor, c.ID, input); err != nil {
		t.Fatalf("修改失败: %v", err)
	}
	ev := e.lastEvent(t)
	want := fmt.Sprintf(`{"description_change_id":%d,"fields":["title","description","priority"],"key":"TST-1","previous_values":{"priority":0},"title":"新标题"}`, changeIDOf(t, ev))
	if changeIDOf(t, ev) == 0 || ev.Payload != want {
		t.Fatalf("事件载荷不对: %s，期望 %s", ev.Payload, want)
	}
}

// TestDescriptionChangeRecordsAgent 验证 agent 改自己持有的任务的描述，记录里的修改者是这个 agent。
func TestDescriptionChangeRecordsAgent(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	c := e.descCard(t, "agent 写的", "初稿")
	e.holdUntil(t, c.ID, testAgent.Name, claimNow.Add(time.Hour))

	e.setDescription(t, testAgent, c.ID, "agent 改过")
	got, err := e.core.GetCardChange(ctx, c.ID, changeIDOf(t, e.lastEvent(t)))
	if err != nil || got.ActorKind != "agent" || got.Actor != testAgent.Name || got.OldValue != "初稿" || got.NewValue != "agent 改过" {
		t.Fatalf("agent 的修改记录不对: %+v %v", got, err)
	}
}

// failingEventStore 包住真实的事件存储，写事件永远失败，用来模拟事务中途出错。
type failingEventStore struct{ kanban.EventStorer }

// WithTx 让事务里拿到的副本同样会失败。
func (f failingEventStore) WithTx(tx orm.Tx) kanban.EventStorer {
	return failingEventStore{f.EventStorer.WithTx(tx)}
}

// Create 总是返回错误。
func (failingEventStore) Create(context.Context, *kanban.Event) error { return errors.New("boom") }

// failingEventsDB 是事件存储会失败、其余照常的存储。
type failingEventsDB struct{ kanbandb.DB }

// Event 返回会失败的事件存储。
func (d failingEventsDB) Event() kanban.EventStorer { return failingEventStore{d.DB.Event()} }

// TestDescriptionChangeRollsBackWithEvent 验证变更记录和事件同生共死：事件写不进去时，
// 整个修改回滚，变更记录、新描述和版本号都不留痕迹。
func TestDescriptionChangeRollsBackWithEvent(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	c := e.descCard(t, "会失败的修改", "旧描述")
	broken := kanban.NewCore(failingEventsDB{e.db}, kanbantest.Resources(t))

	_, err := broken.UpdateCard(ctx, testActor, c.ID, kanban.UpdateCardInput{Description: strPtr("新描述")})
	if err == nil {
		t.Fatal("事件写失败时修改应该报错")
	}
	if _, err := e.db.CardChange().GetByID(ctx, 1); !orm.IsErrRecordNotFound(err) {
		t.Fatalf("回滚后不该留下变更记录，实际: %v", err)
	}
	if stored := e.stored(t, c.ID); stored.Description != "旧描述" || stored.Version != c.Version {
		t.Fatalf("回滚后任务应该保持原样: %+v", stored)
	}
}

// TestGetCardChangeScopedToCard 验证读取变更记录必须带对任务编号：
// 拿别的任务的记录编号、不存在的编号都得到 ErrCardChangeNotFound，读不到别的任务的内容。
func TestGetCardChangeScopedToCard(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	a := e.descCard(t, "卡 A", "A 旧")
	b := e.descCard(t, "卡 B", "B 旧")
	e.setDescription(t, testActor, a.ID, "A 新")
	aChange := changeIDOf(t, e.lastEvent(t))

	if _, err := e.core.GetCardChange(ctx, b.ID, aChange); !errors.Is(err, kanban.ErrCardChangeNotFound) {
		t.Fatalf("用别的任务读取应该得到 ErrCardChangeNotFound，实际 %v", err)
	}
	if _, err := e.core.GetCardChange(ctx, a.ID, 9999); !errors.Is(err, kanban.ErrCardChangeNotFound) {
		t.Fatalf("不存在的编号应该得到 ErrCardChangeNotFound，实际 %v", err)
	}
	if got, err := e.core.GetCardChange(ctx, a.ID, aChange); err != nil || got.NewValue != "A 新" {
		t.Fatalf("用自己的任务应该能读到: %+v %v", got, err)
	}
}

// TestDeleteCardRemovesItsChanges 验证删除任务会一并删掉它的变更记录，别的任务的记录不受影响。
func TestDeleteCardRemovesItsChanges(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	gone := e.descCard(t, "要删的卡", "旧")
	kept := e.descCard(t, "留下的卡", "旧")
	e.setDescription(t, testActor, gone.ID, "新")
	goneChange := changeIDOf(t, e.lastEvent(t))
	e.setDescription(t, testActor, kept.ID, "新")
	keptChange := changeIDOf(t, e.lastEvent(t))

	if err := e.core.DeleteCard(ctx, testActor, gone.ID); err != nil {
		t.Fatalf("删除任务失败: %v", err)
	}
	if _, err := e.core.GetCardChange(ctx, gone.ID, goneChange); !errors.Is(err, kanban.ErrCardChangeNotFound) {
		t.Fatalf("任务删除后它的变更记录应该没了，实际 %v", err)
	}
	if _, err := e.core.GetCardChange(ctx, kept.ID, keptChange); err != nil {
		t.Fatalf("别的任务的变更记录不该受影响: %v", err)
	}
}

// TestDeleteBoardRemovesItsChanges 验证删除看板会清掉全部变更记录，别的看板的记录不受影响。
func TestDeleteBoardRemovesItsChanges(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	other := newCardEnvOn(t, e.core, e.db, "OTH")
	mine := e.descCard(t, "我的卡", "旧")
	theirs := other.descCard(t, "别家的卡", "旧")
	e.setDescription(t, testActor, mine.ID, "新")
	mineChange := changeIDOf(t, e.lastEvent(t))
	other.setDescription(t, testActor, theirs.ID, "新")
	theirChange := changeIDOf(t, other.lastEvent(t))

	if err := e.core.DeleteBoard(ctx, testActor, e.board.ID); err != nil {
		t.Fatalf("删除看板失败: %v", err)
	}
	if _, err := e.db.CardChange().GetByID(ctx, mineChange); !orm.IsErrRecordNotFound(err) {
		t.Fatalf("看板删除后变更记录应该没了，实际 %v", err)
	}
	if _, err := e.core.GetCardChange(ctx, theirs.ID, theirChange); err != nil {
		t.Fatalf("别的看板的变更记录不该受影响: %v", err)
	}
}

// TestGetCardChangeNeighbors 验证读取变更记录时带上同一个任务前一条和后一条记录的编号，
// 第一条没有前一条、最后一条没有后一条（为 0），别的任务的记录不会算作邻居。
func TestGetCardChangeNeighbors(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t, kanban.WithClock(func() time.Time { return changeNow }))
	c := e.descCard(t, "写文档", "v1")
	other := e.descCard(t, "别的卡", "o1")
	var ids []int64
	for _, text := range []string{"v2", "v3", "v4"} {
		e.setDescription(t, testActor, c.ID, text)
		ids = append(ids, changeIDOf(t, e.lastEvent(t)))
		e.setDescription(t, testActor, other.ID, "o-"+text) // 穿插别的任务的修改，邻居不能落到它上面
	}
	want := [][2]int64{{0, ids[1]}, {ids[0], ids[2]}, {ids[1], 0}}
	for i, id := range ids {
		got, err := e.core.GetCardChange(ctx, c.ID, id)
		if err != nil {
			t.Fatalf("读取第 %d 条失败: %v", i, err)
		}
		if got.PrevID != want[i][0] || got.NextID != want[i][1] {
			t.Fatalf("第 %d 条的邻居应该是 %v，实际 prev=%d next=%d", i, want[i], got.PrevID, got.NextID)
		}
	}
}
