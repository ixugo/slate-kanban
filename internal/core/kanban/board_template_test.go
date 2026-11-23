package kanban

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// 四种界面语言，模板必须全部提供。
var supportedLocales = []string{"zh-CN", "zh-TW", "en", "ja"}

// TestNormalizeLocale 验证各种写法的语言标识都能归并到支持的五种，不认识的按 en。
func TestNormalizeLocale(t *testing.T) {
	cases := map[string]string{
		"zh-CN": "zh-CN", "zh_cn": "zh-CN", "zh": "zh-CN", "zh-Hans": "zh-CN", "ZH-SG": "zh-CN",
		"zh-TW": "zh-TW", "zh-HK": "zh-TW", "zh-Hant": "zh-TW", "zh-Hant-TW": "zh-TW",
		"en": "en", "en-US": "en", "": "en", "fr": "en", "xx-YY": "en",
		"ja": "ja", "ja-JP": "ja", "ko": "en", "ko_KR": "en",
	}
	for in, want := range cases {
		if got := NormalizeLocale(in); got != want {
			t.Errorf("NormalizeLocale(%q) = %q，期望 %q", in, got, want)
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

// TestReviewInstructionKeepsInReview 验证简体中文模板的审查列提示词明确写了：审查的任务直接在审查列领取，
// 不要先移到进行中；只有研发任务才在进行中处理。
func TestReviewInstructionKeepsInReview(t *testing.T) {
	review := templateTexts["zh-CN"].Columns[textReview].Instruction
	if !strings.Contains(review, "不要先移到「进行中」") {
		t.Errorf("审查列提示词缺少“不要先移到进行中”的说明: %q", review)
	}
}

// TestDoingInstructionRequiresWorktree 验证简体中文模板的进行中列提示词写明：调度者主动建 worktree 和分支，
// 并发时不共用目录，成果提交到分支后才交接。
func TestDoingInstructionRequiresWorktree(t *testing.T) {
	doing := templateTexts["zh-CN"].Columns[textDoing].Instruction
	for _, s := range []string{"git worktree add", "必须由你主动做", "绝不能让多个任务共用同一个目录", "一个终端只跑一个调度 agent"} {
		if !strings.Contains(doing, s) {
			t.Errorf("进行中列提示词缺少 %q", s)
		}
	}
}

// TestDoingInstructionConcurrencyByRoster 验证简体中文模板对并发的写法：按名册里的研发工程师算，
// 一次领满并同时派出，一个任务新开一个子 agent，审查名额另算；不能再有容易被理解成「研发加审查一共两个」的「没说就是 2」，
// 待办列判断名额时也只数研发工程师。
func TestDoingInstructionConcurrencyByRoster(t *testing.T) {
	texts := templateTexts["zh-CN"].Columns
	doing := texts[textDoing].Instruction
	for _, s := range []string{"并发按名册算", "一次领几个研发任务、同时派出各自的子 agent", "一个任务新开一个子 agent", "审查的名额另算，不占研发名额"} {
		if !strings.Contains(doing, s) {
			t.Errorf("进行中列提示词缺少 %q", s)
		}
	}
	if strings.Contains(doing, "没说就是 2") {
		t.Errorf("进行中列提示词不应该再写“没说就是 2”: %q", doing)
	}
	if todo := texts[textTodo].Instruction; !strings.Contains(todo, "只数研发工程师，审查占用的不算") {
		t.Errorf("待办列提示词应该写明名额只数研发工程师: %q", todo)
	}
}

// TestPlanCommentInTemplate 确认每种语言的研发和审查列都指定先记录方案再开始执行。
func TestPlanCommentInTemplate(t *testing.T) {
	afterComment := map[string]string{
		"zh-CN": "评论成功后",
		"zh-TW": "留言成功後",
		"en":    "only after the comment is saved",
		"ja":    "保存されたことを確認してから",
	}
	for _, locale := range supportedLocales {
		for _, kind := range []textKind{textDoing, textReview} {
			instruction := textsFor(locale).Columns[kind].Instruction
			_, after, found := strings.Cut(instruction, "comment_card")
			if !found || !strings.Contains(after, afterComment[locale]) {
				t.Errorf("%s 的列 %d 没有要求先用 comment_card 记录方案再开始执行", locale, kind)
			}
		}
	}
}

func TestReviewEvidenceInTemplate(t *testing.T) {
	screenshot := map[string]string{
		"zh-CN": "实际截图",
		"zh-TW": "畫面截圖",
		"en":    "screenshots of the actual pages",
		"ja":    "スクリーンショット",
	}
	for _, locale := range supportedLocales {
		instruction := textsFor(locale).Columns[textReview].Instruction
		for _, want := range []string{screenshot[locale], "- [x]", "- [ ]"} {
			if !strings.Contains(instruction, want) {
				t.Errorf("%s 的审查列证据要求缺少 %q", locale, want)
			}
		}
	}
	if !strings.Contains(seedEngineerTexts[seedReview].Prompt, "回传实际效果截图") {
		t.Error("默认审查工程师应把实际页面截图回传给调度者")
	}
}

// TestDoingInstructionTagsAndTaskItems 确认每种语言的进行中列都写明两条规则：
// 领取后只能从看板已有的标签里挑，不新建标签；交接前要在描述里补上任务项清单。
func TestDoingInstructionTagsAndTaskItems(t *testing.T) {
	markers := map[string][2]string{
		"zh-CN": {"不要新建标签", "「任务项」清单"},
		"zh-TW": {"不要新建標籤", "「任務項」清單"},
		"en":    {"do not create new tags", "list of task items"},
		"ja":    {"新しいタグは作らず", "タスク項目の一覧"},
	}
	for _, locale := range supportedLocales {
		doing := textsFor(locale).Columns[textDoing].Instruction
		for _, m := range markers[locale] {
			if !strings.Contains(doing, m) {
				t.Errorf("%s 的进行中列提示词缺少 %q", locale, m)
			}
		}
		if before, _, ok := strings.Cut(doing, "finish_card"); !ok || !strings.Contains(before, "update_card") {
			t.Errorf("%s 的进行中列提示词应该先写 update_card 调标签和补任务项，再写 finish_card", locale)
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
