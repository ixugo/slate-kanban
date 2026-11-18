package mcpapi_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ixugo/goddd/pkg/reason"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/skills"
)

const (
	// skillName 是 skill 的名字，也是它在 host 的 skills 目录里的文件夹名，安装说明里的命令依赖它。
	skillName = "slate-kanban"
	// maxSkillDescriptionRunes 是 skill 简介的字数上限：host 靠简介判断什么时候加载 skill，
	// 常见 host 的上限是 1024 字。
	maxSkillDescriptionRunes = 1024
)

// skillErrors 是 skill 里允许出现的错误名。这里用真实的错误变量取名字：服务端把错误改名时，
// 这里会编译不过，提醒同步更新 skill；skill 里新写了一个这里没有的错误名，下面的测试会失败，
// 提醒确认它是真的。
var skillErrors = []string{
	kanban.ErrNotHolder.GetReason(),
	kanban.ErrCardVersionConflict.GetReason(),
	kanban.ErrAgentPickupColumn.GetReason(),
	kanban.ErrColumnNotPickup.GetReason(),
	kanban.ErrProjectPathLocked.GetReason(),
	kanban.ErrCardClaimed.GetReason(),
	kanban.ErrCardWaiting.GetReason(),
	kanban.ErrCardHumanOnly.GetReason(),
	reason.ErrServiceUnavailable.GetReason(),
}

// errWord 匹配文字里的错误名。
var errWord = regexp.MustCompile(`\bErr[A-Z][A-Za-z]+\b`)

// frontmatter 取出 skill 开头 --- 之间的 key: value，和后面的正文。
func frontmatter(t *testing.T, text string) (map[string]string, string) {
	t.Helper()
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		t.Fatalf("skill 必须以 --- 开头的 frontmatter 起头")
	}
	head, body, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		t.Fatalf("skill 的 frontmatter 没有用 --- 结束")
	}
	fields := map[string]string{}
	for _, line := range strings.Split(head, "\n") {
		key, value, ok := strings.Cut(line, ": ")
		if !ok {
			t.Fatalf("frontmatter 的这一行不是 key: value: %q", line)
		}
		fields[key] = value
	}
	return fields, body
}

// TestSkillFrontmatter 验证 skill 的 frontmatter：名字固定，简介不能空也不能太长，没有多余的字段。
// host 靠 name 和 description 决定要不要加载它，写错了 skill 就等于没装。
func TestSkillFrontmatter(t *testing.T) {
	fields, body := frontmatter(t, skills.SlateKanban())
	if fields["name"] != skillName {
		t.Errorf("name 应该是 %s，实际是 %q", skillName, fields["name"])
	}
	if n := utf8.RuneCountInString(fields["description"]); n == 0 || n > maxSkillDescriptionRunes {
		t.Errorf("description 不能为空，也不能超过 %d 字，实际 %d 字", maxSkillDescriptionRunes, n)
	}
	if len(fields) != 2 {
		t.Errorf("frontmatter 只应该有 name 和 description，实际: %v", fields)
	}
	if strings.TrimSpace(body) == "" {
		t.Errorf("skill 没有正文")
	}
}

// TestSkillMatchesTools 验证 skill 和 agent 能用的工具对得上：
// 每个工具 skill 里都讲了什么时候用（新增工具没有写进 skill 会失败）；
// skill 里长得像工具名的词都是 agent 视图里真实存在的工具（拼错、改名、提到只有人能用的工具都会失败）。
func TestSkillMatchesTools(t *testing.T) {
	text := skills.SlateKanban()
	for _, tool := range agentTools {
		if !regexp.MustCompile(`\b` + tool + `\b`).MatchString(text) {
			t.Errorf("skill 里没有提到工具 %s", tool)
		}
	}
	for _, word := range toolWord.FindAllString(text, -1) {
		if !slices.Contains(agentTools, word) && !slices.Contains(notTools, word) {
			t.Errorf("skill 里提到的 %s 不是 agent 视图里的工具", word)
		}
	}
}

// TestSkillErrorNames 验证 skill 里写的错误名都是服务端真实会返回的，
// agent 是按这些名字查"该怎么办"的，写错了它就查不到。
func TestSkillErrorNames(t *testing.T) {
	text := skills.SlateKanban()
	found := errWord.FindAllString(text, -1)
	if len(found) == 0 {
		t.Fatalf("skill 里应该有错误对照表")
	}
	for _, word := range found {
		if !slices.Contains(skillErrors, word) {
			t.Errorf("skill 里的 %s 不是已登记的服务端错误", word)
		}
	}
}
