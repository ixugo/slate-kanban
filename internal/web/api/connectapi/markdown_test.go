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
		g := buildGuide(base)
		md := renderMarkdown(g)

		if !strings.HasPrefix(md, "# ") || !strings.HasSuffix(md, "\n") {
			t.Errorf("说明应该以一级标题开头、以换行结尾")
		}
		for _, want := range []string{g.BaseURL, g.MCPURL, g.InstallURL, "export SLATE_TOKEN="} {
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
			if strings.Count(md, "### "+h.Name+"\n") != 2 {
				t.Errorf("host %s 应该在安装 MCP 和安装 skill 两节各出现一次", h.Name)
			}
			for _, sn := range slices.Concat(h.MCP, h.Skill) {
				if !strings.Contains(md, sn.Code) || !strings.Contains(md, sn.Title) {
					t.Errorf("host %s 的步骤没有写进说明: %s", h.Name, sn.Title)
				}
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
