package kanban

import (
	"context"
	"strings"
)

// 起步模板只在创建看板时使用一次：列名、说明、指令和默认规则的名称按界面语言生成，
// 之后全部是看板自己的数据，用户可以随意修改。所以这里的文案不会随版本升级而回头改动已有看板。

// textKind 标识起步模板里用到的是哪一段列文案。
type textKind int

const (
	textIdea textKind = iota
	textTodo
	textDoing
	textReview
	textDone
	textKindCount
)

// defaultLocale 不认识的界面语言一律按它处理。
const defaultLocale = "en"

// columnText 是一列的名称、给人看的说明和给 agent 看的指令。
type columnText struct {
	Name, Description, Instruction string
}

// localeTexts 是一种界面语言下，起步模板用到的全部文案。
// 指令里不写下一列叫什么名字，这样用户把列改名以后，指令仍然是对的。
type localeTexts struct {
	Columns [textKindCount]columnText
	// 三条默认自动化规则的名称：开始计时、完成计时、重新打开。
	RuleStart, RuleFinish, RuleReopen string
}

// columnSpec 描述模板里的一列：用哪段文案、两个开关，以及完成后去哪一列。
type columnSpec struct {
	Text        textKind
	AgentPickup bool
	IsDone      bool
	Next        int // 下一列在模板里的下标，-1 表示没有
}

// templateColumns 是各模板的列，按从左到右的顺序。empty 没有列。
var templateColumns = map[string][]columnSpec{
	TemplateDev: {
		{Text: textIdea, Next: 1},
		{Text: textTodo, Next: 2},
		{Text: textDoing, AgentPickup: true, Next: 3},
		{Text: textReview, Next: 4},
		{Text: textDone, IsDone: true, Next: -1},
	},
	TemplateSimple: {
		{Text: textTodo, Next: 1},
		{Text: textDoing, AgentPickup: true, Next: 2},
		{Text: textDone, IsDone: true, Next: -1},
	},
	TemplateEmpty: nil,
}

// normalizeLocale 把各种写法的语言标识归并到支持的五种之一：
// zh-CN、zh-TW、en、ja、ko。大小写和下划线都不敏感，不认识的返回 defaultLocale。
func normalizeLocale(locale string) string {
	l := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(locale), "_", "-"))
	switch {
	case l == "zh-tw" || l == "zh-hk" || l == "zh-mo" || strings.HasPrefix(l, "zh-hant"):
		return "zh-TW"
	case l == "zh" || strings.HasPrefix(l, "zh-"):
		return "zh-CN"
	case l == "ja" || strings.HasPrefix(l, "ja-"):
		return "ja"
	case l == "ko" || strings.HasPrefix(l, "ko-"):
		return "ko"
	}
	return defaultLocale
}

// textsFor 返回某种界面语言的模板文案。
func textsFor(locale string) localeTexts {
	return templateTexts[normalizeLocale(locale)]
}

