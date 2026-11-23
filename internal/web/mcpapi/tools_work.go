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

// nextCardOut 是 next_card 的输出。没有可做的任务时 card 是 null，不是错误，此时没有 agent_notice。
type nextCardOut struct {
	Card   *kanban.CardDetail `json:"card"`
	Notice string             `json:"agent_notice,omitempty"`
}

// nextCardIn 是 next_card 的入参。
type nextCardIn struct {
	ColumnID     int64 `json:"column_id" jsonschema:"必填，先读取 get_board 各列的 instruction，再选择应由你处理的列编号"`
	BoardID      int64 `json:"board_id" jsonschema:"看板编号，来自 list_boards"`
	LeaseMinutes int   `json:"lease_minutes,omitempty" jsonschema:"领取的租约分钟数，默认 30，最多 1440"`
	EngineerID   int64 `json:"engineer_id,omitempty" jsonschema:"由哪位工程师承担（按列提示词选择研发或测试），来自 get_board 的 engineers；不填则由服务端挑一位空闲的；看板没有工程师时忽略"`
}

// claimCardIn 是 claim_card 的入参。
type claimCardIn struct {
	CardID       int64 `json:"card_id" jsonschema:"任务编号，是数字 id，不是 SLATE-12 这样的 key"`
	LeaseMinutes int   `json:"lease_minutes,omitempty" jsonschema:"领取的租约分钟数，默认 30，最多 1440"`
	EngineerID   int64 `json:"engineer_id,omitempty" jsonschema:"由哪位工程师承担，来自 get_board 的 engineers，按列提示词选择研发类或测试类型；不填则任务绑定的优先，否则服务端挑一位空闲的；看板没有工程师时忽略"`
}

// finishCardIn 是 finish_card 的入参。
type finishCardIn struct {
	CardID        int64    `json:"card_id" jsonschema:"任务编号，是数字 id，不是 SLATE-12 这样的 key"`
	MergedCommits []string `json:"merged_commits,omitempty" jsonschema:"仅审查通过的本地任务：已合入主工作区的提交哈希前六位，保持顺序；同时传 version，工具会把列表加进评论"`
	Version       *int64   `json:"version,omitempty" jsonschema:"记录合并结果时必填，使用审查时读到的任务版本"`
	Summary       string   `json:"summary" jsonschema:"这次做了什么、结果如何、审查的人要注意什么；会作为评论写在任务下；引用同板任务时在正文写 #数字，例如 #86"`
	Branch        string   `json:"branch,omitempty" jsonschema:"这次工作用的分支名；不填表示不改"`
	URL           string   `json:"url,omitempty" jsonschema:"相关链接，例如合并请求，必须以 http:// 或 https:// 开头；不填表示不改"`
	ColumnID      int64    `json:"column_id,omitempty" jsonschema:"交接到哪一列；不填就去当前列设置的下一列（通常是审查）；按各列 instruction 的约定选择"`
	BeforeID      int64    `json:"before_id,omitempty" jsonschema:"放到目标列这个任务（数字 id）的前面；和 after_id 最多填一个；都不填放到目标列最顶部"`
	AfterID       int64    `json:"after_id,omitempty" jsonschema:"放到目标列这个任务（数字 id）的后面；和 before_id 最多填一个"`
}

// commentCardIn 是 comment_card 的入参。
type commentCardIn struct {
	CardID int64  `json:"card_id" jsonschema:"任务编号，是数字 id，不是 SLATE-12 这样的 key"`
	Body   string `json:"body" jsonschema:"评论正文，1 到 20000 字；引用同板任务时在正文写 #数字，例如 #86，网页上可悬浮和点击；评论只能新增，不能修改"`
}

