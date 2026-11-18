package connectapi

import (
	"slices"
	"strings"
	"testing"
)

// TestRenderMarkdown 验证 Markdown 安装说明：四个步骤按顺序出现；每个 host 的名字和每一段命令都在里面；
// 代码围栏成对（少一个会让后面的文字全被当成代码）；没有没替换掉的占位符或格式化残留。
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
		if fences := strings.Count(md, "\n```"); fences%2 != 0 {
			t.Errorf("代码围栏不成对: %d 个", fences)
		}
		for _, bad := range []string{"%!", "{base}", "<nil>", "%s", "%v"} {
			if strings.Contains(md, bad) {
				t.Errorf("说明里有残留的 %q", bad)
			}
		}
	}
}