// templateTexts 是五种界面语言的模板文案。
var templateTexts = map[string]localeTexts{
	"zh-CN": {
		Columns: [textKindCount]columnText{
			textIdea: {
				Name:        "想法池",
				Description: "还没想清楚的想法和需求，由人整理后再决定要不要做。",
				Instruction: "这一列的卡片只是想法，不要领取，也不要动手实现。需要时可以在卡片下留言补充分析。",
			},
			textTodo: {
				Name:        "待办",
				Description: "确定要做、正在排队的任务。",
				Instruction: "这一列的卡片还没有开始，不要自己移动它们。由人排好优先级，再拖到下一列。",
			},
			textDoing: {
				Name:        "进行中",
				Description: "正在做的任务。卡片被拖进这一列后，agent 会来领取并开始工作。",
				Instruction: "你可以领取这一列的卡片，一次只做一张。先读完卡片的描述、评论和依赖，在独立的分支或 worktree 里完成工作。" +
					"遇到需要人拍板的问题，在卡片下留言说明并释放卡片。做完后调用 finish_card，写一段总结并填上分支名和链接，卡片会自动进入下一列。",
			},
			textReview: {
				Name:        "审查",
				Description: "agent 已经做完、等人检查的任务。",
				Instruction: "这一列等人检查，不要领取或移动其中的卡片。审查没通过时，人会在卡片下留言并把卡片拖回之前的列，到时按留言继续修改。",
			},
			textDone: {
				Name:        "完成",
				Description: "已经做完并验收的任务。",
				Instruction: "这一列的任务已经结束，不需要处理。",
			},
		},
		RuleStart: "开始计时", RuleFinish: "完成计时", RuleReopen: "重新打开",
	},
	"zh-TW": {
		Columns: [textKindCount]columnText{
			textIdea: {
				Name:        "想法池",
				Description: "還沒想清楚的想法與需求，由人整理後再決定要不要做。",
				Instruction: "這一列的卡片只是想法，不要領取，也不要動手實作。需要時可以在卡片下留言補充分析。",
			},
			textTodo: {
				Name:        "待辦",
				Description: "確定要做、正在排隊的任務。",
				Instruction: "這一列的卡片還沒有開始，不要自己移動它們。由人排好優先順序，再拖到下一列。",
			},
			textDoing: {
				Name:        "進行中",
				Description: "正在做的任務。卡片被拖進這一列後，agent 會來領取並開始工作。",
				Instruction: "你可以領取這一列的卡片，一次只做一張。先讀完卡片的描述、留言與相依項目，在獨立的分支或 worktree 裡完成工作。" +
					"遇到需要人決定的問題，在卡片下留言說明並釋放卡片。做完後呼叫 finish_card，寫一段總結並填上分支名稱與連結，卡片會自動進入下一列。",
			},
			textReview: {
				Name:        "審查",
				Description: "agent 已經做完、等人檢查的任務。",
				Instruction: "這一列等人檢查，不要領取或移動其中的卡片。審查沒通過時，人會在卡片下留言並把卡片拖回之前的列，到時依留言繼續修改。",
			},
			textDone: {
				Name:        "完成",
				Description: "已經做完並驗收的任務。",
				Instruction: "這一列的任務已經結束，不需要處理。",
			},
		},
		RuleStart: "開始計時", RuleFinish: "完成計時", RuleReopen: "重新開啟",
	},
	"en": {
		Columns: [textKindCount]columnText{
			textIdea: {
				Name:        "Ideas",
				Description: "Rough ideas and requests that are not ready yet. A person sorts them out and decides whether to do them.",
				Instruction: "Cards in this column are only ideas. Do not claim them and do not start implementing. You may leave a comment with extra analysis if it helps.",
			},
			textTodo: {
				Name:        "To do",
				Description: "Tasks that are confirmed and waiting in line.",
				Instruction: "Cards in this column have not started. Do not move them yourself. A person sets the priority and drags them to the next column.",
			},
			textDoing: {
				Name:        "In progress",
				Description: "Tasks being worked on. Once a card is dragged into this column, an agent picks it up and starts working.",
				Instruction: "You may claim cards in this column, one at a time. Read the card's description, comments and dependencies first, then do the work in a separate branch or worktree. " +
					"If you hit a question that needs a human decision, leave a comment on the card and release it. When you are done, call finish_card with a summary plus the branch name and links; the card moves to the next column automatically.",
			},
			textReview: {
				Name:        "Review",
				Description: "Tasks an agent has finished and a person needs to check.",
				Instruction: "Cards in this column wait for a person. Do not claim or move them. If the review fails, the person leaves a comment and drags the card back to an earlier column; then continue from the comment.",
			},
			textDone: {
				Name:        "Done",
				Description: "Tasks that are finished and accepted.",
				Instruction: "Tasks here are finished. Nothing to do.",
			},
		},
		RuleStart: "Start timer", RuleFinish: "Finish timer", RuleReopen: "Reopen",
	},
	"ja": {
		Columns: [textKindCount]columnText{
			textIdea: {
				Name:        "アイデア",
				Description: "まだ固まっていないアイデアや要望。人が整理して、やるかどうかを決めます。",
				Instruction: "この列のカードはまだアイデアです。担当せず、実装も始めないでください。必要なら、カードにコメントで分析を補足できます。",
			},
			textTodo: {
				Name:        "未着手",
				Description: "やることが決まっていて、順番待ちのタスク。",
				Instruction: "この列のカードはまだ始まっていません。自分で動かさないでください。人が優先順位を決めて、次の列へドラッグします。",
			},
			textDoing: {
				Name:        "進行中",
				Description: "作業中のタスク。カードがこの列にドラッグされると、エージェントが担当して作業を始めます。",
				Instruction: "この列のカードは担当できます。同時に扱うのは 1 枚だけです。まずカードの説明、コメント、依存関係を読み、別のブランチか worktree で作業してください。" +
					"人の判断が必要な問題があれば、カードにコメントで説明して担当を解除します。終わったら finish_card を呼び、要約とブランチ名、リンクを記入します。カードは自動的に次の列へ移ります。",
			},
			textReview: {
				Name:        "レビュー",
				Description: "エージェントが終えて、人の確認を待つタスク。",
				Instruction: "この列は人の確認待ちです。担当したり動かしたりしないでください。レビューで差し戻された場合は、人がコメントを残してカードを前の列へ戻すので、そのコメントに沿って修正します。",
			},
			textDone: {
				Name:        "完了",
				Description: "終わって確認も済んだタスク。",
				Instruction: "この列のタスクは完了済みです。対応は不要です。",
			},
		},
		RuleStart: "開始時刻を記録", RuleFinish: "完了時刻を記録", RuleReopen: "再オープン",
	},
	"ko": {
		Columns: [textKindCount]columnText{
			textIdea: {
				Name:        "아이디어",
				Description: "아직 정리되지 않은 아이디어와 요청입니다. 사람이 정리한 뒤 진행할지 결정합니다.",
				Instruction: "이 열의 카드는 아직 아이디어입니다. 맡지 말고 구현도 시작하지 마세요. 필요하면 카드에 댓글로 분석을 덧붙일 수 있습니다.",
			},
			textTodo: {
				Name:        "할 일",
				Description: "하기로 확정되어 순서를 기다리는 작업입니다.",
				Instruction: "이 열의 카드는 아직 시작 전입니다. 직접 옮기지 마세요. 사람이 우선순위를 정하고 다음 열로 끌어다 놓습니다.",
			},
			textDoing: {
				Name:        "진행 중",
				Description: "작업 중인 카드입니다. 카드가 이 열로 이동하면 에이전트가 맡아서 작업을 시작합니다.",
				Instruction: "이 열의 카드를 맡을 수 있으며, 한 번에 한 장만 처리합니다. 먼저 카드의 설명, 댓글, 의존 관계를 읽고 별도의 브랜치나 worktree에서 작업하세요. " +
					"사람의 결정이 필요한 문제가 생기면 카드에 댓글로 설명하고 맡은 카드를 놓아 주세요. 끝나면 finish_card를 호출해 요약과 브랜치 이름, 링크를 남기세요. 카드는 자동으로 다음 열로 이동합니다.",
			},
			textReview: {
				Name:        "검토",
				Description: "에이전트가 마치고 사람의 확인을 기다리는 작업입니다.",
				Instruction: "이 열은 사람이 확인하는 곳입니다. 맡거나 옮기지 마세요. 검토를 통과하지 못하면 사람이 댓글을 남기고 카드를 이전 열로 되돌리며, 그 댓글에 따라 이어서 수정하면 됩니다.",
			},
			textDone: {
				Name:        "완료",
				Description: "끝나고 확인까지 마친 작업입니다.",
				Instruction: "이 열의 작업은 끝났습니다. 처리할 필요가 없습니다.",
			},
		},
		RuleStart: "시작 시간 기록", RuleFinish: "완료 시간 기록", RuleReopen: "다시 열기",
	},
}