// createCardIn 是创建任务的入参，agent 视图只有这些字段。
type createCardIn struct {
	BoardID     int64    `json:"board_id" jsonschema:"看板编号，来自 list_boards"`
	Title       string   `json:"title" jsonschema:"标题，1 到 200 字，单行"`
	Description string   `json:"description,omitempty" jsonschema:"详细描述，最多 20000 字；写清要做什么和验收标准"`
	ColumnID    int64    `json:"column_id,omitempty" jsonschema:"放进哪一列；不填放进第一列；新任务放在该列最顶部"`
	Tags        []string `json:"tags,omitempty" jsonschema:"标签文字，最多 10 个，每个 1 到 30 字；看板标签库里没有的文字会新建一个没有颜色的标签，agent 不要这样新建，只贴已有的标签，优先用 tag_ids；和 tag_ids 只填一个"`
	TagIDs      []int64  `json:"tag_ids,omitempty" jsonschema:"标签主键，来自 get_board 的 tags；agent 只能贴这里已有的标签；可以选没有文字的标签；和 tags 只填一个"`
	Branch      string   `json:"branch,omitempty" jsonschema:"关联的分支名"`
	URL         string   `json:"url,omitempty" jsonschema:"关联的链接，必须以 http:// 或 https:// 开头"`
}

// createCardAdminIn 是管理视图的创建任务入参：在 agent 的字段之外还有规划类字段，这些只有人能写。
type createCardAdminIn struct {
	createCardIn
	Priority   int     `json:"priority,omitempty" jsonschema:"优先级：0 未设置，1 低，2 中，3 高"`
	HumanOnly  bool    `json:"human_only,omitempty" jsonschema:"为 true 表示只能由人处理，agent 不会领取"`
	EngineerID int64   `json:"engineer_id,omitempty" jsonschema:"绑定一位研发类工程师（全栈、前端、后端、UI，来自 get_board 的 engineers），领取时由他承担；不填表示领取时随机分配"`
	WaitsOn    []int64 `json:"waits_on,omitempty" jsonschema:"依赖的任务编号（数字 id）；依赖没完成之前这个任务处于等待状态"`
	DueAt      string  `json:"due_at,omitempty" jsonschema:"到期时间，RFC3339 格式，例如 2026-12-01T18:00:00+08:00"`
	StartedAt  string  `json:"started_at,omitempty" jsonschema:"开始时间，RFC3339 格式；通常由自动化规则写入"`
	FinishedAt string  `json:"finished_at,omitempty" jsonschema:"完成时间，RFC3339 格式；通常由自动化规则写入"`
}

// updateCardIn 是修改任务的入参，agent 视图只有这些字段。指针字段为 null 或不填都表示不改。
type updateCardIn struct {
	CardID      int64     `json:"card_id" jsonschema:"任务编号，是数字 id，不是 SLATE-12 这样的 key"`
	Version     *int64    `json:"version,omitempty" jsonschema:"你上次读到的 version；带上后，任务已被别人改过就返回 ErrCardVersionConflict，而不是覆盖对方的修改"`
	Title       *string   `json:"title,omitempty" jsonschema:"新标题；不填不改"`
	Description *string   `json:"description,omitempty" jsonschema:"新描述，整体替换；不填不改"`
	Tags        *[]string `json:"tags,omitempty" jsonschema:"新标签文字列表，整体替换（没有文字的标签会被去掉，要保留请用 tag_ids）；agent 不要用它新建标签，优先用 tag_ids；传空数组清空；不填不改；和 tag_ids 只填一个"`
	TagIDs      *[]int64  `json:"tag_ids,omitempty" jsonschema:"新标签主键列表，来自 get_board 的 tags，整体替换；agent 只能贴已有的标签；传空数组清空；不填不改；和 tags 只填一个"`
	Branch      *string   `json:"branch,omitempty" jsonschema:"分支名；传空字符串清除；不填不改"`
	URL         *string   `json:"url,omitempty" jsonschema:"链接，必须以 http:// 或 https:// 开头；传空字符串清除；不填不改"`
}

// updateCardAdminIn 是管理视图的修改任务入参：在 agent 的字段之外还有规划类字段。
type updateCardAdminIn struct {
	updateCardIn
	Priority   *int     `json:"priority,omitempty" jsonschema:"优先级：0 未设置，1 低，2 中，3 高；不填不改"`
	HumanOnly  *bool    `json:"human_only,omitempty" jsonschema:"是否只能由人处理；不填不改"`
	EngineerID *int64   `json:"engineer_id,omitempty" jsonschema:"绑定的研发工程师编号，来自 get_board 的 engineers；传 0 取消绑定；不填不改"`
	WaitsOn    *[]int64 `json:"waits_on,omitempty" jsonschema:"依赖的任务编号，整体替换；传空数组清空；不填不改"`
	DueAt      *string  `json:"due_at,omitempty" jsonschema:"到期时间，RFC3339 格式；传空字符串清除；不填不改"`
	StartedAt  *string  `json:"started_at,omitempty" jsonschema:"开始时间，RFC3339 格式；传空字符串清除；不填不改"`
	FinishedAt *string  `json:"finished_at,omitempty" jsonschema:"完成时间，RFC3339 格式；传空字符串清除；不填不改"`
}

