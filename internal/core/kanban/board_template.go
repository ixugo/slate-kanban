package kanban

import (
	"context"
	"strings"
)

// 起步模板只在创建看板时使用一次：列名、指令和默认规则的名称按界面语言生成，
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

// columnText 是一列的名称和给 agent 看的指令。
type columnText struct {
	Name, Instruction string
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
	seedReview: {Name: "Sentinel", Prompt: "你是测试工程师，只验收，不改代码。先只读了解任务并向调度者回传审查方案，等方案评论成功后再开始验收。对照任务原文逐条检查，运行测试、构建和静态检查，必要时实际运行功能；回传逐项验收结果和依据，页面验收时回传实际效果截图。给出通过或不通过的结论，不通过时写清哪里不对、怎么复现、怎么改。"},
}

// seedDevPrompt 是两位起步全栈工程师共用的角色提示词。
const seedDevPrompt = "你是全栈工程师，前端、后端、数据库和测试都能做。先只读了解任务和相关代码，向调度者回传研发方案，等方案评论成功后再开工。改动只围绕任务本身，保持项目现有风格；做完自己跑测试和构建，确认通过再交付，总结里写清改了什么、怎么验证的。"

// engineerSeedKinds 是各起步工程师的类型。
var engineerSeedKinds = [engineerSeedCount]string{EngineerDev, EngineerDev, EngineerReview}

// engineerSeeds 是各模板起步招募哪些工程师：研发模板 2 个全栈加 1 个测试，
// 简单模板没有审查列，只有 2 个全栈，empty 没有。
var engineerSeeds = map[string][]int{
	TemplateDev:    {seedDev1, seedDev2, seedReview},
	TemplateSimple: {seedDev1, seedDev2},
}

// columnSpec 描述模板里的一列：用哪段文案、完成标记，以及完成后去哪一列。
type columnSpec struct {
	Text   textKind
	IsDone bool
	Next   int // 下一列在模板里的下标，-1 表示没有
}

// templateColumns 是各模板的列，按从左到右的顺序。empty 没有列。
var templateColumns = map[string][]columnSpec{
	TemplateDev: {
		{Text: textIdea, Next: 1},
		{Text: textTodo, Next: 2},
		{Text: textDoing, Next: 3},
		{Text: textReview, Next: 4},
		{Text: textDone, IsDone: true, Next: -1},
	},
	TemplateSimple: {
		{Text: textTodo, Next: 1},
		{Text: textDoing, Next: 2},
		{Text: textDone, IsDone: true, Next: -1},
	},
	TemplateEmpty: nil,
}

// NormalizeLocale 把各种写法的语言标识归并到支持的四种之一：
// zh-CN、zh-TW、en、ja。大小写和下划线都不敏感，不认识的返回 defaultLocale。
func NormalizeLocale(locale string) string {
	l := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(locale), "_", "-"))
	switch {
	case l == "zh-tw" || l == "zh-hk" || l == "zh-mo" || strings.HasPrefix(l, "zh-hant"):
		return "zh-TW"
	case l == "zh" || strings.HasPrefix(l, "zh-"):
		return "zh-CN"
	case l == "ja" || strings.HasPrefix(l, "ja-"):
		return "ja"
	}
	return defaultLocale
}

// textsFor 返回某种界面语言的模板文案。
func textsFor(locale string) localeTexts {
	return templateTexts[NormalizeLocale(locale)]
}

