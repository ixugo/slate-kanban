package kanban

import (
	"testing"
	"time"
)

// viewFixture 构造一个看板的内存数据：三列（待办、进行中、完成），几张带依赖和租约的卡片。
func viewFixture() (*boardIndex, map[string]*Card) {
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	cols := []*Column{
		{ID: 10, Name: "待办"}, {ID: 20, Name: "进行中"}, {ID: 30, Name: "完成", IsDone: true},
	}
	future, past := now.Add(time.Hour), now.Add(-time.Hour)
	cards := map[string]*Card{
		"open":      {ID: 1, ColumnID: 10, Position: 2048},
		"finished":  {ID: 2, ColumnID: 30, Position: 1024},
		"onOpen":    {ID: 3, ColumnID: 20, Position: 1024, WaitsOn: []int64{1}},
		"onDone":    {ID: 4, ColumnID: 20, Position: 2048, WaitsOn: []int64{2}},
		"onBoth":    {ID: 5, ColumnID: 10, Position: 1024, WaitsOn: []int64{2, 1}},
		"onMissing": {ID: 6, ColumnID: 10, Position: 3072, WaitsOn: []int64{99}},
		"claimed":   {ID: 7, ColumnID: 20, Position: 3072, ClaimedBy: "claude", ClaimExpiresAt: &future},
		"expired":   {ID: 8, ColumnID: 20, Position: 4096, ClaimedBy: "claude", ClaimExpiresAt: &past},
	}
	list := make([]*Card, 0, len(cards))
	for _, c := range cards {
		list = append(list, c)
	}
	return newBoardIndex(now, time.UTC, cols, list), cards
}

// TestBoardIndexView 验证视图里的派生字段：列名、是否完成、是否在等别的卡、领取是否有效。
// 依赖指向已完成的卡、或者指向不存在的卡，都不算等待。
func TestBoardIndexView(t *testing.T) {
	ix, cards := viewFixture()
	cases := []struct {
		name                   string
		column                 string
		done, waiting, claimed bool
	}{
		{"open", "待办", false, false, false},
		{"finished", "完成", true, false, false},
		{"onOpen", "进行中", false, true, false},
		{"onDone", "进行中", false, false, false},
		{"onBoth", "待办", false, true, false},
		{"onMissing", "待办", false, false, false},
		{"claimed", "进行中", false, false, true},
		{"expired", "进行中", false, false, false},
	}
	for _, c := range cases {
		v := ix.view(cards[c.name])
		if v.Column != c.column || v.Done != c.done || v.Waiting != c.waiting || v.ClaimActive != c.claimed {
			t.Errorf("%s：期望 列=%s 完成=%v 等待=%v 领取有效=%v，实际 %+v", c.name, c.column, c.done, c.waiting, c.claimed, v)
		}
	}
}

// TestBoardIndexSortCards 验证显示顺序：先按列从左到右，同一列里按位置，位置相同按主键。
func TestBoardIndexSortCards(t *testing.T) {
	ix, cards := viewFixture()
	cards["open"].Position = cards["onBoth"].Position // 同一列同一位置，按主键排：onBoth(5) 在 open(1) 之后
	list := make([]*Card, 0, len(cards))
	for _, c := range cards {
		list = append(list, c)
	}

	var got []int64
	for _, c := range ix.sortCards(list) {
		got = append(got, c.ID)
	}
	want := []int64{1, 5, 6, 3, 4, 7, 8, 2}
	if len(got) != len(want) {
		t.Fatalf("期望 %v，实际 %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("期望 %v，实际 %v", want, got)
		}
	}
}

// TestClaimActive 验证领取是否有效：必须有持有者且租约晚于现在；刚好到期的那一刻算已过期。
func TestClaimActive(t *testing.T) {
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	later, same, earlier := now.Add(time.Second), now, now.Add(-time.Second)
	cases := []struct {
		name string
		card Card
		want bool
	}{
		{"没有持有者", Card{ClaimExpiresAt: &later}, false},
		{"没有到期时间", Card{ClaimedBy: "a"}, false},
		{"租约还有效", Card{ClaimedBy: "a", ClaimExpiresAt: &later}, true},
		{"刚好到期", Card{ClaimedBy: "a", ClaimExpiresAt: &same}, false},
		{"已经过期", Card{ClaimedBy: "a", ClaimExpiresAt: &earlier}, false},
	}
	for _, c := range cases {
		if got := claimActive(&c.card, now); got != c.want {
			t.Errorf("%s：期望 %v，实际 %v", c.name, c.want, got)
		}
	}
}
