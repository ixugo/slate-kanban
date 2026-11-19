package kanban

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// claimFixture 构造领取测试用的看板：待办、进行中（开启 agent 领取）、审查、完成四列，
// 和一批领取资格各不相同的卡片。
func claimFixture() (*boardIndex, map[string]*Card) {
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	cols := []*Column{
		{ID: 10, Name: "待办"},
		{ID: 20, Name: "进行中", AgentPickup: true},
		{ID: 25, Name: "审查"},
		{ID: 30, Name: "完成", IsDone: true},
	}
	future, past := now.Add(time.Hour), now.Add(-time.Hour)
	cards := map[string]*Card{
		"free":           {ID: 1, Key: "K-1", ColumnID: 20},
		"human":          {ID: 2, Key: "K-2", ColumnID: 20, HumanOnly: true},
		"todo":           {ID: 3, Key: "K-3", ColumnID: 10},
		"review":         {ID: 4, Key: "K-4", ColumnID: 25},
		"waiting":        {ID: 5, Key: "K-5", ColumnID: 20, WaitsOn: []int64{3, 4}},
		"depDone":        {ID: 6, Key: "K-6", ColumnID: 20, WaitsOn: []int64{7}},
		"finished":       {ID: 7, Key: "K-7", ColumnID: 30},
		"heldByOther":    {ID: 8, Key: "K-8", ColumnID: 20, ClaimedBy: "codex", ClaimExpiresAt: &future},
		"expiredOfOther": {ID: 9, Key: "K-9", ColumnID: 20, ClaimedBy: "codex", ClaimExpiresAt: &past},
		"heldBySelf":     {ID: 10, Key: "K-10", ColumnID: 20, ClaimedBy: "claude", ClaimExpiresAt: &future},
	}
	list := make([]*Card, 0, len(cards))
	for _, c := range cards {
		list = append(list, c)
	}
	return newBoardIndex(now, time.UTC, cols, nil, list), cards
}

// TestCheckClaimable 验证领取资格：按固定顺序判断，每种不满足都得到对应的错误，提示里带着有用的细节。
func TestCheckClaimable(t *testing.T) {
	ix, cards := claimFixture()
	me := Actor{Kind: ActorAgent, Name: "claude"}
	cases := []struct {
		card string
		want error    // nil 表示可以领取
		must []string // 错误提示里必须出现的内容
	}{
		{"free", nil, nil},
		{"depDone", nil, nil},
		{"expiredOfOther", nil, nil},
		{"heldBySelf", nil, nil},
		{"human", ErrCardHumanOnly, nil},
		{"todo", ErrColumnNotPickup, []string{"进行中"}},
		{"review", ErrColumnNotPickup, []string{"进行中"}},
		{"waiting", ErrCardWaiting, []string{"K-3", "K-4"}},
		{"heldByOther", ErrCardClaimed, []string{"codex", "2026-11-17T13:00:00Z"}},
	}
	for _, c := range cases {
		err := checkClaimable(me, cards[c.card], ix)
		if c.want == nil {
			if err != nil {
				t.Errorf("%s：应该可以领取，实际 %v", c.card, err)
			}
			continue
		}
		if !errors.Is(err, c.want) {
			t.Errorf("%s：期望 %v，实际 %v", c.card, c.want, err)
			continue
		}
		for _, s := range c.must {
			if !strings.Contains(err.Error(), s) {
				t.Errorf("%s：错误提示里应该有 %q，实际 %q", c.card, s, err.Error())
			}
		}
	}
}

// TestCheckClaimableOrder 验证判断顺序：同时有多个问题时先报前面的，
// 例如又是只能由人处理、又不在领活列的卡，先提示只能由人处理。
func TestCheckClaimableOrder(t *testing.T) {
	ix, _ := claimFixture()
	me := Actor{Kind: ActorAgent, Name: "claude"}
	card := &Card{ID: 99, ColumnID: 10, HumanOnly: true, WaitsOn: []int64{3}}
	if err := checkClaimable(me, card, ix); !errors.Is(err, ErrCardHumanOnly) {
		t.Fatalf("应该先报 ErrCardHumanOnly，实际 %v", err)
	}
	card.HumanOnly = false
	if err := checkClaimable(me, card, ix); !errors.Is(err, ErrColumnNotPickup) {
		t.Fatalf("应该先报 ErrColumnNotPickup，实际 %v", err)
	}
}

// TestNotPickupErrorWithoutPickupColumns 验证看板里没有任何领活列时，提示让人先去列设置里开启。
func TestNotPickupErrorWithoutPickupColumns(t *testing.T) {
	ix := newBoardIndex(time.Now(), time.UTC, []*Column{{ID: 1, Name: "待办"}}, nil, nil)
	card := &Card{ID: 1, ColumnID: 1}
	err := checkClaimable(Actor{Kind: ActorAgent, Name: "claude"}, card, ix)
	if !errors.Is(err, ErrColumnNotPickup) || !strings.Contains(err.Error(), "列设置") {
		t.Fatalf("应该提示去列设置里开启，实际 %v", err)
	}
}

// TestNormalizeLease 验证租约时长：0 取默认 30 分钟，1 到 1440 分钟原样使用，之外报错。
func TestNormalizeLease(t *testing.T) {
	cases := []struct {
		minutes int
		want    time.Duration
		bad     bool
	}{
		{0, 30 * time.Minute, false},
		{1, time.Minute, false},
		{90, 90 * time.Minute, false},
		{1440, 24 * time.Hour, false},
		{-1, 0, true},
		{1441, 0, true},
	}
	for _, c := range cases {
		got, err := normalizeLease(c.minutes)
		if c.bad {
			if !errors.Is(err, ErrInvalidInput) {
				t.Errorf("%d 分钟：应该得到 ErrInvalidInput，实际 %v", c.minutes, err)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("%d 分钟：期望 %v，实际 %v, %v", c.minutes, c.want, got, err)
		}
	}
}