// seedColumns 按模板和语言为新看板创建起步的列，返回的列按从左到右排列。
// 先把所有列建好，再回填各列的下一列，因为建某一列时它的下一列还没有主键。
// 一个模板最多几列，所以逐条写入即可。
func (u *unit) seedColumns(ctx context.Context, boardID int64, template, locale string) ([]*Column, error) {
	specs := templateColumns[template]
	texts := textsFor(locale)
	now := u.now()
	cols := make([]*Column, len(specs))
	for i, s := range specs {
		t := texts.Columns[s.Text]
		cols[i] = &Column{
			BoardID: boardID, Name: t.Name, Description: t.Description, Instruction: t.Instruction,
			AgentPickup: s.AgentPickup, IsDone: s.IsDone,
			Sort: int64(i+1) * columnSortStep, CreatedAt: now, UpdatedAt: now,
		}
		if err := u.column.Create(ctx, cols[i]); err != nil {
			return nil, wrapDB("创建起步的列", err)
		}
	}
	for i, s := range specs {
		if s.Next < 0 {
			continue
		}
		cols[i].NextColumnID = cols[s.Next].ID
		if err := u.column.Save(ctx, cols[i]); err != nil {
			return nil, wrapDB("回填下一列", err)
		}
	}
	return cols, nil
}

// defaultRules 按列的情况生成起步的自动化规则，名称按界面语言。
// 以"agent 领活的列"当作进行中、"完成列"当作完成，所以只要模板里有这样的列就有对应的规则：
// 进入进行中写开始时间、进入完成写完成时间、离开完成清空完成时间。写时间都只在为空时才写，
// 不会覆盖人手动填的值。empty 模板没有列，也就没有规则。
func defaultRules(texts localeTexts, cols []*Column) []*Rule {
	var doing, done *Column
	for _, c := range cols {
		if c.AgentPickup && doing == nil {
			doing = c
		}
		if c.IsDone && done == nil {
			done = c
		}
	}
	var rules []*Rule
	if doing != nil {
		rules = append(rules, &Rule{
			Name: texts.RuleStart, TriggerType: TriggerCardEntered, ColumnID: doing.ID,
			Actions: []RuleAction{{Type: ActionSetStartedAt, Mode: ModeIfEmpty}},
		})
	}
	if done != nil {
		rules = append(rules,
			&Rule{
				Name: texts.RuleFinish, TriggerType: TriggerCardEntered, ColumnID: done.ID,
				Actions: []RuleAction{{Type: ActionSetFinishedAt, Mode: ModeIfEmpty}},
			},
			&Rule{
				Name: texts.RuleReopen, TriggerType: TriggerCardLeft, ColumnID: done.ID,
				Actions: []RuleAction{{Type: ActionClearFinishedAt}},
			})
	}
	return rules
}
