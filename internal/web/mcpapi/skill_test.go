package mcpapi_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ixugo/goddd/pkg/reason"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/skills"
)

const (
	// skillName 是 skill 的名字，也是它在 host 的 skills 目录里的文件夹名，安装说明里的命令依赖它。
	skillName = "slate-kanban"
	// maxSkillDescriptionRunes 是 skill 简介的字数上限：host 靠简介判断什么时候加载 skill，
	// 常见 host 的上限是 1024 字。
	maxSkillDescriptionRunes = 1024
	// skillResourceURI 是 skill 作为 MCP 资源的固定地址。
	skillResourceURI = "slate://skills/slate-kanban"
)

// skillErrors 是 skill 里允许出现的错误名。这里用真实的错误变量取名字：服务端把错误改名时，
// 这里会编译不过，提醒同步更新 skill；skill 里新写了一个这里没有的错误名，下面的测试会失败，
// 提醒确认它是真的。
var skillErrors = []string{
	kanban.ErrNotHolder.GetReason(),
	kanban.ErrCardVersionConflict.GetReason(),
	kanban.ErrProjectPathLocked.GetReason(),
	kanban.ErrCardClaimed.GetReason(),
	kanban.ErrCardWaiting.GetReason(),
	kanban.ErrCardHumanOnly.GetReason(),
	kanban.ErrEngineerBusy.GetReason(),
	kanban.ErrNoIdleEngineer.GetReason(),
	kanban.ErrEngineerBound.GetReason(),
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

// TestSkillResource 验证 skill 资源：两个视图都能在资源列表里看到它，读出来的正是编译进去的 skill 全文，
// agent 没装 skill 的时候，靠它自己取到流程说明。
func TestSkillResource(t *testing.T) {
	s := newStack(t)
	for name, sess := range map[string]*mcp.ClientSession{"agent": s.agent(t, "claude"), "人": s.human(t)} {
		list, err := sess.ListResources(t.Context(), nil)
		if err != nil || !slices.ContainsFunc(list.Resources, func(r *mcp.Resource) bool {
			return r.URI == skillResourceURI && r.MIMEType == "text/markdown"
		}) {
			t.Fatalf("%s：资源列表不对: %+v %v", name, list, err)
		}
		res, err := sess.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: skillResourceURI})
		if err != nil || len(res.Contents) != 1 {
			t.Fatalf("%s：读取 skill 失败: %+v %v", name, res, err)
		}
		got := res.Contents[0]
		if got.URI != skillResourceURI || got.MIMEType != "text/markdown" || got.Text != skills.SlateKanban() {
			t.Errorf("%s：读到的 skill 地址、类型或内容不对: %s %s", name, got.URI, got.MIMEType)
		}
		_, review, _ := strings.Cut(got.Text, "## 五、派审查")
		review, _, _ = strings.Cut(review, "## 六、")
		for _, want := range []string{"回传实际效果截图", "upload_image", "comment_card", "- [x] 项目与证据", "- [ ] 项目与原因", "暂不判通过"} {
			if !strings.Contains(review, want) {
				t.Errorf("%s：skill 的审查证据要求缺少 %q", name, want)
			}
		}
	}
}

func TestWatchScriptResource(t *testing.T) {
	s := newStack(t)
	sess := s.agent(t, "codex")
	list, err := sess.ListResources(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, uri := range []string{skills.WatchURI, skills.CodexWatchURI} {
		if !slices.ContainsFunc(list.Resources, func(r *mcp.Resource) bool {
			return r.URI == uri && r.MIMEType == "text/x-python"
		}) {
			t.Fatalf("资源列表里缺少 %s", uri)
		}
		res, err := sess.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: uri})
		if err != nil || len(res.Contents) != 1 {
			t.Fatalf("读取 Python 监听脚本失败: %+v %v", res, err)
		}
		got := res.Contents[0]
		if got.URI != uri || got.MIMEType != "text/x-python" || got.Text != skills.WatchScript() ||
			!strings.Contains(got.Text, "SLATE_DISPATCH_DONE") || !strings.Contains(got.Text, "--command-json") {
			t.Fatalf("返回的资源没有包含随服务发布的监听脚本: %s %s", got.URI, got.MIMEType)
		}
	}
}
