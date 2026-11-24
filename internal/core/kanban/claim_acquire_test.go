package kanban_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"slate-kanban/internal/core/kanban"
)

// claimNow 是领取测试里固定的"现在"。
var claimNow = time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)

// codex 是测试里的第二个 agent。
var codex = kanban.Actor{Kind: kanban.ActorAgent, Name: "codex"}

// newClaimEnv 创建时钟固定的任务测试环境。
func newClaimEnv(t *testing.T) *cardEnv {
	t.Helper()
	return newCardEnv(t, kanban.WithClock(func() time.Time { return claimNow }))
}

// holdUntil 直接在数据库里把任务标成由某个 agent 持有，租约到 expires 为止。
func (e *cardEnv) holdUntil(t *testing.T, id int64, agent string, expires time.Time) {
	t.Helper()
	c := e.stored(t, id)
	c.ClaimedBy, c.ClaimExpiresAt = agent, &expires
	if err := e.db.Card().Save(context.Background(), c); err != nil {
		t.Fatalf("标记持有失败: %v", err)
	}
}

// TestClaimCard 验证领取：任务记到 agent 名下，租约默认 30 分钟，版本加一，
// 返回的详情带着列的 agent 指令，并追加 card.claimed 事件。
func TestClaimCard(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	instruction := "先写测试"
	if _, err := e.core.UpdateColumn(ctx, testActor, e.board.ID, e.col(colDoing), kanban.UpdateColumnInput{Instruction: &instruction}); err != nil {
		t.Fatalf("设置列指令失败: %v", err)
	}
	c := e.card(t, colDoing, "任务")
	events := len(e.events(t))

	d, err := e.core.ClaimCard(ctx, testAgent, c.ID, kanban.ClaimCardInput{})
	if err != nil {
		t.Fatalf("领取失败: %v", err)
	}
	if d.ClaimedBy != "claude" || !d.ClaimActive || d.Version != 2 || d.ColumnInstruction != instruction || d.UpdatedBy != "claude" {
		t.Fatalf("详情不对: %+v 指令=%q", d.CardView.Card, d.ColumnInstruction)
	}
	if after := e.events(t); len(after) != events+1 {
		t.Fatalf("领取空闲任务只应该写一条 card.claimed，不该有租约到期: %+v", after[events:])
	}
	wantExpire := claimNow.Add(30 * time.Minute)
	if got := e.stored(t, c.ID); got.ClaimedBy != "claude" || got.ClaimExpiresAt == nil || !got.ClaimExpiresAt.Equal(wantExpire) {
		t.Fatalf("存储的领取状态不对: %+v", got)
	}
	want := `{"agent":"claude","engineer_id":0,"expires_at":"2026-11-17T12:30:00Z","key":"TST-1","title":"任务"}`
	if ev := e.lastEvent(t); ev.Type != kanban.EventCardClaimed || ev.CardID != c.ID || ev.Actor != "claude" || ev.Payload != want {
		t.Fatalf("事件不对: %+v，期望内容 %s", ev, want)
	}
}

// TestClaimCardRenew 验证持有者再次领取只顺延租约：用新的时长重新算到期时间，版本不变，也不再次写 card.claimed。
func TestClaimCardRenew(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	c := e.card(t, colDoing, "任务")
	if _, err := e.core.ClaimCard(ctx, testAgent, c.ID, kanban.ClaimCardInput{LeaseMinutes: 5}); err != nil {
		t.Fatalf("首次领取失败: %v", err)
	}
	events := len(e.events(t))

	d, err := e.core.ClaimCard(ctx, testAgent, c.ID, kanban.ClaimCardInput{LeaseMinutes: 120})
	if err != nil {
		t.Fatalf("续租失败: %v", err)
	}
	if d.Version != 2 {
		t.Fatalf("续租不应该涨版本，实际 %d", d.Version)
	}
	if got := e.stored(t, c.ID); !got.ClaimExpiresAt.Equal(claimNow.Add(2 * time.Hour)) {
		t.Fatalf("到期时间应该顺延到两小时后: %v", got.ClaimExpiresAt)
	}
	if after := e.events(t); len(after) != events {
		t.Fatalf("续租不应该新增事件: %+v", after[events:])
	}
}

