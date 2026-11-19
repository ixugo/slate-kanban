package mcpapi

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"slate-kanban/internal/core/kanban"
)

// okOut 是"操作完成、没有别的要返回"的工具的输出，例如释放领取和各种删除。
type okOut struct {
	OK bool `json:"ok"`
}

// nextCardOut 是 next_card 的输出。没有可做的卡片时 card 是 null，不是错误，此时没有 agent_notice。
type nextCardOut struct {
	Card   *kanban.CardDetail `json:"card"`
	Notice string             `json:"agent_notice,omitempty"`
}

// nextCardIn 是 next_card 的入参。
type nextCardIn struct {
	BoardID      int64 `json:"board_id" jsonschema:"看板编号，来自 list_boards"`
	LeaseMinutes int   `json:"lease_minutes,omitempty" jsonschema:"领取的租约分钟数，默认 30，最多 1440"`
}

// claimCardIn 是 claim_card 的入参。
type claimCardIn struct {
	CardID       int64 `json:"card_id" jsonschema:"卡片编号，是数字 id，不是 SLATE-12 这样的 key"`
	LeaseMinutes int   `json:"lease_minutes,omitempty" jsonschema:"领取的租约分钟数，默认 30，最多 1440"`
}

// finishCardIn 是 finish_card 的入参。
type finishCardIn struct {
	CardID   int64  `json:"card_id" jsonschema:"卡片编号，是数字 id，不是 SLATE-12 这样的 key"`
	Summary  string `json:"summary" jsonschema:"这次做了什么、结果如何、审查的人要注意什么；会作为评论写在卡片下"`
	Branch   string `json:"branch,omitempty" jsonschema:"这次工作用的分支名；不填表示不改"`
	URL      string `json:"url,omitempty" jsonschema:"相关链接，例如合并请求，必须以 http:// 或 https:// 开头；不填表示不改"`
	ColumnID int64  `json:"column_id,omitempty" jsonschema:"交接到哪一列；不填就去当前列设置的下一列（通常是审查）；按各列 instruction 的约定选择"`
	BeforeID int64  `json:"before_id,omitempty" jsonschema:"放到目标列这张卡片（数字 id）的前面；和 after_id 最多填一个；都不填放到目标列最顶部"`
	AfterID  int64  `json:"after_id,omitempty" jsonschema:"放到目标列这张卡片（数字 id）的后面；和 before_id 最多填一个"`
}

// commentCardIn 是 comment_card 的入参。
type commentCardIn struct {
	CardID int64  `json:"card_id" jsonschema:"卡片编号，是数字 id，不是 SLATE-12 这样的 key"`
	Body   string `json:"body" jsonschema:"评论正文，1 到 20000 字；评论只能新增，不能修改"`
}

// createCardIn 是创建卡片的入参，agent 视图只有这些字段。
type createCardIn struct {
	BoardID     int64    `json:"board_id" jsonschema:"看板编号，来自 list_boards"`
	Title       string   `json:"title" jsonschema:"标题，1 到 200 字，单行"`
	Description string   `json:"description,omitempty" jsonschema:"详细描述，最多 20000 字；写清要做什么和验收标准"`
	ColumnID    int64    `json:"column_id,omitempty" jsonschema:"放进哪一列；不填放进第一列；新卡片放在该列最顶部"`
	Tags        []string `json:"tags,omitempty" jsonschema:"标签文字，最多 10 个，每个 1 到 30 字；看板标签库里没有的文字会新建一个没有颜色的标签；和 tag_ids 只填一个"`
	TagIDs      []int64  `json:"tag_ids,omitempty" jsonschema:"标签主键，来自 get_board 的 tags；可以选没有文字的标签；和 tags 只填一个"`
	Branch      string   `json:"branch,omitempty" jsonschema:"关联的分支名"`
	URL         string   `json:"url,omitempty" jsonschema:"关联的链接，必须以 http:// 或 https:// 开头"`
}