// moveCardIn 是 move_card 的入参。
type moveCardIn struct {
	CardID   int64 `json:"card_id" jsonschema:"任务编号，是数字 id，不是 SLATE-12 这样的 key"`
	ColumnID int64 `json:"column_id,omitempty" jsonschema:"移到哪一列；不填表示留在当前列，只调整顺序"`
	BeforeID int64 `json:"before_id,omitempty" jsonschema:"放到这个任务（数字 id）的前面；和 after_id 最多填一个"`
	AfterID  int64 `json:"after_id,omitempty" jsonschema:"放到这个任务（数字 id）的后面；和 before_id 最多填一个；都不填放到目标列最顶部"`
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
	out.EngineerID = in.EngineerID
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
	out.EngineerID = in.EngineerID
	return out
}

// registerWorkTools 注册干活用的工具。两个视图都有的几个（释放、交接、评论、移动）先注册；
// 之后 agent 视图加上领取类、创建和修改任务、写项目路径，管理视图加上带规划类字段的创建和修改。
func (a API) registerWorkTools(s *mcp.Server, v view) {
	a.registerSharedWork(s)
	if v == viewAdmin {
		a.registerAdminCards(s)
		return
	}
	a.registerAgentWork(s)
}

// registerSharedWork 注册人和 agent 都有的干活工具。权限上的差别由 Core 判断：
// agent 只能动自己持有的任务，人可以动任何任务。
func (a API) registerSharedWork(s *mcp.Server) {
	register(s, spec{
		name: "release_card", kind: kindWrite,
		desc: "释放领取，卡片仍在原列。人可收回领取，agent 只能释放自己持有的任务。",
	}, a.releaseCard)
	register(s, spec{
		name: "finish_card", kind: kindWrite,
		desc: "一次事务写交接评论、更新分支和链接、移到下一列并结束领取。按列指令和 skill 更新任务项及审查结果。",
	}, a.finishCard)
	register(s, spec{
		name: "comment_card", kind: kindWrite,
		desc: "新增任务评论。持有者评论顺延租约。",
	}, a.commentCard)
	register(s, spec{
		name: "move_card", kind: kindWrite,
		desc: "移动或重排任务。换列结束领取并触发自动化；研发交接用 finish_card。",
	}, a.moveCard)
}

// registerAgentWork 注册只有 agent 视图才有的干活工具。
func (a API) registerAgentWork(s *mcp.Server) {
	register(s, spec{
		name: "next_card", kind: kindWrite,
		desc: "按列顺序领取首张可做任务，优先返回自己已持有的；card 为 null 表示本列无任务。返回详情、评论及列指令。",
	}, a.nextCard)
	register(s, spec{
		name: "claim_card", kind: kindWrite,
		desc: "领取指定任务或续租，返回详情、评论及列指令。需满足依赖、工程师绑定和空闲名额。",
	}, a.claimCard)
	register(s, spec{
		name: "create_card", kind: kindWrite,
		desc: "创建任务；column_id 缺省放入第一列。优先级、依赖与时间由人管理。",
	}, a.createCard)
	register(s, spec{
		name: "update_card", kind: kindWrite,
		desc: "更新持有任务中填写的字段。带上 version 避免并发覆盖。",
	}, a.updateCard)
	register(s, spec{
		name: "set_project_path", kind: kindWrite,
		desc: "看板路径为空且人已明确确认后，登记本机绝对路径。已有路径由人修改。",
	}, a.setProjectPath)
}