// TestClaimCardReacquiresExpiredLeaseForSameAgent 验证同一个 agent 在自己的租约到期后重新领取：
// 版本加一，先记一条租约到期（card.released，why 为 expired），再记一条新的 card.claimed。
func TestClaimCardReacquiresExpiredLeaseForSameAgent(t *testing.T) {
	ctx := t.Context()
	e := newClaimEnv(t)
	c := e.card(t, colDoing, "任务")
	e.holdUntil(t, c.ID, testAgent.Name, claimNow)
	events := len(e.events(t))

	d, err := e.core.ClaimCard(ctx, testAgent, c.ID, kanban.ClaimCardInput{})
	if err != nil {
		t.Fatal(err)
	}
	if !d.ClaimActive || d.Version != c.Version+1 {
		t.Fatalf("同一 agent 重新取得过期租约应该涨版本: %+v", d)
	}
	after := e.events(t)
	if len(after) != events+2 {
		t.Fatalf("重新领取过期租约应该写租约到期和新领取两条事件: %+v", after[events:])
	}
	wantExpired := `{"agent":"claude","expired_at":"2026-11-17T12:00:00Z","key":"TST-1","title":"任务","why":"expired"}`
	if ev := after[events]; ev.Type != kanban.EventCardReleased || ev.CardID != c.ID || ev.Actor != "claude" || ev.Payload != wantExpired {
		t.Fatalf("第一条应该是租约到期，实际 %+v，期望内容 %s", ev, wantExpired)
	}
	if ev := after[events+1]; ev.Type != kanban.EventCardClaimed || ev.CardID != c.ID {
		t.Fatalf("第二条应该是新的领取: %+v", ev)
	}
}

// TestClaimCardTakeover 验证接手：别人的租约还有效就拒绝并说明是谁；已经过期就可以接手，版本加一。
func TestClaimCardTakeover(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	c := e.card(t, colDoing, "任务")

	e.holdUntil(t, c.ID, "codex", claimNow.Add(time.Minute))
	_, err := e.core.ClaimCard(ctx, testAgent, c.ID, kanban.ClaimCardInput{})
	if !errors.Is(err, kanban.ErrCardClaimed) {
		t.Fatalf("租约有效应该得到 ErrCardClaimed，实际 %v", err)
	}
	if got := e.stored(t, c.ID); got.ClaimedBy != "codex" || got.Version != 1 {
		t.Fatalf("被拒绝的领取不应该改动任务: %+v", got)
	}

	e.holdUntil(t, c.ID, "codex", claimNow.Add(-time.Second))
	events := len(e.events(t))
	d, err := e.core.ClaimCard(ctx, testAgent, c.ID, kanban.ClaimCardInput{})
	if err != nil || d.ClaimedBy != "claude" || d.Version != 2 {
		t.Fatalf("租约过期应该可以接手: %+v %v", d, err)
	}
	after := e.events(t)
	if len(after) != events+2 {
		t.Fatalf("接手过期租约应该写租约到期和新领取两条事件: %+v", after[events:])
	}
	// 到期事件记在接手者名下，原持有者才能在看板更新里收到「你的租约到期了」。
	wantExpired := `{"agent":"codex","expired_at":"2026-11-17T11:59:59Z","key":"TST-1","title":"任务","why":"expired"}`
	if ev := after[events]; ev.Type != kanban.EventCardReleased || ev.Actor != "claude" || ev.Payload != wantExpired {
		t.Fatalf("第一条应该是原持有者的租约到期，实际 %+v，期望内容 %s", ev, wantExpired)
	}
	if ev := after[events+1]; ev.Type != kanban.EventCardClaimed || ev.Actor != "claude" {
		t.Fatalf("第二条应该是接手者的领取: %+v", ev)
	}
}