// createCardAdminIn 是管理视图的创建卡片入参：在 agent 的字段之外还有规划类字段，这些只有人能写。
type createCardAdminIn struct {
	createCardIn
	Priority   int     `json:"priority,omitempty" jsonschema:"优先级：0 未设置，1 低，2 中，3 高"`
	HumanOnly  bool    `json:"human_only,omitempty" jsonschema:"为 true 表示只能由人处理，agent 不会领取"`
	WaitsOn    []int64 `json:"waits_on,omitempty" jsonschema:"依赖的卡片编号（数字 id）；依赖没完成之前这张卡处于等待状态"`
	DueAt      string  `json:"due_at,omitempty" jsonschema:"到期时间，RFC3339 格式，例如 2026-12-01T18:00:00+08:00"`
	StartedAt  string  `json:"started_at,omitempty" jsonschema:"开始时间，RFC3339 格式；通常由自动化规则写入"`
	FinishedAt string  `json:"finished_at,omitempty" jsonschema:"完成时间，RFC3339 格式；通常由自动化规则写入"`
}

// updateCardIn 是修改卡片的入参，agent 视图只有这些字段。指针字段为 null 或不填都表示不改。
type updateCardIn struct {
	CardID      int64     `json:"card_id" jsonschema:"卡片编号，是数字 id，不是 SLATE-12 这样的 key"`
	Version     *int64    `json:"version,omitempty" jsonschema:"你上次读到的 version；带上后，卡片已被别人改过就返回 ErrCardVersionConflict，而不是覆盖对方的修改"`
	Title       *string   `json:"title,omitempty" jsonschema:"新标题；不填不改"`
	Description *string   `json:"description,omitempty" jsonschema:"新描述，整体替换；不填不改"`
	Tags        *[]string `json:"tags,omitempty" jsonschema:"新标签文字列表，整体替换（没有文字的标签会被去掉，要保留请用 tag_ids）；传空数组清空；不填不改；和 tag_ids 只填一个"`
	TagIDs      *[]int64  `json:"tag_ids,omitempty" jsonschema:"新标签主键列表，来自 get_board 的 tags，整体替换；传空数组清空；不填不改；和 tags 只填一个"`
	Branch      *string   `json:"branch,omitempty" jsonschema:"分支名；传空字符串清除；不填不改"`
	URL         *string   `json:"url,omitempty" jsonschema:"链接，必须以 http:// 或 https:// 开头；传空字符串清除；不填不改"`
}

// updateCardAdminIn 是管理视图的修改卡片入参：在 agent 的字段之外还有规划类字段。
type updateCardAdminIn struct {
	updateCardIn
	Priority   *int     `json:"priority,omitempty" jsonschema:"优先级：0 未设置，1 低，2 中，3 高；不填不改"`
	HumanOnly  *bool    `json:"human_only,omitempty" jsonschema:"是否只能由人处理；不填不改"`
	WaitsOn    *[]int64 `json:"waits_on,omitempty" jsonschema:"依赖的卡片编号，整体替换；传空数组清空；不填不改"`
	DueAt      *string  `json:"due_at,omitempty" jsonschema:"到期时间，RFC3339 格式；传空字符串清除；不填不改"`
	StartedAt  *string  `json:"started_at,omitempty" jsonschema:"开始时间，RFC3339 格式；传空字符串清除；不填不改"`
	FinishedAt *string  `json:"finished_at,omitempty" jsonschema:"完成时间，RFC3339 格式；传空字符串清除；不填不改"`
}

// moveCardIn 是 move_card 的入参。
type moveCardIn struct {
	CardID   int64 `json:"card_id" jsonschema:"卡片编号，是数字 id，不是 SLATE-12 这样的 key"`
	ColumnID int64 `json:"column_id,omitempty" jsonschema:"移到哪一列；不填表示留在当前列，只调整顺序"`
	BeforeID int64 `json:"before_id,omitempty" jsonschema:"放到这张卡片（数字 id）的前面；和 after_id 最多填一个"`
	AfterID  int64 `json:"after_id,omitempty" jsonschema:"放到这张卡片（数字 id）的后面；和 before_id 最多填一个；都不填放到目标列最顶部"`
}

// setProjectPathIn 是 set_project_path 的入参。
type setProjectPathIn struct {
	BoardID     int64  `json:"board_id" jsonschema:"看板编号，来自 list_boards"`
	ProjectPath string `json:"project_path" jsonschema:"这个看板对应的项目在本机的绝对路径，例如 /Users/me/work/slate"`
}

