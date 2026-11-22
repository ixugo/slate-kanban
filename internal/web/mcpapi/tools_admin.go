package mcpapi

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"slate-kanban/internal/core/kanban"
)

// columnsOut 是 reorder_columns 的输出：重排后的列，从左到右。
type columnsOut struct {
	Columns []*kanban.Column `json:"columns"`
}

// createBoardIn 是 create_board 的入参。
type createBoardIn struct {
	Name        string `json:"name" jsonschema:"看板名称，1 到 100 字，单行"`
	Slug        string `json:"slug,omitempty" jsonschema:"看板标识，2 到 10 位，大写字母开头，只含大写字母和数字；会拼进每个任务的编号（例如 SLATE-12），创建后不能改；不填按名称生成"`
	Description string `json:"description,omitempty" jsonschema:"看板说明，最多 2000 字"`
	ProjectPath string `json:"project_path,omitempty" jsonschema:"这个看板对应的项目在本机的绝对路径"`
	Template    string `json:"template,omitempty" jsonschema:"起步模板：dev（收件箱、待办、进行中、审查、完成，默认）、simple（待办、进行中、完成）、empty（没有列）"`
	Locale      string `json:"locale,omitempty" jsonschema:"起步列名的语言：zh-CN、zh-TW、en、ja、ko；不填按 en"`
}

// updateBoardIn 是 update_board 的入参，指针字段不填或为 null 都表示不改。
type updateBoardIn struct {
	BoardID     int64   `json:"board_id" jsonschema:"看板编号，来自 list_boards"`
	Name        *string `json:"name,omitempty" jsonschema:"新名称；不填不改"`
	Description *string `json:"description,omitempty" jsonschema:"新说明；不填不改"`
	ProjectPath *string `json:"project_path,omitempty" jsonschema:"新的项目路径，必须是绝对路径；传空字符串清除；不填不改"`
}

// createColumnIn 是 create_column 的入参。
type createColumnIn struct {
	BoardID      int64  `json:"board_id" jsonschema:"看板编号，来自 list_boards"`
	Name         string `json:"name" jsonschema:"列名称，1 到 60 字，同一个看板里不能重名"`
	Instruction  string `json:"instruction,omitempty" jsonschema:"给 agent 的做事规则，最多 4000 字；agent 领到这一列的任务时必须遵守"`
	IsDone       bool   `json:"is_done,omitempty" jsonschema:"为 true 表示这是完成列，进入这一列的任务算已完成"`
	NextColumnID int64  `json:"next_column_id,omitempty" jsonschema:"agent 用 finish_card 交接后，任务去的下一列（列编号）；不填表示没有"`
}

// updateColumnIn 是 update_column 的入参，指针字段不填或为 null 都表示不改。
type updateColumnIn struct {
	BoardID      int64   `json:"board_id" jsonschema:"看板编号，来自 list_boards"`
	ColumnID     int64   `json:"column_id" jsonschema:"列编号，来自 get_board"`
	Name         *string `json:"name,omitempty" jsonschema:"新名称；不填不改"`
	Instruction  *string `json:"instruction,omitempty" jsonschema:"新的 agent 指令；传空字符串清除；不填不改"`
	IsDone       *bool   `json:"is_done,omitempty" jsonschema:"是否是完成列；不填不改"`
	NextColumnID *int64  `json:"next_column_id,omitempty" jsonschema:"交接后的下一列（列编号）；传 0 清除；不填不改"`
}

// reorderColumnsIn 是 reorder_columns 的入参。
type reorderColumnsIn struct {
	BoardID int64   `json:"board_id" jsonschema:"看板编号，来自 list_boards"`
	IDs     []int64 `json:"ids" jsonschema:"列编号，按希望的从左到右顺序排列；必须包含看板的全部列，每列恰好一次"`
}

// deleteColumnIn 是 delete_column 的入参。
type deleteColumnIn struct {
	BoardID  int64 `json:"board_id" jsonschema:"看板编号，来自 list_boards"`
	ColumnID int64 `json:"column_id" jsonschema:"要删除的列编号"`
	MoveTo   int64 `json:"move_to,omitempty" jsonschema:"列里有任务时，把它们移入的列编号（追加到末尾）；列是空的不用填"`
}

// deleteCommentIn 是 delete_comment 的入参。
type deleteCommentIn struct {
	CommentID int64 `json:"comment_id" jsonschema:"评论编号，在 get_card 返回的 comments 里"`
}

// finished 把只返回 error 的 Core 方法的结果换成工具的输出：成功时是 {"ok": true}。
func finished(err error) (any, error) {
	if err != nil {
		return nil, err
	}
	return okOut{OK: true}, nil
}

// registerAdminTools 注册只有管理视图才有的工具：看板（项目）的修改、列、规则的管理，以及任务和评论的删除。
// 它们对应的 Core 方法都只允许人调用，agent 视图里没有这些工具，减少它的干扰。
// 项目只能创建和修改，不提供删除：删除项目只能由人在网页上输入项目名称确认后完成，MCP 不开这个口子。
func (a API) registerAdminTools(s *mcp.Server) {
	a.registerBoardTools(s)
	a.registerColumnTools(s)
	a.registerRuleTools(s)
	register(s, spec{
		name: "delete_card", kind: kindDestroy,
		desc: "删除一个任务和它的全部评论，不可恢复。任务已经不存在也返回成功。",
	}, a.deleteCard)
	register(s, spec{
		name: "delete_comment", kind: kindDestroy,
		desc: "删除一条评论，不可恢复。评论已经不存在也返回成功。",
	}, a.deleteComment)
}