// TestClaimCardRejects 验证拒绝情况：人和身份无效、任务不存在、不满足领取资格、租约时长不合法；
// 失败都不改动任务、不写事件。
func TestClaimCardRejects(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	free := e.card(t, colDoing, "可领")
	humanOnly, err := e.core.CreateCard(ctx, testActor, e.board.ID, kanban.CreateCardInput{Title: "只能人做", ColumnID: e.col(colDoing), HumanOnly: true})
	if err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}
	todo := e.card(t, colTodo, "还在待办")
	waiting := e.card(t, colDoing, "在等待")
	deps := []int64{todo.ID}
	if _, err := e.core.UpdateCard(ctx, testActor, waiting.ID, kanban.UpdateCardInput{WaitsOn: &deps}); err != nil {
		t.Fatalf("设置依赖失败: %v", err)
	}
	events := len(e.events(t))

	cases := []struct {
		name  string
		actor kanban.Actor
		id    int64
		in    kanban.ClaimCardInput
		want  error
	}{
		{"人领取", testActor, free.ID, kanban.ClaimCardInput{}, kanban.ErrAgentOnly},
		{"自动化领取", kanban.Actor{Kind: kanban.ActorSystem, Name: "automation"}, free.ID, kanban.ClaimCardInput{}, kanban.ErrAgentOnly},
		{"身份无效", kanban.Actor{}, free.ID, kanban.ClaimCardInput{}, kanban.ErrInvalidActor},
		{"任务不存在", testAgent, 9999, kanban.ClaimCardInput{}, kanban.ErrCardNotFound},
		{"只能由人处理", testAgent, humanOnly.ID, kanban.ClaimCardInput{}, kanban.ErrCardHumanOnly},
		{"依赖未完成", testAgent, waiting.ID, kanban.ClaimCardInput{}, kanban.ErrCardWaiting},
		{"租约为负", testAgent, free.ID, kanban.ClaimCardInput{LeaseMinutes: -1}, kanban.ErrInvalidInput},
		{"租约超过上限", testAgent, free.ID, kanban.ClaimCardInput{LeaseMinutes: 1441}, kanban.ErrInvalidInput},
	}
	for _, c := range cases {
		if _, err := e.core.ClaimCard(ctx, c.actor, c.id, c.in); !errors.Is(err, c.want) {
			t.Errorf("%s：期望 %v，实际 %v", c.name, c.want, err)
		}
	}
	if after := len(e.events(t)); after != events {
		t.Errorf("失败的领取不应该写事件，事件数 %d → %d", events, after)
	}
	if got := e.stored(t, free.ID); got.ClaimedBy != "" || got.Version != 1 {
		t.Errorf("失败的领取不应该改动任务: %+v", got)
	}
}

// TestClaimCardConcurrently 验证并发领取同一张卡：很多个 agent 同时抢，恰好一个成功，其余都得到 ErrCardClaimed。
func TestClaimCardConcurrently(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	c := e.card(t, colDoing, "抢手的卡")

	const agents = 8
	results := make([]error, agents)
	var wg sync.WaitGroup
	for i := range agents {
		wg.Add(1)
		go func() {
			defer wg.Done()
			actor := kanban.Actor{Kind: kanban.ActorAgent, Name: fmt.Sprintf("agent-%d", i)}
			_, results[i] = e.core.ClaimCard(ctx, actor, c.ID, kanban.ClaimCardInput{})
		}()
	}
	wg.Wait()

	won := 0
	for i, err := range results {
		switch {
		case err == nil:
			won++
		case !errors.Is(err, kanban.ErrCardClaimed):
			t.Errorf("agent-%d：失败的原因应该是 ErrCardClaimed，实际 %v", i, err)
		}
	}
	if won != 1 {
		t.Fatalf("应该恰好一个成功，实际 %d 个", won)
	}
}