// input 把 agent 视图的创建入参换成 Core 的入参。规划类字段保持零值，也就是"没有填"。
func (in createCardIn) input() kanban.CreateCardInput {
	return kanban.CreateCardInput{
		Title: in.Title, Description: in.Description, ColumnID: in.ColumnID,
		Tags: in.Tags, TagIDs: in.TagIDs, Branch: in.Branch, URL: in.URL,
	}
}

// input 把管理视图的创建入参换成 Core 的入参，带上规划类字段。
func (in createCardAdminIn) input() kanban.CreateCardInput {
	out := in.createCardIn.input()
	out.Priority, out.HumanOnly, out.WaitsOn = in.Priority, in.HumanOnly, in.WaitsOn
	out.DueAt, out.StartedAt, out.FinishedAt = in.DueAt, in.StartedAt, in.FinishedAt
	return out
}

// patch 把 agent 视图的修改入参换成 Core 的入参。规划类字段保持 nil，也就是"不修改"。
func (in updateCardIn) patch() kanban.UpdateCardInput {
	return kanban.UpdateCardInput{
		Title: in.Title, Description: in.Description, Tags: in.Tags, TagIDs: in.TagIDs,
		Branch: in.Branch, URL: in.URL, Version: in.Version,
	}
}

// patch 把管理视图的修改入参换成 Core 的入参，带上规划类字段。
func (in updateCardAdminIn) patch() kanban.UpdateCardInput {
	out := in.updateCardIn.patch()
	out.Priority, out.HumanOnly, out.WaitsOn = in.Priority, in.HumanOnly, in.WaitsOn
	out.DueAt, out.StartedAt, out.FinishedAt = in.DueAt, in.StartedAt, in.FinishedAt
	return out
}

// registerWorkTools 注册干活用的工具。两个视图都有的几个（释放、交接、评论、移动）先注册；
// 之后 agent 视图加上领取类、创建和修改卡片、写项目路径，管理视图加上带规划类字段的创建和修改。
func (a API) registerWorkTools(s *mcp.Server, v view) {
	a.registerSharedWork(s)
	if v == viewAdmin {
		a.registerAdminCards(s)
		return
	}
	a.registerAgentWork(s)
}

// registerSharedWork 注册人和 agent 都有的干活工具。权限上的差别由 Core 判断：
// agent 只能动自己持有的卡片，人可以动任何卡片。
func (a API) registerSharedWork(s *mcp.Server) {
	register(s, spec{
		name: "release_card", kind: kindWrite,
		desc: "释放卡片上的领取。你做不下去、想让别的 agent 接手时用它；释放后卡片留在原来的列，所以你自己的 next_card 也会再领到它。要等人回复的话，改用 move_card 把卡片移出领取列。" +
			"agent 只能释放自己持有的卡片，人可以收回任何一张。已经没有领取时也返回成功。",
	}, a.releaseCard)
	register(s, spec{
		name: "finish_card", kind: kindWrite,
		desc: "完成交接：把 summary 写成评论、记下分支和链接、把卡片移到下一列（通常是「审查」）并结束你的领取，全部在一次操作里完成。" +
			"做完一张卡片就调用它，不要自己用 move_card 去拖；是否算完成由审查决定，具体按各列的 instruction。" +
			"summary 要让审查的人不看代码也能知道做了什么、怎么验证的、有什么风险。",
	}, a.finishCard)
	register(s, spec{
		name: "comment_card", kind: kindWrite,
		desc: "在卡片下写一条评论，用来汇报进度、提问或回复人的问题。不必持有这张卡片也能评论；" +
			"持有者评论会顺延领取的租约。评论只能新增，不能修改。",
	}, a.commentCard)
	register(s, spec{
		name: "move_card", kind: kindWrite,
		desc: "把卡片移到另一列，或在同一列里调整顺序。换列会结束领取，并触发看板上对应的自动化规则。" +
			"不能移动正被别的 agent 持有的卡片；什么状态下该动哪张卡、能放进哪一列，按卡片所在列和目标列的 instruction 来（例如审查不通过时放回「进行中」）。做完一张卡片请用 finish_card，不要用它。",
	}, a.moveCard)
}

