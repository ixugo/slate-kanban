package kanban

import (
	"os"
	"strings"
	"testing"
)

// webTemplateTextFile 是网页里按界面语言换起步文案用的表，必须和 templateTexts 逐字一致。
const webTemplateTextFile = "../../../web/src/lib/templateText.ts"

// tsQuote 按网页源码里的写法给字符串加单引号。
func tsQuote(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}

// TestWebTemplateTextMatches 保证服务端起步模板的列名和规则名都出现在网页的对照表里；
// 改了这里的文案却忘了同步网页，网页就认不出旧看板里的文字，不会换语言。
func TestWebTemplateTextMatches(t *testing.T) {
	raw, err := os.ReadFile(webTemplateTextFile)
	if err != nil {
		t.Fatalf("读网页对照表失败: %v", err)
	}
	web := string(raw)
	for locale, texts := range templateTexts {
		var want []string
		for _, c := range texts.Columns {
			want = append(want, c.Name)
		}
		want = append(want, texts.RuleStart, texts.RuleFinish, texts.RuleReopen)
		for _, s := range want {
			if !strings.Contains(web, tsQuote(s)) {
				t.Errorf("%s 的文案 %q 不在 templateText.ts 里", locale, s)
			}
		}
	}
}