// TestNextCardOrder 验证取卡的顺序：领活列从左到右、同列从上到下，跳过依赖未完成的、被别人持有的、只能人做的；
// 优先级不参与排序；自己已经持有的任务优先，重复领取返回同一任务。
func TestNextCardOrder(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	todo := e.card(t, colTodo, "没做完的前置")
	blocked := e.card(t, colDoing, "被依赖卡住")
	deps := []int64{todo.ID}
	if _, err := e.core.UpdateCard(ctx, testActor, blocked.ID, kanban.UpdateCardInput{WaitsOn: &deps}); err != nil {
		t.Fatalf("设置依赖失败: %v", err)
	}
	heldByOther := e.card(t, colDoing, "别人在做")
	e.holdUntil(t, heldByOther.ID, "someone-else", claimNow.Add(time.Hour))
	if _, err := e.core.CreateCard(ctx, testActor, e.board.ID, kanban.CreateCardInput{Title: "只能人做", ColumnID: e.col(colDoing), HumanOnly: true}); err != nil {
		t.Fatalf("创建只能人做的任务失败: %v", err)
	}
	first := e.card(t, colDoing, "进行中第一张可领的")
	second := e.card(t, colDoing, "进行中第二张但优先级高")
	high := 3
	if _, err := e.core.UpdateCard(ctx, testActor, second.ID, kanban.UpdateCardInput{Priority: &high}); err != nil {
		t.Fatalf("设置优先级失败: %v", err)
	}
	review := e.card(t, colReview, "审查列的卡")

	d, err := e.core.NextCard(ctx, testAgent, e.board.ID, kanban.NextCardInput{ColumnID: e.col(colDoing)})
	if err != nil || d == nil || d.ID != first.ID || d.ClaimedBy != "claude" || !d.ClaimActive {
		t.Fatalf("应该取到进行中第一张可领的: %+v %v", d, err)
	}
	again, err := e.core.NextCard(ctx, testAgent, e.board.ID, kanban.NextCardInput{ColumnID: e.col(colDoing)})
	if err != nil || again == nil || again.ID != first.ID || again.Version != d.Version {
		t.Fatalf("再取应该返回自己持有的同一张并续租，不涨版本: %+v %v", again, err)
	}
	other, err := e.core.NextCard(ctx, codex, e.board.ID, kanban.NextCardInput{ColumnID: e.col(colDoing)})
	if err != nil || other == nil || other.ID != second.ID {
		t.Fatalf("别的 agent 应该取到下一张，不管优先级: %+v %v", other, err)
	}
	if remaining, err := e.core.NextCard(ctx, codex, e.board.ID, kanban.NextCardInput{ColumnID: e.col(colDoing)}); err != nil || remaining == nil || remaining.ID != second.ID {
		t.Fatalf("只能续领所选列内的任务，不可跳到审查列: %+v %v", remaining, err)
	}
	third, err := e.core.NextCard(ctx, kanban.Actor{Kind: kanban.ActorAgent, Name: "gemini"}, e.board.ID, kanban.NextCardInput{ColumnID: e.col(colReview)})
	if err != nil || third == nil || third.ID != review.ID {
		t.Fatalf("显式选择审查列应取到该列的任务: %+v %v", third, err)
	}
}

// TestNextCardPrefersOwnHeldCard 验证 agent 重启后接着做原来的活：自己持有的任务排在后面，
// 前面有别的空闲任务时，仍然先返回自己持有的那项任务。
func TestNextCardPrefersOwnHeldCard(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	e.card(t, colDoing, "前面的空闲卡")
	mine := e.card(t, colDoing, "我持有的")
	e.holdUntil(t, mine.ID, "claude", claimNow.Add(-time.Hour)) // 租约过期了也算自己的
	events := len(e.events(t))

	d, err := e.core.NextCard(ctx, testAgent, e.board.ID, kanban.NextCardInput{ColumnID: e.col(colDoing), LeaseMinutes: 10})
	if err != nil || d == nil || d.ID != mine.ID || !d.ClaimActive {
		t.Fatalf("应该先返回自己持有的并续租: %+v %v", d, err)
	}
	after := e.events(t)
	if len(after) != events+2 || after[events].Type != kanban.EventCardReleased || after[events+1].Type != kanban.EventCardClaimed {
		t.Fatalf("租约已到期时取回自己的任务应该先记租约到期再记新领取: %+v", after[events:])
	}
}