// registerAgentWork 注册只有 agent 视图才有的干活工具。
func (a API) registerAgentWork(s *mcp.Server) {
	register(s, spec{
		name: "next_card", kind: kindWrite,
		desc: "领取下一张该你做的卡片。只从 agent_pickup 为 true 的列（通常叫「进行中」）里取，顺序和看板上一致（列从左到右、同一列从上到下），" +
			"跳过只能由人处理的、还在等依赖的和别人正在做的；你自己已经持有的卡片优先，所以重启之后能接着做原来的活。" +
			"领到后返回卡片全文、评论、依赖和所在列的指令（column_instruction，必须遵守）。" +
			"没有可做的卡片时 card 为 null，不是错误：用 wait_for_updates 等人安排新工作。领到卡片时结果带 agent_notice：任务不明确就先评论反馈，不要直接开始。" +
			"领取有租约（lease_minutes，默认 30 分钟，最多 1440 分钟）：持有期间在卡片上评论或修改会把租约顺延到至少 30 分钟之后，" +
			"租约到期后别的 agent 可以接手。",
	}, a.nextCard)
	register(s, spec{
		name: "claim_card", kind: kindWrite,
		desc: "领取指定的卡片，已经由你持有时只是续租。哪一列的卡片你能领，按这一列的 instruction 来（例如审查 agent 领审查列的卡）；卡片不是只能由人处理的，" +
			"依赖已经完成，并且没有被别的 agent 持有。领到后返回卡片全文、评论、依赖和所在列的指令，以及 agent_notice 提醒（任务不明确就先评论反馈）。一般用 next_card 就够了。",
	}, a.claimCard)
	register(s, spec{
		name: "create_card", kind: kindWrite,
		desc: "创建卡片，用来记录你在工作中发现的新任务、问题或后续事项。放进哪一列按各列 instruction 的约定，没有约定就建在「收件箱」，" +
			"由人决定做不做、什么时候做。优先级、依赖和时间是人的安排，agent 不能设置。",
	}, a.createCard)
	register(s, spec{
		name: "update_card", kind: kindWrite,
		desc: "修改你持有的卡片的标题、描述、标签、分支或链接，只更新你填写的字段。" +
			"带上 version（上次读到的版本号）可以避免覆盖别人刚做的修改。优先级、依赖和时间是人的安排，agent 不能修改。",
	}, a.updateCard)
	register(s, spec{
		name: "set_project_path", kind: kindWrite,
		desc: "记录这个看板对应的项目在本机的绝对路径。只有看板上还没有路径时才能写；已经有路径了就不能改，只有人可以改。" +
			"先用 get_board 看 project_path，是空的才需要问用户并用它写下来。",
	}, a.setProjectPath)
}

// registerAdminCards 注册管理视图里带规划类字段的创建和修改卡片。
func (a API) registerAdminCards(s *mcp.Server) {
	register(s, spec{
		name: "create_card", kind: kindWrite,
		desc: "创建卡片。可以直接设置优先级、仅人处理、依赖、到期时间等规划类字段；column_id 不填放进第一列。",
	}, a.createCardAdmin)
	register(s, spec{
		name: "update_card", kind: kindWrite,
		desc: "修改卡片，只更新填写的字段，包括优先级、仅人处理、依赖和三个时间。时间用 RFC3339 格式，传空字符串清除。" +
			"带上 version（上次读到的版本号）可以避免覆盖别人刚做的修改。",
	}, a.updateCardAdmin)
}

// nextCard 为 agent 领取下一张该做的卡片，没有时返回 card 为 null。
func (a API) nextCard(ctx context.Context, actor kanban.Actor, in nextCardIn) (any, error) {
	card, err := a.core.NextCard(ctx, actor, in.BoardID, kanban.NextCardInput{LeaseMinutes: in.LeaseMinutes})
	if err != nil {
		return nil, err
	}
	out := nextCardOut{Card: card}
	if card != nil {
		out.Notice = unclearTaskNotice
	}
	return out, nil
}

// claimCard 领取指定的卡片，返回卡片全文和「任务不清楚先反馈」的提示。
func (a API) claimCard(ctx context.Context, actor kanban.Actor, in claimCardIn) (any, error) {
	detail, err := a.core.ClaimCard(ctx, actor, in.CardID, kanban.ClaimCardInput{LeaseMinutes: in.LeaseMinutes})
	if err != nil {
		return nil, err
	}
	return withNotice(detail), nil
}

// releaseCard 释放领取。
func (a API) releaseCard(ctx context.Context, actor kanban.Actor, in cardIn) (any, error) {
	return finished(a.core.ReleaseCard(ctx, actor, in.CardID))
}

