package kanban

import (
	"errors"
	"slices"
	"testing"
)

// depCard 构造一张只关心主键、依赖和所在列的任务。
func depCard(id, columnID int64, waitsOn ...int64) *Card {
	return &Card{ID: id, ColumnID: columnID, WaitsOn: waitsOn}
}

// TestNormalizeWaitsOn 验证依赖整理：去重并保留首次出现的顺序，nil 也得到空数组，
// 超过 20 个或者出现非正数 id 返回 ErrInvalidInput，并且不改动传入的切片。
func TestNormalizeWaitsOn(t *testing.T) {
	in := []int64{3, 1, 3, 2, 1}
	got, err := normalizeWaitsOn(in)
	if err != nil || !slices.Equal(got, []int64{3, 1, 2}) {
		t.Fatalf("应该去重并保留顺序，实际 %v, %v", got, err)
	}
	if !slices.Equal(in, []int64{3, 1, 3, 2, 1}) {
		t.Fatalf("不应该改动传入的切片: %v", in)
	}
	if got, err := normalizeWaitsOn(nil); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("nil 应该得到空数组，实际 %v, %v", got, err)
	}

	many := make([]int64, maxWaitsOn+1)
	for i := range many {
		many[i] = int64(i + 1)
	}
	if _, err := normalizeWaitsOn(many); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("超过上限应该得到 ErrInvalidInput，实际 %v", err)
	}
	if got, err := normalizeWaitsOn(many[:maxWaitsOn]); err != nil || len(got) != maxWaitsOn {
		t.Fatalf("刚好等于上限应该通过，实际 %v, %v", len(got), err)
	}
	for _, bad := range [][]int64{{0}, {-1}, {1, 0}} {
		if _, err := normalizeWaitsOn(bad); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%v 应该得到 ErrInvalidInput，实际 %v", bad, err)
		}
	}
}

// TestValidateWaitsOn 验证依赖校验：不能指向自己、不能指向不存在或别的看板的任务、
// 不能成环（直接环和间接环）；菱形依赖和空依赖是合法的。
func TestValidateWaitsOn(t *testing.T) {
	// 1 等 2，2 等 3，3 没有依赖；4 等 2 和 3（菱形）。
	board := []*Card{depCard(1, 1, 2), depCard(2, 1, 3), depCard(3, 1), depCard(4, 1, 2, 3)}
	cases := []struct {
		name   string
		cardID int64
		deps   []int64
		ok     bool
	}{
		{"没有依赖", 3, nil, true},
		{"依赖链上更靠前的卡", 4, []int64{1, 2, 3}, true},
		{"依赖一张已经在等自己的卡", 3, []int64{4}, false},
		{"菱形依赖", 1, []int64{2, 3}, true},
		{"指向自己", 3, []int64{3}, false},
		{"指向不存在的任务", 3, []int64{99}, false},
		{"直接成环", 2, []int64{1}, false},
		{"间接成环", 3, []int64{1}, false},
		{"间接成环（经过多张卡）", 3, []int64{4, 1}, false},
		{"新建任务没有人依赖它，不可能成环", 0, []int64{1, 2}, true},
		{"新建任务依赖不存在的任务", 0, []int64{99}, false},
	}
	for _, c := range cases {
		err := validateWaitsOn(c.cardID, c.deps, board)
		if (err == nil) != c.ok {
			t.Errorf("%s：期望通过=%v，实际 %v", c.name, c.ok, err)
		}
		if err != nil && !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s：应该是 ErrInvalidInput，实际 %v", c.name, err)
		}
	}
}

// TestValidateWaitsOnLongChain 验证很长的依赖链不会栈溢出，也能正确发现链尾指回链头的环。
func TestValidateWaitsOnLongChain(t *testing.T) {
	const n = 20000
	board := make([]*Card, n)
	for i := range n {
		id := int64(i + 1)
		board[i] = depCard(id, 1)
		if i > 0 {
			board[i].WaitsOn = []int64{id - 1}
		}
	}
	if err := validateWaitsOn(1, []int64{n}, board); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("链头依赖链尾应该成环，实际 %v", err)
	}
	if err := validateWaitsOn(n, []int64{1}, board); err != nil {
		t.Fatalf("链尾依赖链头只是多一条边，不成环，实际 %v", err)
	}
}

// TestUnfinishedDeps 验证等待判断：依赖在完成列里就算做完，不在完成列里就算没完成，
// 依赖的任务已经不存在则忽略。
func TestUnfinishedDeps(t *testing.T) {
	const todo, done int64 = 1, 2
	cards := []*Card{depCard(10, todo, 11, 12, 13, 99), depCard(11, todo), depCard(12, done), depCard(13, todo)}
	byID := map[int64]*Card{}
	for _, c := range cards {
		byID[c.ID] = c
	}
	got := unfinishedDeps(cards[0], byID, map[int64]bool{done: true})
	if len(got) != 2 || got[0].ID != 11 || got[1].ID != 13 {
		t.Fatalf("应该还在等 11 和 13，实际 %+v", got)
	}
	if got := unfinishedDeps(cards[1], byID, map[int64]bool{done: true}); len(got) != 0 {
		t.Fatalf("没有依赖就不在等待: %+v", got)
	}
	if got := unfinishedDeps(cards[0], byID, map[int64]bool{todo: true, done: true}); len(got) != 0 {
		t.Fatalf("所有依赖都在完成列就不在等待: %+v", got)
	}
}