// TestNextCardNothingToDo 验证没有可做的任务不是错误：返回 nil，不写事件。
func TestNextCardNothingToDo(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	e.card(t, colTodo, "只有待办的卡")
	events := len(e.events(t))

	d, err := e.core.NextCard(ctx, testAgent, e.board.ID, kanban.NextCardInput{ColumnID: e.col(colDoing)})
	if err != nil || d != nil {
		t.Fatalf("没有可做的卡应该返回 nil, nil，实际 %+v %v", d, err)
	}
	if after := len(e.events(t)); after != events {
		t.Fatalf("没有可做的卡不应该写事件，事件数 %d → %d", events, after)
	}
}

// TestNextCardRejects 验证拒绝情况：人、身份无效、看板不存在、租约不合法。
func TestNextCardRejects(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	for name, tc := range map[string]struct {
		actor kanban.Actor
		board int64
		in    kanban.NextCardInput
		want  error
	}{
		"未选择列":  {testAgent, e.board.ID, kanban.NextCardInput{}, kanban.ErrInvalidInput},
		"列不存在":  {testAgent, e.board.ID, kanban.NextCardInput{ColumnID: 99999}, kanban.ErrInvalidInput},
		"完成列":   {testAgent, e.board.ID, kanban.NextCardInput{ColumnID: e.col(colDone)}, kanban.ErrInvalidInput},
		"人":     {testActor, e.board.ID, kanban.NextCardInput{ColumnID: e.col(colDoing)}, kanban.ErrAgentOnly},
		"身份无效":  {kanban.Actor{}, e.board.ID, kanban.NextCardInput{ColumnID: e.col(colDoing)}, kanban.ErrInvalidActor},
		"看板不存在": {testAgent, 9999, kanban.NextCardInput{ColumnID: e.col(colDoing)}, kanban.ErrBoardNotFound},
		"租约不合法": {testAgent, e.board.ID, kanban.NextCardInput{ColumnID: e.col(colDoing), LeaseMinutes: 2000}, kanban.ErrInvalidInput},
	} {
		if _, err := e.core.NextCard(ctx, tc.actor, tc.board, tc.in); !errors.Is(err, tc.want) {
			t.Errorf("%s：期望 %v，实际 %v", name, tc.want, err)
		}
	}
}

// TestNextCardConcurrently 验证并发取卡：任务数等于 agent 数时，每个 agent 都取到一张，而且互不相同。
func TestNextCardConcurrently(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	const agents = 8
	for i := range agents {
		e.card(t, colDoing, fmt.Sprintf("任务 %d", i))
	}

	got := make([]*kanban.CardDetail, agents)
	errs := make([]error, agents)
	var wg sync.WaitGroup
	for i := range agents {
		wg.Add(1)
		go func() {
			defer wg.Done()
			actor := kanban.Actor{Kind: kanban.ActorAgent, Name: fmt.Sprintf("agent-%d", i)}
			got[i], errs[i] = e.core.NextCard(ctx, actor, e.board.ID, kanban.NextCardInput{ColumnID: e.col(colDoing)})
		}()
	}
	wg.Wait()

	seen := map[int64]string{}
	for i := range agents {
		if errs[i] != nil || got[i] == nil {
			t.Fatalf("agent-%d 应该取到一张卡: %+v %v", i, got[i], errs[i])
		}
		if prev, dup := seen[got[i].ID]; dup {
			t.Fatalf("任务 %d 被 %s 和 agent-%d 同时取到", got[i].ID, prev, i)
		}
		seen[got[i].ID] = fmt.Sprintf("agent-%d", i)
	}
}