// finishCard 完成交接，返回交接后的卡片摘要（带着新的所在列），不回显评论和描述。
func (a API) finishCard(ctx context.Context, actor kanban.Actor, in finishCardIn) (any, error) {
	detail, err := a.core.FinishCard(ctx, actor, in.CardID, kanban.FinishCardInput{
		Summary: in.Summary, Branch: in.Branch, URL: in.URL, ColumnID: in.ColumnID,
		BeforeID: in.BeforeID, AfterID: in.AfterID, Top: true,
	})
	if err != nil {
		return nil, err
	}
	return summarize(detail.CardView), nil
}

// commentCard 写一条评论，返回写下的评论。
func (a API) commentCard(ctx context.Context, actor kanban.Actor, in commentCardIn) (any, error) {
	return a.core.AddComment(ctx, actor, in.CardID, in.Body)
}

// createCard 以 agent 的字段创建卡片。
func (a API) createCard(ctx context.Context, actor kanban.Actor, in createCardIn) (any, error) {
	return a.makeCard(ctx, actor, in.BoardID, in.input())
}

// createCardAdmin 以管理视图的字段创建卡片，规划类字段也会写入。
func (a API) createCardAdmin(ctx context.Context, actor kanban.Actor, in createCardAdminIn) (any, error) {
	return a.makeCard(ctx, actor, in.BoardID, in.input())
}

// makeCard 创建卡片并返回简要字段。两个视图的创建共用它，权限和校验都在 Core；
// 通过 MCP 创建的卡片放在所在列的最顶部，最新的在最上面。
func (a API) makeCard(ctx context.Context, actor kanban.Actor, boardID int64, in kanban.CreateCardInput) (any, error) {
	in.Top = true
	card, err := a.core.CreateCard(ctx, actor, boardID, in)
	if err != nil {
		return nil, err
	}
	return a.briefOf(ctx, card)
}

// updateCard 以 agent 的字段修改卡片。
func (a API) updateCard(ctx context.Context, actor kanban.Actor, in updateCardIn) (any, error) {
	return a.saveCard(ctx, actor, in.CardID, in.patch())
}

// updateCardAdmin 以管理视图的字段修改卡片，规划类字段也可以改。
func (a API) updateCardAdmin(ctx context.Context, actor kanban.Actor, in updateCardAdminIn) (any, error) {
	return a.saveCard(ctx, actor, in.CardID, in.patch())
}

// saveCard 修改卡片并返回简要字段。两个视图的修改共用它，权限和校验都在 Core。
func (a API) saveCard(ctx context.Context, actor kanban.Actor, cardID int64, patch kanban.UpdateCardInput) (any, error) {
	card, err := a.core.UpdateCard(ctx, actor, cardID, patch)
	if err != nil {
		return nil, err
	}
	return a.briefOf(ctx, card)
}

// moveCard 移动卡片并返回简要字段。
func (a API) moveCard(ctx context.Context, actor kanban.Actor, in moveCardIn) (any, error) {
	card, err := a.core.MoveCard(ctx, actor, in.CardID, kanban.MoveCardInput{
		ColumnID: in.ColumnID, BeforeID: in.BeforeID, AfterID: in.AfterID, Top: true,
	})
	if err != nil {
		return nil, err
	}
	return a.briefOf(ctx, card)
}

// briefOf 取出写入类工具返回的简要字段，tags 的文字在这里从看板的标签库里查出来。
func (a API) briefOf(ctx context.Context, card *kanban.Card) (any, error) {
	names, err := a.core.CardTagNames(ctx, card)
	if err != nil {
		return nil, err
	}
	return briefOf(card, names), nil
}

// setProjectPath 为 agent 写看板的项目路径。路径不能为空：
// 空路径在人那里表示清除，但 agent 只能写入，所以在这里直接拒绝，免得"写了个空"被当成成功。
func (a API) setProjectPath(ctx context.Context, actor kanban.Actor, in setProjectPathIn) (any, error) {
	path := strings.TrimSpace(in.ProjectPath)
	if path == "" {
		return nil, kanban.ErrInvalidInput.WithMsg("项目路径不能为空")
	}
	return a.core.UpdateBoard(ctx, actor, in.BoardID, kanban.UpdateBoardInput{ProjectPath: &path})
}
