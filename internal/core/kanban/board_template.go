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

// engineerText 是一位起步工程师的名称和角色提示词。
type engineerText struct {
	Name, Prompt string
}

// 起步工程师在 seedEngineerTexts 里的下标。
const (
	seedDev1 = iota
	seedDev2
	seedReview
	engineerSeedCount
)

// seedEngineerTexts 是起步工程师的名称和角色提示词。名称固定用英文名、提示词固定写中文，
// 不随界面语言变化；之后都是看板自己的数据，可以随意改。
var seedEngineerTexts = [engineerSeedCount]engineerText{
	seedDev1:   {Name: "Atlas", Prompt: seedDevPrompt},
	seedDev2:   {Name: "Orion", Prompt: seedDevPrompt},
	seedReview: {Name: "Sentinel", Prompt: "你是测试工程师，只验收，不改代码。对照任务原文逐条检查，运行测试、构建和静态检查，必要时实际运行功能；给出通过或不通过的结论，不通过时写清哪里不对、怎么复现、怎么改。"},
}

// seedDevPrompt 是两位起步全栈工程师共用的角色提示词。
const seedDevPrompt = "你是全栈工程师，前端、后端、数据库和测试都能做。先读懂任务原文和相关代码，改动只围绕任务本身，保持项目现有风格；做完自己跑测试和构建，确认通过再交付，总结里写清改了什么、怎么验证的。"

// engineerSeedKinds 是各起步工程师的类型。
var engineerSeedKinds = [engineerSeedCount]string{EngineerDev, EngineerDev, EngineerReview}

