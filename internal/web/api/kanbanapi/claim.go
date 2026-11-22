package kanbanapi

import (
	"context"

	"github.com/gin-gonic/gin"

	"slate-kanban/internal/core/kanban"
)

// claimRoutes 注册领取相关的路由：领取、释放、完成交接、取下一张。
func (a API) claimRoutes(r gin.IRouter) {
	r.POST("/boards/:board_id/cards/next", route(a.nextCard))
	r.POST("/cards/:card_id/claim", route(a.claimCard))
	r.POST("/cards/:card_id/release", route(a.releaseCard))
	r.POST("/cards/:card_id/finish", route(a.finishCard))
}

// claimCardInput 是领取任务的请求，请求体可以省略，省略时用默认租约时长。
type claimCardInput struct {
	cardPath
	kanban.ClaimCardInput
}

// finishCardInput 是完成交接的请求。
type finishCardInput struct {
	cardPath
	kanban.FinishCardInput
}

// nextCardInput 是取下一个任务的请求，请求体可以省略。
type nextCardInput struct {
	boardPath
	kanban.NextCardInput
}

// nextCardOutput 是取下一个任务的响应。没有可做的任务不是错误，此时 card 为 null，
// 用对象包一层，客户端不用处理"整个响应体就是 null"这种容易出错的形态。
type nextCardOutput struct {
	Card *kanban.CardDetail `json:"card"`
}

// nextCard 为 agent 挑选并领取看板里下一个该做的任务，只有 agent 能做。
func (a API) nextCard(ctx context.Context, actor kanban.Actor, in *nextCardInput) (*nextCardOutput, error) {
	card, err := a.core.NextCard(ctx, actor, in.BoardID, in.NextCardInput)
	if err != nil {
		return nil, err
	}
	return &nextCardOutput{Card: card}, nil
}

// claimCard 让 agent 领取指定的任务，已经由自己持有时只是续租，只有 agent 能做。
func (a API) claimCard(ctx context.Context, actor kanban.Actor, in *claimCardInput) (*kanban.CardDetail, error) {
	return a.core.ClaimCard(ctx, actor, in.CardID, in.ClaimCardInput)
}

// releaseCard 释放任务上的领取。agent 只能释放自己的，人可以收回任何一张；已经没有领取也返回成功。
func (a API) releaseCard(ctx context.Context, actor kanban.Actor, in *cardPath) (empty, error) {
	return empty{}, a.core.ReleaseCard(ctx, actor, in.CardID)
}

// finishCard 完成交接：写总结、记录分支和链接、移到下一列并清除领取。
func (a API) finishCard(ctx context.Context, actor kanban.Actor, in *finishCardInput) (*kanban.CardDetail, error) {
	return a.core.FinishCard(ctx, actor, in.CardID, in.FinishCardInput)
}
