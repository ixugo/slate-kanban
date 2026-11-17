package kanban

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

// filterNow 是筛选测试里的"现在"：2026-11-17 中午 12 点（UTC）。
var filterNow = time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)

// day 返回 UTC 的某一天某个时刻，让测试数据读起来像日历。
func day(month time.Month, d, hour int) time.Time {
	return time.Date(2026, month, d, hour, 0, 0, 0, time.UTC)
}

// ptr 返回时间的指针。
func ptr(t time.Time) *time.Time { return &t }

// filterFixture 构造筛选测试用的看板：三列（待办、进行中、完成），五张各有特点的卡片。
// loc 是日期类筛选使用的时区。
func filterFixture(loc *time.Location) (*boardIndex, []*Card) {
	cols := []*Column{{ID: 10, Name: "待办"}, {ID: 20, Name: "进行中"}, {ID: 30, Name: "完成", IsDone: true}}
	cards := []*Card{
		{
			ID: 1, Key: "K-1", Title: "修复登录 Bug", Description: "用户无法登录", ColumnID: 10, Priority: 3,
			Tags: []string{"bug", "urgent"}, DueAt: ptr(day(11, 16, 9)), Branch: "fix/login", CreatedAt: day(11, 1, 9), UpdatedAt: day(11, 1, 9),
		},
		{
			ID: 2, Key: "K-2", Title: "Add Dark Mode", Description: "support theme", ColumnID: 20, Priority: 2,
			Tags: []string{"feature"}, DueAt: ptr(day(11, 17, 18)), StartedAt: ptr(day(11, 12, 9)),
			ClaimedBy: "claude", ClaimExpiresAt: ptr(day(11, 17, 13)), CreatedAt: day(11, 10, 9), UpdatedAt: day(11, 10, 9),
		},
		{
			ID: 3, Key: "K-3", Title: "写文档", ColumnID: 30, DueAt: ptr(day(11, 1, 9)), FinishedAt: ptr(day(11, 17, 9)),
			URL: "https://x/1", CreatedAt: day(10, 20, 9), UpdatedAt: day(11, 17, 9),
		},
		{
			ID: 4, Key: "K-4", Title: "重构 API", ColumnID: 10, Priority: 1, Tags: []string{"Feature", "backend"},
			HumanOnly: true, WaitsOn: []int64{1}, CreatedAt: day(11, 15, 9), UpdatedAt: day(11, 15, 9),
		},
		{
			ID: 5, Key: "K-5", Title: "发布", ColumnID: 20, WaitsOn: []int64{3},
			ClaimedBy: "codex", ClaimExpiresAt: ptr(day(11, 17, 11)), CreatedAt: day(11, 17, 8), UpdatedAt: day(11, 17, 8),
		},
	}
	return newBoardIndex(filterNow, loc, cols, cards), cards
}

// runFilter 解析筛选并返回命中卡片的主键，按卡片主键升序。
func runFilter(q string, ix *boardIndex, cards []*Card) ([]int64, error) {
	f, err := parseFilter(q)
	if err != nil {
		return nil, err
	}
	hits := []int64{}
	for _, c := range cards {
		if f.match(c, ix) {
			hits = append(hits, c.ID)
		}
	}
	return hits, nil
}

// TestFilterMatches 验证每一种筛选写法命中的卡片：普通文字、各种前缀、取反、引号、组合。
func TestFilterMatches(t *testing.T) {
	ix, cards := filterFixture(time.UTC)
	cases := []struct {
		q    string
		want []int64
	}{
		{"", []int64{1, 2, 3, 4, 5}},
		{"   ", []int64{1, 2, 3, 4, 5}},
		// 普通文字：标题、描述、编号、标签，不区分大小写
		{"登录", []int64{1}},
		{"dark", []int64{2}},
		{"theme", []int64{2}},
		{"k-3", []int64{3}},
		{"BUG", []int64{1}},
		{"backend", []int64{4}},
		{"不存在的词", []int64{}},
		// 标签、列、编号、标题、描述
		{"tag:feature", []int64{2, 4}},
		{"-tag:feature", []int64{1, 3, 5}},
		{"col:待办", []int64{1, 4}},
		{"column:进行中", []int64{2, 5}},
		{"col:20", []int64{2, 5}},
		{`col:"进行中"`, []int64{2, 5}},
		{"COL:完成", []int64{3}},
		{"key:k-2", []int64{2}},
		{"title:api", []int64{4}},
		{"desc:theme", []int64{2}},
		{"title:theme", []int64{}},
		// 领取
		{"claimed:claude", []int64{2}},
		{"claimed:codex", []int64{}},
		// 引号和取反
		{`"add dark"`, []int64{2}},
		{`"Dark Mode"`, []int64{2}},
		{`-"dark mode"`, []int64{1, 3, 4, 5}},
		{`"is:done"`, []int64{}},
		{`"-bug"`, []int64{}},
		{"12:30", []int64{}},
		{"-", []int64{1, 2, 3, 4, 5}},
		{`""`, []int64{1, 2, 3, 4, 5}},
		// 多个词都满足才命中
		{"col:待办 col:进行中", []int64{}},
	}
	for _, c := range cases {
		got, err := runFilter(c.q, ix, cards)
		if err != nil || !slices.Equal(got, c.want) {
			t.Errorf("筛选 %q：期望 %v，实际 %v，错误 %v", c.q, c.want, got, err)
		}
	}
}

// TestFilterErrors 验证错误：未知前缀、未知取值、空值、不合法的优先级和日期、引号没闭合、过长、过多、非法字符。
// 错误都是 ErrInvalidInput，并且提示里写出了出错的词。
func TestFilterErrors(t *testing.T) {
	ix, cards := filterFixture(time.UTC)
	cases := []struct {
		q    string
		must []string // 错误提示里必须出现的内容
	}{
		{"foo:bar", []string{"foo:bar", "foo", "双引号"}},
		{"https://example.com", []string{"https://example.com", "双引号"}},
		{"tag:", []string{"tag:", "不能为空"}},
		{`tag:"没有闭合`, []string{"tag:没有闭合", "双引号"}},
		{`"abc`, []string{"abc", "双引号"}},
		{strings.Repeat("a", maxFilterLen+1), []string{"筛选条件"}},
		{strings.Repeat("a ", maxFilterTerms+1), []string{"最多"}},
		{"a\x00b", []string{"空字符"}},
	}
	for _, c := range cases {
		_, err := runFilter(c.q, ix, cards)
		if !errors.Is(err, ErrInvalidInput) {
			t.Errorf("筛选 %q：应该得到 ErrInvalidInput，实际 %v", c.q, err)
			continue
		}
		for _, want := range c.must {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("筛选 %q：错误提示里应该有 %q，实际 %q", c.q, want, err.Error())
			}
		}
	}
}

// TestFilterBoundaryLengths 验证长度和词数的边界：刚好等于上限可以通过。
func TestFilterBoundaryLengths(t *testing.T) {
	ix, cards := filterFixture(time.UTC)
	if _, err := runFilter(strings.Repeat("a", maxFilterLen), ix, cards); err != nil {
		t.Errorf("刚好 %d 个字符应该通过: %v", maxFilterLen, err)
	}
	if _, err := runFilter(strings.TrimSpace(strings.Repeat("a ", maxFilterTerms)), ix, cards); err != nil {
		t.Errorf("刚好 %d 个词应该通过: %v", maxFilterTerms, err)
	}
}