// engineerSeeds 是各模板起步招募哪些工程师：研发模板 2 个全栈加 1 个测试，
// 简单模板没有审查列，只有 2 个全栈，empty 没有。
var engineerSeeds = map[string][]int{
	TemplateDev:    {seedDev1, seedDev2, seedReview},
	TemplateSimple: {seedDev1, seedDev2},
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
				Name:        "收件箱",
				Description: "还没想清楚的想法和需求，由人整理后再决定要不要做。",
				Instruction: "这一列是人收集想法的地方，agent 不处理这里已有的卡片：不派发子 agent，不领取，不动手实现，也不要移动或修改别人创建的卡片。你可以在这里创建新任务，并把自己创建的任务移到下一列，之后才轮到研发承接；需要时可以在卡片下留言补充分析。",
			},
			textTodo: {
				Name:        "待办",
				Description: "确定要做、正在排队的任务。",
				Instruction: "这一列的任务已确定但还没安排开始。不要派发子 agent，不要领取，也不要自己移动卡片；由人排好优先级后拖到下一列。需要时可以留言补充分析。",
			},
			textDoing: {
				Name:        "进行中",
				Description: "正在做的任务。卡片被拖进这一列后，agent 会来领取并开始工作。",
				Instruction: "这一列是研发区：只有这一列的卡片才可以派发研发子 agent 处理。你作为调度者领取卡片（next_card 或 claim_card），先读完卡片的描述、评论和依赖，把任务原文按你的理解写成角色和提示词，交给子 agent 在独立的分支或 worktree 里完成，自己不写具体代码。同时进行的卡片数不超过人告诉你的并发数（没说就是 2）。任务不明确或遇到需要人拍板的问题，先在卡片下留言写出疑问，再把卡片移回「待办」这类不领取的列等人回复，不要直接开始。做完后调用 finish_card，写一段总结并填上分支名和链接，卡片会自动进入下一列。任务是否完成不由你决定，不要把卡片移进完成列。",
			},
			textReview: {
				Name:        "审查",
				Description: "agent 已经做完、等待验收的任务。",
				Instruction: "这一列是审查区：只有这一列的卡片才可以派发审查子 agent 处理，审查只验收，不修改代码。领取卡片后读完描述、评论、研发的交接总结和分支，把任务原文按你的理解写成审查要求，派子 agent 逐条对照验收、跑测试，必要时实际运行。通过：在卡片下留言写出审查结论，再把卡片移到完成列，任务才算完成，这个决定只能由审查做出。不通过：在卡片下留言写清楚哪里不对、怎么改，再把卡片放回 agent 领取任务的那一列（通常是「进行中」），由研发返工。",
			},
			textDone: {
				Name:        "完成",
				Description: "已经做完并验收的任务。",
				Instruction: "这一列是已经完成的任务：agent 一律不要操作，包括领取、移出、修改卡片和留言，也不要派发子 agent。只有审查通过时，审查才能把卡片移进来；其他情况要移动或修改，都由人来做。",
			},
		},
		RuleStart: "开始计时", RuleFinish: "完成计时", RuleReopen: "重新打开",
	},
	"zh-TW": {
		Columns: [textKindCount]columnText{
			textIdea: {
				Name:        "收件匣",
				Description: "還沒想清楚的想法與需求，由人整理後再決定要不要做。",
				Instruction: "這一列是人收集想法的地方，agent 不處理這裡已有的卡片：不派發子 agent，不領取，不動手實作，也不要移動或修改別人建立的卡片。你可以在這裡建立新任務，並把自己建立的任務移到下一列，之後才輪到研發承接；需要時可以在卡片下留言補充分析。",
			},
			textTodo: {
				Name:        "待辦",
				Description: "確定要做、正在排隊的任務。",
				Instruction: "這一列的任務已確定但還沒安排開始。不要派發子 agent，不要領取，也不要自己移動卡片；由人排好優先順序後拖到下一列。需要時可以留言補充分析。",
			},
			textDoing: {
				Name:        "進行中",
				Description: "正在做的任務。卡片被拖進這一列後，agent 會來領取並開始工作。",
				Instruction: "這一列是研發區：只有這一列的卡片才可以派發研發子 agent 處理。你作為調度者領取卡片（next_card 或 claim_card），先讀完卡片的描述、留言與相依項目，把任務原文依你的理解寫成角色與提示詞，交給子 agent 在獨立的分支或 worktree 裡完成，自己不寫具體程式碼。同時進行的卡片數不超過人告訴你的並行數（沒說就是 2）。任務不明確或遇到需要人決定的問題，先在卡片下留言寫出疑問，再把卡片移回「待辦」這類不領取的列等人回覆，不要直接開始。做完後呼叫 finish_card，寫一段總結並填上分支名稱與連結，卡片會自動進入下一列。任務是否完成不由你決定，不要把卡片移進完成列。",
			},
			textReview: {
				Name:        "審查",
				Description: "agent 已經做完、等待驗收的任務。",
				Instruction: "這一列是審查區：只有這一列的卡片才可以派發審查子 agent 處理，審查只驗收，不修改程式碼。領取卡片後讀完描述、留言、研發的交接總結與分支，把任務原文依你的理解寫成審查要求，派子 agent 逐條對照驗收、跑測試，必要時實際執行。通過：在卡片下留言寫出審查結論，再把卡片移到完成列，任務才算完成，這個決定只能由審查做出。不通過：在卡片下留言寫清楚哪裡不對、怎麼改，再把卡片放回 agent 領取任務的那一列（通常是「進行中」），由研發返工。",
			},
			textDone: {
				Name:        "完成",
				Description: "已經做完並驗收的任務。",
				Instruction: "這一列是已經完成的任務：agent 一律不要操作，包括領取、移出、修改卡片與留言，也不要派發子 agent。只有審查通過時，審查才能把卡片移進來；其他情況要移動或修改，都由人來做。",
			},
		},
		RuleStart: "開始計時", RuleFinish: "完成計時", RuleReopen: "重新開啟",
	},
	"en": {
		Columns: [textKindCount]columnText{
			textIdea: {
				Name:        "Inbox",
				Description: "Rough ideas and requests that are not ready yet. A person sorts them out and decides whether to do them.",
				Instruction: "This column is where people collect ideas. Agents do not work on the cards already here: do not dispatch sub-agents, do not claim them, do not start implementing, and do not move or edit cards that others created. You may create new tasks here and move the ones you created to the next column, and only then does development pick them up. You may leave a comment with extra analysis if it helps.",
			},
			textTodo: {
				Name:        "To do",
				Description: "Tasks that are confirmed and waiting in line.",
				Instruction: "Tasks here are confirmed but not scheduled to start. Do not dispatch sub-agents, do not claim them, and do not move cards yourself; a person sets the priority and drags them to the next column. You may leave a comment with extra analysis if it helps.",
			},
			textDoing: {
				Name:        "In progress",
				Description: "Tasks being worked on. Once a card is dragged into this column, an agent picks it up and starts working.",
				Instruction: "This is the development column: only cards here may be handed to development sub-agents. As the dispatcher, claim a card (next_card or claim_card), read its description, comments and dependencies first, then write a role and prompt from the original task as you understand it and hand it to a sub-agent that works in a separate branch or worktree; do not write the code yourself. Keep the number of cards in progress within the concurrency a person gave you (2 if none was given). If the task is unclear or needs a human decision, leave a comment with your questions first, then move the card back to a column that agents do not pick up from, such as “To do”, and wait for a reply instead of starting. When done, call finish_card with a summary plus the branch name and links; the card moves to the next column automatically. Whether the task is complete is not your call: never move a card into the Done column.",
			},
			textReview: {
				Name:        "Review",
				Description: "Tasks an agent has finished and that are waiting to be checked.",
				Instruction: "This is the review column: only cards here may be handed to review sub-agents, and review only checks the work and never edits code. After claiming a card, read its description, comments, the developer's hand-over summary and the branch, write review requirements from the original task as you understand it, and have a sub-agent verify each point, run the tests and run the code when needed. Pass: leave a comment with the review conclusion, then move the card to the Done column; the task is complete only then, and only review may decide that. Fail: leave a comment saying what is wrong and how to fix it, then put the card back in the column where agents pick up work (usually “In progress”) for rework.",
			},
			textDone: {
				Name:        "Done",
				Description: "Tasks that are finished and accepted.",
				Instruction: "This column holds finished tasks. Agents must not touch it: do not claim, move out, edit or comment on cards here, and do not dispatch sub-agents. Only review may move a card in, and only when the review passes; any other move or edit is done by a person.",
			},
		},
		RuleStart: "Start timer", RuleFinish: "Finish timer", RuleReopen: "Reopen",
	},
	"ja": {
		Columns: [textKindCount]columnText{
			textIdea: {
				Name:        "受信トレイ",
				Description: "まだ固まっていないアイデアや要望。人が整理して、やるかどうかを決めます。",
				Instruction: "この列は人がアイデアを集める場所で、エージェントはここにあるカードを処理しません。サブエージェントを割り当てず、担当せず、実装も始めず、他の人が作ったカードを移動・編集しないでください。ここで新しいタスクを作り、自分で作ったタスクを次の列へ移せます。開発が引き受けるのはその後です。必要なら、カードにコメントで分析を補足できます。",
			},
			textTodo: {
				Name:        "未着手",
				Description: "やることが決まっていて、順番待ちのタスク。",
				Instruction: "この列のタスクは決まっていますが、まだ着手の予定がありません。サブエージェントを割り当てず、担当せず、カードを自分で動かさないでください。人が優先順位を決めて次の列へドラッグします。必要ならコメントで分析を補足できます。",
			},
			textDoing: {
				Name:        "進行中",
				Description: "作業中のタスク。カードがこの列にドラッグされると、エージェントが担当して作業を始めます。",
				Instruction: "この列は開発用です。サブエージェントに開発を任せられるのは、この列のカードだけです。あなたは手配役としてカードを担当し（next_card または claim_card）、まず説明、コメント、依存関係を読み、タスクの原文を自分の理解で役割とプロンプトにまとめて、別のブランチか worktree で作業するサブエージェントに渡します。自分ではコードを書きません。同時に進めるカード数は、人から伝えられた並行数までです（指定がなければ 2）。タスクが不明確、または人の判断が必要なときは、先にカードへ疑問をコメントし、カードを「未着手」のような担当対象外の列へ戻して返事を待ち、勝手に始めないでください。終わったら finish_card を呼び、要約とブランチ名、リンクを記入します。カードは自動的に次の列へ移ります。タスクが完了したかどうかはあなたが決めることではないので、カードを完了列へ移さないでください。",
			},
			textReview: {
				Name:        "レビュー",
				Description: "エージェントが終えて、検収を待つタスク。",
				Instruction: "この列はレビュー用です。サブエージェントにレビューを任せられるのは、この列のカードだけで、レビューは検収のみでコードは変更しません。カードを担当したら、説明、コメント、開発側の引き継ぎ要約、ブランチを読み、タスクの原文を自分の理解でレビュー要件にまとめ、サブエージェントに 1 項目ずつ検収させ、テストを実行し、必要なら実際に動かさせます。合格：レビュー結果をカードにコメントしてから、カードを完了列へ移します。それで初めてタスク完了となり、この判断はレビューだけが行えます。不合格：どこが違うか、どう直すかをコメントしてから、エージェントが作業を担当する列（通常は「進行中」）へカードを戻し、開発側にやり直させます。",
			},
			textDone: {
				Name:        "完了",
				Description: "終わって確認も済んだタスク。",
				Instruction: "この列は完了したタスクです。エージェントは一切操作しないでください。この列のカードの担当、移動、編集、コメントも、サブエージェントの割り当ても禁止です。カードを移せるのは、レビューが合格したときのレビューだけで、それ以外の移動や編集は人が行います。",
			},
		},
		RuleStart: "開始時刻を記録", RuleFinish: "完了時刻を記録", RuleReopen: "再オープン",
	},
	"ko": {
		Columns: [textKindCount]columnText{
			textIdea: {
				Name:        "받은 편지함",
				Description: "아직 정리되지 않은 아이디어와 요청입니다. 사람이 정리한 뒤 진행할지 결정합니다.",
				Instruction: "이 열은 사람이 아이디어를 모으는 곳이며, 에이전트는 여기에 있는 카드를 처리하지 않습니다. 하위 에이전트를 배정하지 말고, 맡지 말고, 구현도 시작하지 말며, 다른 사람이 만든 카드를 옮기거나 수정하지 마세요. 여기에 새 작업을 만들고 직접 만든 작업을 다음 열로 옮길 수 있으며, 개발이 맡는 것은 그 이후입니다. 필요하면 카드에 댓글로 분석을 덧붙일 수 있습니다.",
			},
			textTodo: {
				Name:        "할 일",
				Description: "하기로 확정되어 순서를 기다리는 작업입니다.",
				Instruction: "이 열의 작업은 확정되었지만 아직 시작할 예정이 없습니다. 하위 에이전트를 배정하지 말고, 맡지 말고, 카드를 직접 옮기지 마세요. 사람이 우선순위를 정하고 다음 열로 끌어다 놓습니다. 필요하면 댓글로 분석을 덧붙일 수 있습니다.",
			},
			textDoing: {
				Name:        "진행 중",
				Description: "작업 중인 카드입니다. 카드가 이 열로 이동하면 에이전트가 맡아서 작업을 시작합니다.",
				Instruction: "이 열은 개발 영역입니다. 개발 하위 에이전트에게 맡길 수 있는 카드는 이 열의 카드뿐입니다. 당신은 조율자로서 카드를 맡고(next_card 또는 claim_card), 먼저 설명, 댓글, 의존 관계를 읽은 뒤, 작업 원문을 당신의 이해대로 역할과 프롬프트로 정리해 별도의 브랜치나 worktree에서 작업할 하위 에이전트에게 넘기세요. 직접 코드를 작성하지 않습니다. 동시에 진행하는 카드 수는 사람이 알려 준 동시 처리 수를 넘지 않습니다(알려 주지 않으면 2). 작업이 불명확하거나 사람의 결정이 필요하면 먼저 카드에 의문점을 댓글로 남기고, 카드를 '할 일' 같은 맡지 않는 열로 옮겨 답을 기다리며 바로 시작하지 마세요. 끝나면 finish_card를 호출해 요약과 브랜치 이름, 링크를 남기세요. 카드는 자동으로 다음 열로 이동합니다. 작업이 끝났는지는 당신이 정하지 않으므로 카드를 완료 열로 옮기지 마세요.",
			},
			textReview: {
				Name:        "검토",
				Description: "에이전트가 마치고 검수를 기다리는 작업입니다.",
				Instruction: "이 열은 검토 영역입니다. 검토 하위 에이전트에게 맡길 수 있는 카드는 이 열의 카드뿐이며, 검토는 검수만 하고 코드를 수정하지 않습니다. 카드를 맡은 뒤 설명, 댓글, 개발 쪽 인수인계 요약, 브랜치를 읽고, 작업 원문을 당신의 이해대로 검토 요구 사항으로 정리해 하위 에이전트가 항목별로 검수하고 테스트를 돌리고 필요하면 실제로 실행하게 하세요. 통과: 카드에 검토 결론을 댓글로 남긴 뒤 카드를 완료 열로 옮깁니다. 그때 비로소 작업이 완료되며, 이 결정은 검토만 내릴 수 있습니다. 통과하지 못함: 무엇이 틀렸고 어떻게 고칠지 댓글로 남긴 뒤, 에이전트가 작업을 맡는 열(보통 '진행 중')로 카드를 되돌려 개발이 다시 하게 하세요.",
			},
			textDone: {
				Name:        "완료",
				Description: "끝나고 확인까지 마친 작업입니다.",
				Instruction: "이 열은 완료된 작업입니다. 에이전트는 일절 건드리지 마세요. 이 열의 카드를 맡거나 옮기거나 수정하거나 댓글을 다는 것도, 하위 에이전트를 배정하는 것도 금지입니다. 카드를 옮겨 올 수 있는 것은 검토가 통과했을 때의 검토뿐이며, 그 밖의 이동과 수정은 사람이 합니다.",
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

// seedRules 为新看板创建起步的自动化规则并返回，顺序就是执行顺序。
// 起步规则是模板自己生成的，不走用户输入的校验，但测试会保证它们都能通过 normalizeRule。
func (u *unit) seedRules(ctx context.Context, boardID int64, locale string, cols []*Column) ([]*Rule, error) {
	rules := defaultRules(textsFor(locale), cols)
	now := u.now()
	for i, r := range rules {
		r.BoardID, r.Enabled, r.Sort = boardID, true, int64(i+1)*ruleSortStep
		r.CreatedAt, r.UpdatedAt = now, now
		if err := u.rule.Create(ctx, r); err != nil {
			return nil, wrapDB("创建起步的规则", err)
		}
	}
	if rules == nil {
		rules = []*Rule{}
	}
	return rules, nil
}
