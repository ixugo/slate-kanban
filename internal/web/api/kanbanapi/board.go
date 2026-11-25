package kanbanapi

import (
	"context"

	"github.com/gin-gonic/gin"

	"slate-kanban/internal/core/kanban"
)

// boardRoutes 注册看板自身的路由。
func (a API) boardRoutes(r gin.IRouter) {
	r.GET("/boards", route(a.listBoards))
	r.POST("/boards", route(a.createBoard))
	r.POST("/boards/check-project-path", route(a.checkProjectPath))
	r.GET("/boards/:board_id", route(a.getBoard))
	r.PATCH("/boards/:board_id", route(a.updateBoard))
	r.DELETE("/boards/:board_id", route(a.deleteBoard))
}

type checkProjectPathInput struct {
	ProjectPath string `json:"project_path"`
}

// updateBoardInput 是修改看板的请求：路径里的编号加请求体里要改的字段。
type updateBoardInput struct {
	boardPath
	kanban.UpdateBoardInput
}

// listBoards 返回全部看板。
func (a API) listBoards(ctx context.Context, _ kanban.Actor, _ *struct{}) ([]*kanban.Board, error) {
	return a.core.ListBoards(ctx)
}

// checkProjectPath 检查工作目录是否存在，不写入看板。
func (a API) checkProjectPath(_ context.Context, _ kanban.Actor, in *checkProjectPathInput) (empty, error) {
	return empty{}, kanban.ValidateLocalProjectPath(in.ProjectPath)
}

// createBoard 创建看板。REST 填写的工作目录必须存在于服务端。
func (a API) createBoard(ctx context.Context, actor kanban.Actor, in *kanban.CreateBoardInput) (*kanban.BoardDetail, error) {
	if err := kanban.ValidateLocalProjectPath(in.ProjectPath); err != nil {
		return nil, err
	}
	return a.core.CreateBoard(ctx, actor, *in)
}

// getBoard 返回看板连同它的列和规则。
func (a API) getBoard(ctx context.Context, _ kanban.Actor, in *boardPath) (*kanban.BoardDetail, error) {
	return a.core.GetBoard(ctx, in.BoardID)
}

// updateBoard 修改看板。只在提交了工作目录时检查，未提交则保留原路径。
func (a API) updateBoard(ctx context.Context, actor kanban.Actor, in *updateBoardInput) (*kanban.Board, error) {
	if in.ProjectPath != nil {
		if err := kanban.ValidateLocalProjectPath(*in.ProjectPath); err != nil {
			return nil, err
		}
	}
	return a.core.UpdateBoard(ctx, actor, in.BoardID, in.UpdateBoardInput)
}

// deleteBoard 删除看板和它的全部数据，只有人能做；看板已经不存在也返回成功。
func (a API) deleteBoard(ctx context.Context, actor kanban.Actor, in *boardPath) (empty, error) {
	return empty{}, a.core.DeleteBoard(ctx, actor, in.BoardID)
}
