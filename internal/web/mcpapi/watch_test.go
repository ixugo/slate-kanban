package mcpapi

import (
	"strings"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// TestSlotsText 验证并发名额的算法：研发类（全栈、前端、后端、UI）和测试各算各的，
// 某一类一位都没有时按 1 个算，和 skill 第三节一致。
func TestSlotsText(t *testing.T) {
	eng := func(name, kind string) *kanban.Engineer { return &kanban.Engineer{Name: name, Kind: kind} }
	cases := []struct {
		name      string
		engineers []*kanban.Engineer
		want      []string
	}{
		{
			"默认名册：两位全栈加一位测试",
			[]*kanban.Engineer{eng("Atlas", kanban.EngineerDev), eng("Orion", kanban.EngineerDev), eng("Sentinel", kanban.EngineerReview)},
			[]string{"2 位研发工程师（Atlas、Orion）", "1 位测试工程师（Sentinel）", "最多同时做 2 个研发任务、同时审 1 个审查任务", "共 3 个子 agent 并行"},
		},
		{
			"前端、后端、UI 都算研发，两位测试各占一个审查名额",
			[]*kanban.Engineer{
				eng("A", kanban.EngineerFrontend), eng("B", kanban.EngineerBackend), eng("C", kanban.EngineerUI),
				eng("D", kanban.EngineerDev), eng("T1", kanban.EngineerReview), eng("T2", kanban.EngineerReview),
			},
			[]string{"4 位研发工程师（A、B、C、D）", "2 位测试工程师（T1、T2）", "最多同时做 4 个研发任务、同时审 2 个审查任务", "共 6 个子 agent 并行"},
		},
		{
			"没有测试工程师时审查按 1 个算",
			[]*kanban.Engineer{eng("Atlas", kanban.EngineerDev), eng("Orion", kanban.EngineerDev)},
			[]string{"2 位研发工程师（Atlas、Orion）", "没有测试工程师", "最多同时做 2 个研发任务、同时审 1 个审查任务", "共 3 个子 agent 并行"},
		},
		{
			"只有测试工程师时研发按 1 个算",
			[]*kanban.Engineer{eng("Sentinel", kanban.EngineerReview)},
			[]string{"没有研发工程师", "1 位测试工程师（Sentinel）", "最多同时做 1 个研发任务、同时审 1 个审查任务", "共 2 个子 agent 并行"},
		},
		{
			"名册为空时研发和审查各按 1 个算",
			nil,
			[]string{"没有研发工程师，没有测试工程师", "最多同时做 1 个研发任务、同时审 1 个审查任务"},
		},
	}
	for _, tc := range cases {
		got := slotsText(tc.engineers)
		for _, want := range tc.want {
			if !strings.Contains(got, want) {
				t.Errorf("%s：并发名额说明缺少 %q，实际: %s", tc.name, want, got)
			}
		}
	}
}

// TestWatchPromptOf 验证守候提示词：填好了看板名和并发名额，没有残留的格式占位符；
// 并写明一次领满、同一次回复里同时派出、一个任务一个新子 agent 这几条并发纪律。
func TestWatchPromptOf(t *testing.T) {
	board := &kanban.Board{Name: "发布 100%", Engineers: []*kanban.Engineer{
		{Name: "Atlas", Kind: kanban.EngineerDev}, {Name: "Orion", Kind: kanban.EngineerDev},
	}}
	got := watchPromptOf(&kanban.BoardDetail{Board: board})
	for _, want := range []string{
		"「发布 100%」", "2 位研发工程师（Atlas、Orion）", "研发和审查的名额各算各的，互不占用",
		"slate://scripts/slate-watch.py", "--host", "--command-json", "opencode", "on-my-pi",
		"一次领满", "在同一次回复里把这几个子 agent 一起派出", "不要领一个、等它做完再领下一个",
		"一个任务新开一个子 agent", "不要把另一个任务交给做过别的任务的子 agent",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("守候提示词缺少 %q", want)
		}
	}
	if strings.Contains(got, "%!") || strings.Contains(got, "%s") {
		t.Errorf("守候提示词有没填好的格式占位符: %s", got)
	}
}
