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
const maxInstructionRunes = 500

// toolWord 匹配说明里像工具名的词：whoami，或者以工具常用的动词开头的下划线词。
var toolWord = regexp.MustCompile(`\b(?:whoami|(?:list|get|set|enable|create|update|delete|move|claim|release|finish|comment|pull|wait_for|next|reorder)_[a-z_]+)\b`)

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
			[]string{"whoami", "list_boards", "get_board", "finish_card", "enable_management", "slate://skills/slate-kanban"},
			agent,
		},
		{
			"人", adminTools,
			[]string{"get_board", "slate://skills/slate-kanban"},
			s.human(t).InitializeResult().Instructions,
		},
	}
	for _, want := range []string{
		"先只读拟定方案", "评论成功后", "用户明确要求", "由监听器触发的一轮调度",
		"Python 监听脚本 slate://scripts/slate-watch.py", "看板的 watch_prompt",
	} {
		if !strings.Contains(agent, want) {
			t.Errorf("agent 初始化说明缺少 %q", want)
		}
	}
	for _, tc := range cases {
		if tc.text == "" || utf8.RuneCountInString(tc.text) > maxInstructionRunes {
			t.Errorf("%s：使用说明不能为空，也不能超过 %d 字，实际 %d 字", tc.name, maxInstructionRunes, utf8.RuneCountInString(tc.text))
		}
		for _, want := range []string{
			"用 Markdown 写卡片描述和评论", "短段落或列表", "**加粗重点与待决事项**",
			"1. 短句，一句一事", "规则编号", "删开篇多余依据、套话、常识和重复",
			"2. 需求先写用户何时能做什么、做完看到什么",
			"3. 限量时写谁来计数、怎么算、到上限会怎样",
			"4. 复用时写明对象和接法", "字段、接口和验收写到可实施",
			"5. Git 提交哈希只写前 6 位",
		} {
			if !strings.Contains(tc.text, want) {
				t.Errorf("%s：卡片写作说明缺少 %q", tc.name, want)
			}
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
