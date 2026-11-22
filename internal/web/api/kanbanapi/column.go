package kanbanapi

import (
	"context"

	"github.com/gin-gonic/gin"

	"slate-kanban/internal/core/kanban"
)

// columnRoutes 注册列的路由。排序用 PUT .../columns/order，不会和 PATCH/DELETE 的 :column_id 冲突，
// 因为 gin 对每种方法各有一棵路由树。
func (a API) columnRoutes(r gin.IRouter) {
	r.POST("/boards/:board_id/columns", route(a.createColumn))
	r.PUT("/boards/:board_id/columns/order", route(a.reorderColumns))
	r.PATCH("/boards/:board_id/columns/:column_id", route(a.updateColumn))
	r.DELETE("/boards/:board_id/columns/:column_id", route(a.deleteColumn))
}

// createColumnInput 是新增列的请求。
type createColumnInput struct {
	boardPath
	kanban.CreateColumnInput
}

// updateColumnInput 是修改列的请求。
type updateColumnInput struct {
	columnPath
	kanban.UpdateColumnInput
}

// reorderColumnsInput 是调整列顺序的请求，IDs 是看板全部列的编号，按期望的从左到右排列。
type reorderColumnsInput struct {
	boardPath
	IDs []int64 `json:"ids"`
}

// deleteColumnInput 是删除列的请求。列里还有任务时，MoveTo 指定任务搬去的列，走查询参数。
type deleteColumnInput struct {
	columnPath
	MoveTo int64 `form:"move_to" json:"-"`
}

// createColumn 新增一列，只有人能做。
func (a API) createColumn(ctx context.Context, actor kanban.Actor, in *createColumnInput) (*kanban.Column, error) {
	return a.core.CreateColumn(ctx, actor, in.BoardID, in.CreateColumnInput)
}

// updateColumn 修改一列，只更新请求里出现的字段，只有人能做。
func (a API) updateColumn(ctx context.Context, actor kanban.Actor, in *updateColumnInput) (*kanban.Column, error) {
	return a.core.UpdateColumn(ctx, actor, in.BoardID, in.ColumnID, in.UpdateColumnInput)
}

// reorderColumns 调整列的顺序，只有人能做。
func (a API) reorderColumns(ctx context.Context, actor kanban.Actor, in *reorderColumnsInput) ([]*kanban.Column, error) {
	return a.core.ReorderColumns(ctx, actor, in.BoardID, in.IDs)
}

// deleteColumn 删除一列，列里的任务搬到 move_to 指定的列，只有人能做。
func (a API) deleteColumn(ctx context.Context, actor kanban.Actor, in *deleteColumnInput) (empty, error) {
	return empty{}, a.core.DeleteColumn(ctx, actor, in.BoardID, in.ColumnID, in.MoveTo)
}