// registerCreateBoard 注册 create_board。工作视图和管理视图都有这个工具：
// agent 在用户要求时也能新建项目，但没有任何视图能删除项目，见 registerAdminTools。
func (a API) registerCreateBoard(s *mcp.Server) {
	register(s, spec{
		name: "create_board", kind: kindWrite,
		desc: "创建看板。template 决定起步的列：dev（收件箱、待办、进行中、审查、完成，带开始和完成计时的自动化规则，默认）、" +
			"simple（待办、进行中、完成）、empty（没有列，再用 create_column 自己搭）；locale 决定起步列名的语言。" +
			"返回看板、列和规则，列的编号用于后面的 create_card、update_column 等。",
	}, a.createBoard)
}

// registerBoardTools 注册看板（项目）的修改；创建见 registerCreateBoard，有意不注册删除，见 registerAdminTools。
func (a API) registerBoardTools(s *mcp.Server) {
	register(s, spec{
		name: "update_board", kind: kindWrite,
		desc: "修改看板的名称、说明或项目路径，只更新填写的字段；project_path 传空字符串表示清除。看板标识创建后不能改。",
	}, a.updateBoard)
}

// registerColumnTools 注册列的新增、修改、重排和删除。
func (a API) registerColumnTools(s *mcp.Server) {
	register(s, spec{
		name: "create_column", kind: kindWrite,
		desc: "在看板的最右边新增一列。instruction 是写给 agent 的提示词，写明任务在这个状态下谁能做什么、能不能派 subagent、做完去哪一列，agent 必须遵守；" +
			"is_done 为 true 表示完成列；next_column_id 是 agent 用 finish_card 交接后任务去的下一列。",
	}, a.createColumn)
	register(s, spec{
		name: "update_column", kind: kindWrite,
		desc: "修改列，只更新填写的字段。next_column_id 传 0 表示清除；布尔字段传 false 也会生效。",
	}, a.updateColumn)
	register(s, spec{
		name: "reorder_columns", kind: kindWrite,
		desc: "按 ids 的顺序重排看板的列（从左到右）。ids 必须是看板当前的全部列，每列一次，" +
			"否则返回 ErrColumnOrderStale，说明依据的数据已经过期，先用 get_board 重新读一遍。",
	}, a.reorderColumns)
	register(s, spec{
		name: "delete_column", kind: kindDestroy,
		desc: "删除一列。列里有任务时必须用 move_to 指定把它们移入的列（追加到那一列末尾），否则返回 ErrColumnNotEmpty。" +
			"其他列指向它的「下一列」会被清除，用到它的规则会被停用。列已经不存在也返回成功。",
	}, a.deleteColumn)
}

// createBoard 创建看板，返回看板、列和规则。
func (a API) createBoard(ctx context.Context, actor kanban.Actor, in createBoardIn) (any, error) {
	return a.core.CreateBoard(ctx, actor, kanban.CreateBoardInput{
		Name: in.Name, Slug: in.Slug, Description: in.Description, ProjectPath: in.ProjectPath,
		Template: in.Template, Locale: in.Locale,
	})
}

// updateBoard 修改看板的名称、说明或项目路径。
func (a API) updateBoard(ctx context.Context, actor kanban.Actor, in updateBoardIn) (any, error) {
	return a.core.UpdateBoard(ctx, actor, in.BoardID, kanban.UpdateBoardInput{
		Name: in.Name, Description: in.Description, ProjectPath: in.ProjectPath,
	})
}

// createColumn 在看板最右边新增一列。
func (a API) createColumn(ctx context.Context, actor kanban.Actor, in createColumnIn) (any, error) {
	return a.core.CreateColumn(ctx, actor, in.BoardID, kanban.CreateColumnInput{
		Name: in.Name, Instruction: in.Instruction,
		IsDone: in.IsDone, NextColumnID: in.NextColumnID,
	})
}

// updateColumn 修改列，只更新填写的字段。
func (a API) updateColumn(ctx context.Context, actor kanban.Actor, in updateColumnIn) (any, error) {
	return a.core.UpdateColumn(ctx, actor, in.BoardID, in.ColumnID, kanban.UpdateColumnInput{
		Name: in.Name, Instruction: in.Instruction,
		IsDone: in.IsDone, NextColumnID: in.NextColumnID,
	})
}

// reorderColumns 按给定顺序重排列，返回重排后的列。
func (a API) reorderColumns(ctx context.Context, actor kanban.Actor, in reorderColumnsIn) (any, error) {
	cols, err := a.core.ReorderColumns(ctx, actor, in.BoardID, in.IDs)
	if err != nil {
		return nil, err
	}
	return columnsOut{Columns: cols}, nil
}

// deleteColumn 删除一列，列里有任务时先搬到 move_to 指定的列。
func (a API) deleteColumn(ctx context.Context, actor kanban.Actor, in deleteColumnIn) (any, error) {
	return finished(a.core.DeleteColumn(ctx, actor, in.BoardID, in.ColumnID, in.MoveTo))
}

// deleteCard 删除一个任务和它的评论。
func (a API) deleteCard(ctx context.Context, actor kanban.Actor, in cardIn) (any, error) {
	return finished(a.core.DeleteCard(ctx, actor, in.CardID))
}

// deleteComment 删除一条评论。
func (a API) deleteComment(ctx context.Context, actor kanban.Actor, in deleteCommentIn) (any, error) {
	return finished(a.core.DeleteComment(ctx, actor, in.CommentID))
}
