package connectapi

import (
	"slices"
	"strings"
	"testing"
)

// TestRenderMarkdown 验证 Markdown 安装说明：四个步骤按顺序出现；每个 host 的名字和每一段命令都在里面；
// 代码围栏一开一关交替出现（少一个关闭的，后面的文字就全被当成代码）；没有没替换掉的占位符或格式化残留。
func TestRenderMarkdown(t *testing.T) {
	for _, base := range guideBases {
		g := buildGuide(base, false)
		md := renderMarkdown(g, false)

		if !strings.HasPrefix(md, "# ") || !strings.HasSuffix(md, "\n") {
			t.Errorf("说明应该以一级标题开头、以换行结尾")
		}
		for _, want := range []string{g.BaseURL, g.MCPURL, g.InstallURL, "?name="} {
			if !strings.Contains(md, want) {
				t.Errorf("说明里应该有 %q", want)
			}
		}
		last := -1
		for _, heading := range []string{"## 1. ", "## 2. ", "## 3. ", "## 4. "} {
			at := strings.Index(md, heading)
			if at <= last {
				t.Errorf("步骤 %q 缺失或顺序不对", heading)
			}
			last = at
		}
		for _, h := range g.Hosts {
			if strings.Count(md, "### "+h.Name+"\n") != 3 {
				t.Errorf("host %s 应该在安装 MCP、安装 skill 和长期挂着三节各出现一次", h.Name)
			}
			for _, sn := range slices.Concat(h.MCP, h.Skill, h.Watch) {
				if !strings.Contains(md, sn.Code) || !strings.Contains(md, sn.Title) {
					t.Errorf("host %s 的步骤没有写进说明: %s", h.Name, sn.Title)
				}
			}
		}
		for _, want := range []string{g.WatchPrompt, g.LoopPrompt, boardPlaceholder, "wait_for_updates"} {
			if want == "" || !strings.Contains(md, want) {
				t.Errorf("长期挂着一节缺少 %q", want)
			}
		}
		assertFencesAlternate(t, md)
		for _, bad := range []string{"%!", "{base}", "<nil>", "%s", "%v"} {
			if strings.Contains(md, bad) {
				t.Errorf("说明里有残留的 %q", bad)
			}
		}
	}
}

// assertFencesAlternate 验证代码围栏严格地一开一关交替出现：开的那行带语言，关的那行只有三个反引号，
// 最后必须是关闭状态。只数个数不够：同时少了偶数个关闭围栏时，个数仍然是偶数。
func assertFencesAlternate(t *testing.T, md string) {
	t.Helper()
	open := false
	for i, line := range strings.Split(md, "\n") {
		if !strings.HasPrefix(line, "```") {
			continue
		}
		switch {
		case !open && line != "```":
			open = true
		case open && line == "```":
			open = false
		default:
			t.Fatalf("第 %d 行的代码围栏不对（open=%v）: %q", i+1, open, line)
		}
	}
	if open {
		t.Errorf("最后一个代码围栏没有关闭")
	}
}

// TestRenderMarkdownVerifyAndFAQ 验证步骤 3 按 host 写明终端里怎么检查，常见问题写明各 host 在真实环境里的表现；
// 口令相关的现象只在服务启用口令时才写。
func TestRenderMarkdownVerifyAndFAQ(t *testing.T) {
	g := buildGuide(guideBases[0], false)
	plain := renderMarkdown(g, false)
	verify, faq, ok := strings.Cut(plain[strings.Index(plain, "## 3. "):], "## 常见问题")
	if !ok {
		t.Fatalf("缺少常见问题一节")
	}
	for _, want := range []string{"新开一个会话", "claude mcp list", "✔ Connected", "codex mcp list", "Unsupported",
		"agent mcp list", "ready", "agent mcp enable", "重启 Cursor"} {
		if !strings.Contains(verify, want) {
			t.Errorf("验证一节应该有 %q", want)
		}
	}
	if !strings.Contains(faq, "?name=") {
		t.Errorf("常见问题应该解释 Claude Code 打印的地址不带 ?name=")
	}
	for _, unwanted := range []string{"401", "AuthRequired"} {
		if strings.Contains(faq, unwanted) {
			t.Errorf("未启用口令时常见问题不该提 %q", unwanted)
		}
	}

	guarded := renderMarkdown(buildGuide(guideBases[0], true), true)
	for _, want := range []string{"401", "Failed to connect", "Error: Connection failed", "AuthRequired", "${env:SLATE_PASSCODE}"} {
		if !strings.Contains(guarded[strings.Index(guarded, "## 常见问题"):], want) {
			t.Errorf("启用口令时常见问题应该有 %q", want)
		}
	}
}
