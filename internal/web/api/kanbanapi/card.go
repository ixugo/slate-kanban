package kanbanapi

import (
	"context"

	"github.com/gin-gonic/gin"

	"slate-kanban/internal/core/kanban"
)

// cardRoutes 注册卡片读写的路由。
func (a API) cardRoutes(r gin.IRouter) {
	r.GET("/boards/:board_id/cards", route(a.listCards))
	r.POST("/boards/:board_id/cards", route(a.createCard))
	r.GET("/cards/:card_id", route(a.getCard))
	r.PATCH("/cards/:card_id", route(a.updateCard))
	r.DELETE("/cards/:card_id", route(a.deleteCard))
	r.GET("/cards/:card_id/changes/:change_id", route(a.getCardChange))
	r.POST("/cards/:card_id/move", route(a.moveCard))
	r.POST("/cards/:card_id/read", route(a.markCardRead))
	r.POST("/cards/:card_id/comments", route(a.addComment))
	r.DELETE("/comments/:comment_id", route(a.deleteComment))
}

// listCardsInput 是列出卡片的请求。Core 的入参只有 json 标签，查询参数要单独声明 form 标签。
type listCardsInput struct {
	boardPath
	Query string `form:"query" json:"-"`
	Limit int    `form:"limit" json:"-"`
}

// createCardInput 是创建卡片的请求。
type createCardInput struct {
	boardPath
	kanban.CreateCardInput
}

// updateCardInput 是修改卡片的请求。
type updateCardInput struct {
	cardPath
	kanban.UpdateCardInput
}

// moveCardInput 是移动卡片的请求。
type moveCardInput struct {
	cardPath
	kanban.MoveCardInput
}

// addCommentInput 是写评论的请求。
type addCommentInput struct {
	cardPath
	Body string `json:"body"`
}

// listCards 按筛选条件列出看板里的卡片，筛选语法见看板设计文档。
func (a API) listCards(ctx context.Context, _ kanban.Actor, in *listCardsInput) (*kanban.CardList, error) {
	return a.core.ListCards(ctx, in.BoardID, kanban.ListCardsInput{Query: in.Query, Limit: in.Limit})
}

// createCard 创建卡片。人和 agent 都能做，agent 不能填规划类字段，由 Core 判断。
func (a API) createCard(ctx context.Context, actor kanban.Actor, in *createCardInput) (*kanban.Card, error) {
	return a.core.CreateCard(ctx, actor, in.BoardID, in.CreateCardInput)
}

// getCard 返回卡片详情：派生状态、列的工作指令、依赖和全部评论。
func (a API) getCard(ctx context.Context, _ kanban.Actor, in *cardPath) (*kanban.CardDetail, error) {
	return a.core.GetCardDetail(ctx, in.CardID)
}

// updateCard 修改卡片，只更新请求里出现的字段。agent 只能改自己持有的卡片，由 Core 判断。
func (a API) updateCard(ctx context.Context, actor kanban.Actor, in *updateCardInput) (*kanban.Card, error) {
	return a.core.UpdateCard(ctx, actor, in.CardID, in.UpdateCardInput)
}

// deleteCard 删除卡片，只有人能做；卡片已经不存在也返回成功。
func (a API) deleteCard(ctx context.Context, actor kanban.Actor, in *cardPath) (empty, error) {
	return empty{}, a.core.DeleteCard(ctx, actor, in.CardID)
}

// getCardChange 返回卡片的一次变更记录：修改前和修改后的内容，网页的「查看变更」据此左右对比两个版本。
func (a API) getCardChange(ctx context.Context, _ kanban.Actor, in *cardChangePath) (*kanban.CardChange, error) {
	return a.core.GetCardChange(ctx, in.CardID, in.ChangeID)
}

// moveCard 移动卡片到某列或调整同一列里的顺序。agent 只能移自己持有的卡片，由 Core 判断。
func (a API) moveCard(ctx context.Context, actor kanban.Actor, in *moveCardInput) (*kanban.Card, error) {
	return a.core.MoveCard(ctx, actor, in.CardID, in.MoveCardInput)
}

// markCardRead 把任务的未读数清零，只给人用；不写事件，不碰版本号。
func (a API) markCardRead(ctx context.Context, _ kanban.Actor, in *cardPath) (empty, error) {
	return empty{}, a.core.MarkCardRead(ctx, in.CardID)
}

// addComment 在卡片下写一条评论，人和 agent 都能写。
func (a API) addComment(ctx context.Context, actor kanban.Actor, in *addCommentInput) (*kanban.Comment, error) {
	return a.core.AddComment(ctx, actor, in.CardID, in.Body)
}

// deleteComment 删除一条评论，只有人能做；评论已经不存在也返回成功。
func (a API) deleteComment(ctx context.Context, actor kanban.Actor, in *commentPath) (empty, error) {
	return empty{}, a.core.DeleteComment(ctx, actor, in.CommentID)
}
