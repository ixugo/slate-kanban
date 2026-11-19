package kanban

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// 五种界面语言，模板必须全部提供。
var supportedLocales = []string{"zh-CN", "zh-TW", "en", "ja", "ko"}

// TestNormalizeLocale 验证各种写法的语言标识都能归并到支持的五种，不认识的按 en。
func TestNormalizeLocale(t *testing.T) {
	cases := map[string]string{
		"zh-CN": "zh-CN", "zh_cn": "zh-CN", "zh": "zh-CN", "zh-Hans": "zh-CN", "ZH-SG": "zh-CN",
		"zh-TW": "zh-TW", "zh-HK": "zh-TW", "zh-Hant": "zh-TW", "zh-Hant-TW": "zh-TW",
		"en": "en", "en-US": "en", "": "en", "fr": "en", "xx-YY": "en",
		"ja": "ja", "ja-JP": "ja", "ko": "ko", "ko_KR": "ko",
	}
	for in, want := range cases {
		if got := normalizeLocale(in); got != want {
			t.Errorf("normalizeLocale(%q) = %q，期望 %q", in, got, want)
		}
	}
}

// TestTemplateTextsComplete 验证每种语言的文案齐全，并且都在数据库列长度和校验规则的上限之内，
// 同一种语言里模板各列的名称不重复（列名在看板里不区分大小写唯一）。
func TestTemplateTextsComplete(t *testing.T) {
	for _, locale := range supportedLocales {
		texts, ok := templateTexts[locale]
		if !ok {
			t.Fatalf("缺少语言 %s 的模板文案", locale)
		}
		seen := map[string]bool{}
		for kind, c := range texts.Columns {
			if c.Name == "" || c.Instruction == "" {
				t.Errorf("%s 第 %d 段列文案有空字段: %+v", locale, kind, c)
			}
			if utf8.RuneCountInString(c.Name) > 60 ||
				utf8.RuneCountInString(c.Instruction) > 4000 {
				t.Errorf("%s 第 %d 段列文案超过长度上限", locale, kind)
			}
			if key := strings.ToLower(c.Name); seen[key] {
				t.Errorf("%s 的列名重复: %s", locale, c.Name)
			} else {
				seen[key] = true
			}
		}
		for _, name := range []string{texts.RuleStart, texts.RuleFinish, texts.RuleReopen} {
			if name == "" || utf8.RuneCountInString(name) > 100 {
				t.Errorf("%s 的规则名称不合法: %q", locale, name)
			}
		}
	}
}

// TestSeedEngineerTexts 验证起步工程师的名称（英文名）和提示词（中文）合法、名称不重名。
func TestSeedEngineerTexts(t *testing.T) {
	names := map[string]bool{}
	for i, eng := range seedEngineerTexts {
		if eng.Name == "" || eng.Prompt == "" || utf8.RuneCountInString(eng.Name) > maxEngineerNameLen ||
			utf8.RuneCountInString(eng.Prompt) > maxEngineerPromptLen || names[strings.ToLower(eng.Name)] {
			t.Errorf("第 %d 位起步工程师文案不合法: %+v", i, eng)
		}
		names[strings.ToLower(eng.Name)] = true
	}
}

// TestTemplateColumns 验证模板结构：下一列的下标有效并且只指向右边（不会成环），
// 有 agent 领活的列和完成列都恰好各一个，完成列是最后一列。
func TestTemplateColumns(t *testing.T) {
	if len(templateColumns[TemplateEmpty]) != 0 {
		t.Fatal("empty 模板不应该有列")
	}
	for _, name := range []string{TemplateDev, TemplateSimple} {
		specs := templateColumns[name]
		pickups, dones := 0, 0
		for i, s := range specs {
			if s.Next >= len(specs) || (s.Next >= 0 && s.Next <= i) {
				t.Errorf("%s 第 %d 列的下一列下标不合法: %d", name, i, s.Next)
			}
			if s.Text == textDoing {
				pickups++
			}
			if s.IsDone {
				dones++
				if i != len(specs)-1 || s.Next != -1 {
					t.Errorf("%s 的完成列必须是最后一列且没有下一列", name)
				}
			}
		}
		if pickups != 1 || dones != 1 {
			t.Errorf("%s 应该恰好有一个领活列和一个完成列，实际 %d、%d", name, pickups, dones)
		}
	}
	if got := len(templateColumns[TemplateDev]); got != 5 {
		t.Errorf("dev 模板应该有 5 列，实际 %d", got)
	}
	if got := len(templateColumns[TemplateSimple]); got != 3 {
		t.Errorf("simple 模板应该有 3 列，实际 %d", got)
	}
}

// TestDefaultRulesValid 验证每种模板、每种语言生成的起步规则都能通过保存前校验，
// 并且数量和触发正好是：开始计时、完成计时、重新打开；empty 模板没有规则。
func TestDefaultRulesValid(t *testing.T) {
	for _, template := range []string{TemplateDev, TemplateSimple} {
		specs := templateColumns[template]
		cols := make([]*Column, len(specs))
		for i, s := range specs {
			cols[i] = &Column{ID: int64(i + 1), Name: textsFor("en").Columns[s.Text].Name, IsDone: s.IsDone}
		}
		for _, locale := range supportedLocales {
			for i, spec := range specs {
				cols[i].Name = textsFor(locale).Columns[spec.Text].Name
			}
			rules := defaultRules(textsFor(locale), cols)
			if len(rules) != 3 {
				t.Fatalf("%s/%s 应该有 3 条起步规则，实际 %d", template, locale, len(rules))
			}
			for _, r := range rules {
				if err := normalizeRule(r, cols); err != nil {
					t.Errorf("%s/%s 的规则 %q 校验失败: %v", template, locale, r.Name, err)
				}
			}
		}
	}
	if rules := defaultRules(textsFor("en"), nil); len(rules) != 0 {
		t.Fatalf("没有列就不应该有规则: %+v", rules)
	}
}

// TestDefaultRulesDoNotNeedBothColumns 验证只有领活列或只有完成列时，只生成对应的规则，不会因为缺列而出错。
func TestDefaultRulesDoNotNeedBothColumns(t *testing.T) {
	texts := textsFor("en")
	onlyDoing := defaultRules(texts, []*Column{{ID: 1, Name: texts.Columns[textDoing].Name}})
	if len(onlyDoing) != 1 || onlyDoing[0].Name != texts.RuleStart || onlyDoing[0].ColumnID != 1 {
		t.Fatalf("只有领活列应该只生成开始计时: %+v", onlyDoing)
	}
	onlyDone := defaultRules(texts, []*Column{{ID: 2, IsDone: true}})
	if len(onlyDone) != 2 || onlyDone[0].Name != texts.RuleFinish || onlyDone[1].Name != texts.RuleReopen {
		t.Fatalf("只有完成列应该生成完成计时和重新打开: %+v", onlyDone)
	}
}