// templateTexts 是四种界面语言的模板文案。
var templateTexts = map[string]localeTexts{
	"zh-CN": {
		Columns: [textKindCount]columnText{
			textIdea: {
				Name:        "收件箱",
				Instruction: "这一列是人收集想法的地方，agent 不处理这里已有的任务：不派发子 agent，不领取，不动手实现，也不要移动或修改别人创建的任务。你可以在这里创建新任务，并把自己创建的任务移到下一列，之后才轮到研发承接；需要时可以在任务下留言补充分析。",
			},
			textTodo: {
				Name:        "待办",
				Instruction: "这一列的任务已确定但还没安排开始，由你（agent）管理：用 next_card（column_id 选这一列）从上到下领取，读完描述、评论和依赖。任务清楚且研发工程师有空闲名额（只数研发工程师，审查占用的不算）时，用 move_card 移到「进行中」，再在那里重新领取并开工（换列会结束这里的领取）；有几个空闲名额就一次移几个，名额满了才留在这一列排队，不要硬领。任务不明确，先在任务下留言写出疑问，再把任务移回「收件箱」等人回复，留在这一列会被马上又领到。不要在这一列派发子 agent，也不要把任务移进完成列。",
			},
			textDoing: {
				Name:        "进行中",
				Instruction: "这一列是研发区：只有这一列的任务才可以派发研发子 agent 处理。你作为调度者领取任务（next_card 或 claim_card，带上 engineer_id 拿到租约；任务已经在这一列就直接领取，不用移动），先读完任务的描述、评论和依赖，把任务原文按你的理解写成角色和提示词，交给子 agent 完成，自己不写具体代码。每个任务都由你先在项目目录之外建好独立的 git worktree 和分支（例如 git worktree add ../<项目目录名>-<任务key小写> -b slate/<key小写>-<英文标题>），再把 worktree 的绝对路径交给子 agent，要求它只在里面改动，测试通过后提交到这个分支，不合并、不推送；并发时（同时有两个及以上任务在做，或者还有别的调度 agent 在守候同一个看板）这一步必须由你主动做，绝不能让多个任务共用同一个目录。子 agent 先只读拟定研发方案并回传给你，你用 comment_card 代发到任务下，评论成功后才通知它开始开发。一个终端只跑一个调度 agent：你只负责领任务、建 worktree 和分支、派子 agent、收结果。并发按名册算：研发工程师有几位空闲，就一次领几个研发任务、同时派出各自的子 agent，不要领一个等它做完再领下一个；一个任务新开一个子 agent，不要把新任务交给做过别的任务的子 agent，只有同一个任务返工才继续用原来的。审查的名额另算，不占研发名额；人明确给了并发数就按人说的。任务不明确或遇到需要人拍板的问题，先在任务下留言写出疑问，再把任务移回「收件箱」等人回复，不要直接开始。领取后先对照任务内容用 update_card 调整标签：只能从看板已有的标签里挑（用 tag_ids），不要新建标签，没有合适的就不贴。做完调用 finish_card 之前，先用 update_card 在描述末尾补上「任务项」清单：每项一行，已实现的写成「- [x] 内容」，没做完的写成「- [ ] 内容」，让人看描述就知道实现了什么。做完后确认子 agent 的成果已经提交到分支，再调用 finish_card，写一段总结并填上分支名和链接，任务会自动进入下一列。任务是否完成不由你决定，不要把任务移进完成列。",
			},
			textReview: {
				Name:        "审查",
				Instruction: "这一列是审查区：只有这一列的任务才可以派发审查子 agent 处理，审查只验收，不修改代码。任务在这一列就直接在这里领取并审查，不要先移到「进行中」：只有研发任务才在「进行中」处理，审查不通过需要返工时才放回去。领取任务后读完描述、评论、研发的交接总结和分支，把任务原文按你的理解写成审查要求，派子 agent 逐条对照验收、跑测试，必要时实际运行。子 agent 先只读拟定审查方案并回传给你，你用 comment_card 代发到任务下，评论成功后才通知它开始验收。用 comment_card 发布结论和证据：页面验收附实际截图，功能验收逐项勾选并写明依据（- [x] 通过，- [ ] 未通过或未验证）。缺少必要证据时写清原因，暂不判通过。通过：在任务下留言写出审查结论。适用本机专属 Slate 分支时，由调度 agent 按 Slate 技能第八节合并，记录提交后移到完成列并清理；其他任务直接移到完成列。不通过：在任务下留言写清楚哪里不对、怎么改，再把任务放回 agent 领取任务的那一列（通常是「进行中」），由研发返工。",
			},
			textDone: {
				Name:        "完成",
				Instruction: "这一列是已经完成的任务：agent 一律不要操作，包括领取、移出、修改任务和留言，也不要派发子 agent。只有审查通过时，审查才能把任务移进来；其他情况要移动或修改，都由人来做。",
			},
		},
		RuleStart: "开始计时", RuleFinish: "完成计时", RuleReopen: "重新打开",
	},
	"zh-TW": {
		Columns: [textKindCount]columnText{
			textIdea: {
				Name:        "收件匣",
				Instruction: "這一列是人收集想法的地方，agent 不處理這裡已有的任務：不派發子 agent，不領取，不動手實作，也不要移動或修改別人建立的任務。你可以在這裡建立新任務，並把自己建立的任務移到下一列，之後才輪到研發承接；需要時可以在任務下留言補充分析。",
			},
			textTodo: {
				Name:        "待辦",
				Instruction: "這一列的任務已確定但還沒安排開始。不要派發子 agent，不要領取，也不要自己移動任務；由人排好優先順序後拖到下一列。需要時可以留言補充分析。",
			},
			textDoing: {
				Name:        "進行中",
				Instruction: "這一列是研發區：只有這一列的任務才可以派發研發子 agent 處理。你作為調度者領取任務（next_card 或 claim_card），先讀完任務的描述、留言與相依項目，把任務原文依你的理解寫成角色與提示詞，交給子 agent 在專用 worktree 裡完成，自己不寫具體程式碼。先為這個任務建立專用 worktree 和分支。子 agent 先以唯讀方式擬定研發方案並回傳給你，由你用 comment_card 代為留言，留言成功後才通知它開始開發。同時進行的任務數不超過人告訴你的並行數（沒說就是 2）。任務不明確或遇到需要人決定的問題，先在任務下留言寫出疑問，再把任務移回「待辦」這類不領取的列等人回覆，不要直接開始。領取後先對照任務內容用 update_card 調整標籤：只能從看板已有的標籤裡挑（用 tag_ids），不要新建標籤，沒有合適的就不貼。做完呼叫 finish_card 之前，先用 update_card 在描述末尾補上「任務項」清單：每項一行，已實作的寫成「- [x] 內容」，沒做完的寫成「- [ ] 內容」，讓人看描述就知道實作了什麼。做完後呼叫 finish_card，寫一段總結並填上分支名稱與連結，任務會自動進入下一列。任務是否完成不由你決定，不要把任務移進完成列。",
			},
			textReview: {
				Name:        "審查",
				Instruction: "這一列是審查區：只有這一列的任務才可以派發審查子 agent 處理，審查只驗收，不修改程式碼。領取任務後讀完描述、留言、研發的交接總結與分支，把任務原文依你的理解寫成審查要求，派子 agent 逐條對照驗收、跑測試，必要時實際執行。子 agent 先以唯讀方式擬定審查方案並回傳給你，由你用 comment_card 代為留言，留言成功後才通知它開始驗收。用 comment_card 發布結論與證據：頁面驗收附上實際畫面截圖，功能驗收逐項勾選並寫明依據（- [x] 通過，- [ ] 未通過或未驗證）。缺少必要證據時寫明原因，暫不判通過。通過：在任務下留言寫出審查結論。使用本機專屬 Slate 分支時，由調度 agent 按 Slate 技能第八節合併，記錄提交後移到完成列並清理；其他任務直接移到完成列。不通過：在任務下留言寫清楚哪裡不對、怎麼改，再把任務放回 agent 領取任務的那一列（通常是「進行中」），由研發返工。",
			},
			textDone: {
				Name:        "完成",
				Instruction: "這一列是已經完成的任務：agent 一律不要操作，包括領取、移出、修改任務與留言，也不要派發子 agent。只有審查通過時，審查才能把任務移進來；其他情況要移動或修改，都由人來做。",
			},
		},
		RuleStart: "開始計時", RuleFinish: "完成計時", RuleReopen: "重新開啟",
	},
	"en": {
		Columns: [textKindCount]columnText{
			textIdea: {
				Name:        "Inbox",
				Instruction: "This column is where people collect ideas. Agents do not work on the tasks already here: do not dispatch sub-agents, do not claim them, do not start implementing, and do not move or edit tasks that others created. You may create new tasks here and move the ones you created to the next column, and only then does development pick them up. You may leave a comment with extra analysis if it helps.",
			},
			textTodo: {
				Name:        "To do",
				Instruction: "Tasks here are confirmed but not scheduled to start. Do not dispatch sub-agents, do not claim them, and do not move tasks yourself; a person sets the priority and drags them to the next column. You may leave a comment with extra analysis if it helps.",
			},
			textDoing: {
				Name:        "In progress",
				Instruction: "This is the development column: only tasks here may be handed to development sub-agents. As the dispatcher, claim a task (next_card or claim_card), read its description, comments and dependencies first, then write a role and prompt from the original task as you understand it and hand it to a sub-agent that works in the dedicated worktree; do not write the code yourself. Create a dedicated worktree and branch for the task before dispatch. The sub-agent first inspects the task without editing and sends you a development plan. Post it to the task with comment_card and tell the sub-agent to start development only after the comment is saved. Keep the number of tasks in progress within the concurrency a person gave you (2 if none was given). If the task is unclear or needs a human decision, leave a comment with your questions first, then move the task back to a column that agents do not pick up from, such as “To do”, and wait for a reply instead of starting. After claiming a task, adjust its tags to match the content with update_card: pick only from the tags that already exist on the board (use tag_ids) and do not create new tags; if none fits, leave it untagged. Before calling finish_card, use update_card to append a list of task items to the end of the description, one per line, written as \"- [x] what was done\" or \"- [ ] what is left\", so people can see what was implemented from the description alone. When done, call finish_card with a summary plus the branch name and links; the task moves to the next column automatically. Whether the task is complete is not your call: never move a task into the Done column.",
			},
			textReview: {
				Name:        "Review",
				Instruction: "This is the review column: only tasks here may be handed to review sub-agents, and review only checks the work and never edits code. After claiming a task, read its description, comments, the developer's hand-over summary and the branch, write review requirements from the original task as you understand it, and have a sub-agent verify each point, run the tests and run the code when needed. The sub-agent first inspects the task without editing and sends you a review plan. Post it to the task with comment_card and tell the sub-agent to start review only after the comment is saved. Use comment_card to publish the conclusion and evidence: attach screenshots of the actual pages reviewed. For functional review, record evidence and mark each checklist item (- [x] passed, - [ ] failed or unverified). If required evidence is missing, explain why and withhold a pass. Pass: leave the review conclusion in a comment. For a dedicated local Slate branch, the dispatcher follows section eight of the Slate skill to integrate, record the commits, move to Done and clean up. Other reviewed tasks go directly to Done. Fail: leave a comment saying what is wrong and how to fix it, then put the task back in the column where agents pick up work (usually “In progress”) for rework.",
			},
			textDone: {
				Name:        "Done",
				Instruction: "This column holds finished tasks. Agents must not touch it: do not claim, move out, edit or comment on tasks here, and do not dispatch sub-agents. Only review may move a task in, and only when the review passes; any other move or edit is done by a person.",
			},
		},
		RuleStart: "Start timer", RuleFinish: "Finish timer", RuleReopen: "Reopen",
	},
	"ja": {
		Columns: [textKindCount]columnText{
			textIdea: {
				Name:        "受信トレイ",
				Instruction: "この列は人がアイデアを集める場所で、エージェントはここにあるタスクを処理しません。サブエージェントを割り当てず、担当せず、実装も始めず、他の人が作ったタスクを移動・編集しないでください。ここで新しいタスクを作り、自分で作ったタスクを次の列へ移せます。開発が引き受けるのはその後です。必要なら、タスクにコメントで分析を補足できます。",
			},
			textTodo: {
				Name:        "未着手",
				Instruction: "この列のタスクは決まっていますが、まだ着手の予定がありません。サブエージェントを割り当てず、担当せず、タスクを自分で動かさないでください。人が優先順位を決めて次の列へドラッグします。必要ならコメントで分析を補足できます。",
			},
			textDoing: {
				Name:        "進行中",
				Instruction: "この列は開発用です。サブエージェントに開発を任せられるのは、この列のタスクだけです。あなたは手配役としてタスクを担当し（next_card または claim_card）、まず説明、コメント、依存関係を読み、タスクの原文を自分の理解で役割とプロンプトにまとめて、専用の worktree で作業するサブエージェントに渡します。自分ではコードを書きません。割り当て前に、タスク専用の worktree とブランチを用意します。サブエージェントは最初に読み取りだけを行い、開発計画をあなたに返します。あなたが comment_card でタスクに計画を投稿し、保存されたことを確認してから開発を始めさせます。同時に進めるタスク数は、人から伝えられた並行数までです（指定がなければ 2）。タスクが不明確、または人の判断が必要なときは、先にタスクへ疑問をコメントし、タスクを「未着手」のような担当対象外の列へ戻して返事を待ち、勝手に始めないでください。担当したら、タスクの内容に合わせて update_card でタグを調整します。選べるのはボードに既にあるタグだけ（tag_ids を使う）で、新しいタグは作らず、合うものがなければ付けません。finish_card を呼ぶ前に、update_card で説明の末尾にタスク項目の一覧を追記します。1 項目 1 行で、実装したものは「- [x] 内容」、残っているものは「- [ ] 内容」と書き、説明を見るだけで何を実装したか分かるようにします。 終わったら finish_card を呼び、要約とブランチ名、リンクを記入します。タスクは自動的に次の列へ移ります。タスクが完了したかどうかはあなたが決めることではないので、タスクを完了列へ移さないでください。",
			},
			textReview: {
				Name:        "レビュー",
				Instruction: "この列はレビュー用です。サブエージェントにレビューを任せられるのは、この列のタスクだけで、レビューは検収のみでコードは変更しません。タスクを担当したら、説明、コメント、開発側の引き継ぎ要約、ブランチを読み、タスクの原文を自分の理解でレビュー要件にまとめ、サブエージェントに 1 項目ずつ検収させ、テストを実行し、必要なら実際に動かさせます。サブエージェントは最初に読み取りだけを行い、レビュー計画をあなたに返します。あなたが comment_card でタスクに計画を投稿し、保存されたことを確認してから検収を始めさせます。comment_card で結論と証拠を投稿します。画面の検収では実際の画面のスクリーンショットを添付し、機能の検収では各項目に根拠と結果（- [x] 合格、- [ ] 不合格または未確認）を記録します。必要な証拠がない場合は理由を記載し、合格判定を保留します。合格：レビュー結果をタスクにコメントします。ローカルの専用 Slate ブランチがある場合は、Slate スキルの第8節に従って取り込み、結果を記録して完了列に移し、作業領域を片付けます。それ以外は完了列へ移します。不合格：どこが違うか、どう直すかをコメントしてから、エージェントが作業を担当する列（通常は「進行中」）へタスクを戻し、開発側にやり直させます。",
			},
			textDone: {
				Name:        "完了",
				Instruction: "この列は完了したタスクです。エージェントは一切操作しないでください。この列のタスクの担当、移動、編集、コメントも、サブエージェントの割り当ても禁止です。タスクを移せるのは、レビューが合格したときのレビューだけで、それ以外の移動や編集は人が行います。",
			},
		},
		RuleStart: "開始時刻を記録", RuleFinish: "完了時刻を記録", RuleReopen: "再オープン",
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
			BoardID: boardID, Name: t.Name, Instruction: t.Instruction,
			IsDone: s.IsDone,
			Sort:   int64(i+1) * columnSortStep, CreatedAt: now, UpdatedAt: now,
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
// 以模板的进行中列名称识别开始列、"完成列"当作完成，所以只要模板里有这样的列就有对应的规则：
// 进入进行中写开始时间、进入完成写完成时间、离开完成清空完成时间。写时间都只在为空时才写，
// 不会覆盖人手动填的值。empty 模板没有列，也就没有规则。
func defaultRules(texts localeTexts, cols []*Column) []*Rule {
	var doing, done *Column
	for _, c := range cols {
		if c.Name == texts.Columns[textDoing].Name && doing == nil {
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