// registerAdminCards 注册管理视图里带规划类字段的创建和修改任务。
func (a API) registerAdminCards(s *mcp.Server) {
	register(s, spec{
		name: "create_card", kind: kindWrite,
		desc: "创建任务。可以直接设置优先级、仅人处理、依赖、到期时间等规划类字段；column_id 不填放进第一列。",
	}, a.createCardAdmin)
	register(s, spec{
		name: "update_card", kind: kindWrite,
		desc: "修改任务，只更新填写的字段，包括优先级、仅人处理、依赖和三个时间。时间用 RFC3339 格式，传空字符串清除。" +
			"带上 version（上次读到的版本号）可以避免覆盖别人刚做的修改。",
	}, a.updateCardAdmin)
}

// nextCard 为 agent 领取下一个该做的任务，没有时返回 card 为 null。
func (a API) nextCard(ctx context.Context, actor kanban.Actor, in nextCardIn) (any, error) {
	card, err := a.core.NextCard(ctx, actor, in.BoardID, kanban.NextCardInput{ColumnID: in.ColumnID, LeaseMinutes: in.LeaseMinutes, EngineerID: in.EngineerID})
	if err != nil {
		return nil, err
	}
	out := nextCardOut{Card: card}
	if card != nil {
		out.Notice = unclearTaskNotice
	}
	return out, nil
}

// claimCard 领取指定的任务，返回任务全文和「任务不清楚先反馈」的提示。
func (a API) claimCard(ctx context.Context, actor kanban.Actor, in claimCardIn) (any, error) {
	detail, err := a.core.ClaimCard(ctx, actor, in.CardID, kanban.ClaimCardInput{LeaseMinutes: in.LeaseMinutes, EngineerID: in.EngineerID})
	if err != nil {
		return nil, err
	}
	return withNotice(detail), nil
}

// releaseCard 释放领取。
func (a API) releaseCard(ctx context.Context, actor kanban.Actor, in cardIn) (any, error) {
	return finished(a.core.ReleaseCard(ctx, actor, in.CardID))
}

// finishCard 完成交接，返回交接后的任务摘要（带着新的所在列），不回显评论和描述。
func (a API) finishCard(ctx context.Context, actor kanban.Actor, in finishCardIn) (any, error) {
	detail, err := a.core.FinishCard(ctx, actor, in.CardID, kanban.FinishCardInput{
		Summary: in.Summary, Branch: in.Branch, URL: in.URL, ColumnID: in.ColumnID,
		BeforeID: in.BeforeID, AfterID: in.AfterID, Top: true,
		MergedCommits: in.MergedCommits, Version: in.Version,
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

// createCard 以 agent 的字段创建任务。
func (a API) createCard(ctx context.Context, actor kanban.Actor, in createCardIn) (any, error) {
	return a.makeCard(ctx, actor, in.BoardID, in.input())
}

// createCardAdmin 以管理视图的字段创建任务，规划类字段也会写入。
func (a API) createCardAdmin(ctx context.Context, actor kanban.Actor, in createCardAdminIn) (any, error) {
	return a.makeCard(ctx, actor, in.BoardID, in.input())
}

// makeCard 创建任务并返回简要字段。两个视图的创建共用它，权限和校验都在 Core；
// 通过 MCP 创建的任务放在所在列的最顶部，最新的在最上面。
func (a API) makeCard(ctx context.Context, actor kanban.Actor, boardID int64, in kanban.CreateCardInput) (any, error) {
	in.Top = true
	card, err := a.core.CreateCard(ctx, actor, boardID, in)
	if err != nil {
		return nil, err
	}
	return a.briefOf(ctx, card)
}

// updateCard 以 agent 的字段修改任务。
func (a API) updateCard(ctx context.Context, actor kanban.Actor, in updateCardIn) (any, error) {
	return a.saveCard(ctx, actor, in.CardID, in.patch())
}

// updateCardAdmin 以管理视图的字段修改任务，规划类字段也可以改。
func (a API) updateCardAdmin(ctx context.Context, actor kanban.Actor, in updateCardAdminIn) (any, error) {
	return a.saveCard(ctx, actor, in.CardID, in.patch())
}

// saveCard 修改任务并返回简要字段。两个视图的修改共用它，权限和校验都在 Core。
func (a API) saveCard(ctx context.Context, actor kanban.Actor, cardID int64, patch kanban.UpdateCardInput) (any, error) {
	card, err := a.core.UpdateCard(ctx, actor, cardID, patch)
	if err != nil {
		return nil, err
	}
	return a.briefOf(ctx, card)
}

// moveCard 移动任务并返回简要字段。
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
