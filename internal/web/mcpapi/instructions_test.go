package mcpapi_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// maxInstructionRunes 是使用说明的字数上限：它在每个会话开始时都会进入模型的上下文，
// 所以要短，完整的流程放在 skill 里。
const maxInstructionRunes = 1500

// toolWord 匹配说明里像工具名的词：whoami，或者以工具常用的动词开头的下划线词。
var toolWord = regexp.MustCompile(`\b(?:whoami|(?:list|get|set|create|update|delete|move|claim|release|finish|comment|pull|wait_for|next|reorder)_[a-z_]+)\b`)

// notTools 是说明里出现的、长得像工具名但其实是字段名的词。
var notTools = []string{"next_column_id", "claim_engineer_id"}

// TestInstructions 验证两个视图都在连接时下发了使用说明：不能太长；里面提到的工具必须真的存在于这个视图
// （写错或者提到别的视图的工具，agent 就会去调用不存在的工具）；关键的工作流程工具一个不少。
func TestInstructions(t *testing.T) {
	s := newStack(t)
	agent := s.agent(t, "claude").InitializeResult().Instructions
	cases := []struct {
		name     string
		tools    []string
		required []string
		text     string
	}{
		{
			"agent", agentTools,
			[]string{"whoami", "list_boards", "get_board", "set_project_path", "next_card", "comment_card", "create_card", "finish_card", "release_card", "wait_for_updates", "pull_updates", "list_cards", "slate://skills/slate-kanban"},
			agent,
		},
		{
			"人", adminTools,
			[]string{"create_rule", "update_column"},
			s.human(t).InitializeResult().Instructions,
		},
	}
	if !strings.Contains(agent, "先只读拟定方案") || !strings.Contains(agent, "评论成功后") {
		t.Errorf("agent 初始化说明应该要求调度者先记录方案再让子 agent 开工: %q", agent)
	}
	if !strings.Contains(agent, "仍有空位必须查待办") || !strings.Contains(agent, "只表示当前列") {
		t.Errorf("agent 初始化说明应该要求查完可办列及待办再等待: %q", agent)
	}
	if !strings.Contains(agent, "不触发终端阻塞式提问") || !strings.Contains(agent, "路径为空且经人明确确认") {
		t.Errorf("agent 初始化说明应该在看板澄清并确认路径: %q", agent)
	}
	for _, want := range []string{"已有的标签", "不要新建", "任务项"} {
		if !strings.Contains(agent, want) {
			t.Errorf("agent 初始化说明应该提醒标签只用已有的、交接前补任务项，缺少 %q", want)
		}
	}
	for _, tc := range cases {
		if tc.text == "" || utf8.RuneCountInString(tc.text) > maxInstructionRunes {
			t.Errorf("%s：使用说明不能为空，也不能超过 %d 字，实际 %d 字", tc.name, maxInstructionRunes, utf8.RuneCountInString(tc.text))
		}
		for _, word := range toolWord.FindAllString(tc.text, -1) {
			if !slices.Contains(tc.tools, word) && !slices.Contains(notTools, word) {
				t.Errorf("%s：说明里提到的 %s 不是这个视图里的工具", tc.name, word)
			}
		}
		for _, word := range tc.required {
			if !regexp.MustCompile(`\b` + word + `\b`).MatchString(tc.text) {
				t.Errorf("%s：说明里应该提到 %s", tc.name, word)
			}
		}
	}
}
