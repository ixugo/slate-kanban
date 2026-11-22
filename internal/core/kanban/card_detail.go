package kanban

import (
	"context"
	"time"

	"github.com/ixugo/goddd/pkg/orm"
)

// CardRef 是依赖列表里一个任务的概要，足够让人和 agent 看出它是什么、做完了没有。
type CardRef struct {
	ID     int64  `json:"id"`
	Key    string `json:"key"`
	Title  string `json:"title"`
	Column string `json:"column"`
	Done   bool   `json:"done"`
}

// CardDetail 是一个任务的完整信息：任务本身和派生状态，所在列给 agent 的工作指令，
// 依赖的任务，以及全部评论。agent 领到活之后靠它知道该做什么、该怎么做、前面聊过什么。
type CardDetail struct {
	*CardView
	ColumnInstruction string     `json:"column_instruction"`
	Dependencies      []*CardRef `json:"dependencies"`
	Comments          []*Comment `json:"comments"`
}

// GetCardDetail 读取一个任务的完整信息，人和 agent 都可以读。任务不存在返回 404。
// 只读取这个任务自己的依赖，不会把整个看板的任务都读出来。
func (c Core) GetCardDetail(ctx context.Context, cardID int64) (*CardDetail, error) {
	card, err := c.store.Card().GetByID(ctx, cardID)
	if err != nil {
		return nil, mapNotFound(err, ErrCardNotFound, "读取任务")
	}
	cols, err := c.store.Column().ListByBoard(ctx, card.BoardID)
	if err != nil {
		return nil, wrapDB("读取列", err)
	}
	tags, err := c.store.Tag().ListByBoard(ctx, card.BoardID)
	if err != nil {
		return nil, wrapDB("读取标签库", err)
	}
	deps, err := loadDependencies(ctx, c.store.Card(), card)
	if err != nil {
		return nil, err
	}
	comments, err := c.store.Comment().ListByCard(ctx, cardID)
	if err != nil {
		return nil, wrapDB("读取评论", err)
	}
	ix := newBoardIndex(c.now(), time.Local, cols, tags, append([]*Card{card}, deps...))
	detail := &CardDetail{
		CardView: ix.view(card), Dependencies: make([]*CardRef, 0, len(deps)),
		Comments: comments,
	}
	if detail.Comments == nil {
		detail.Comments = []*Comment{}
	}
	if col, ok := ix.columns[card.ColumnID]; ok {
		detail.ColumnInstruction = col.Instruction
	}
	for _, dep := range deps {
		v := ix.view(dep)
		detail.Dependencies = append(detail.Dependencies, &CardRef{ID: dep.ID, Key: dep.Key, Title: dep.Title, Column: v.Column, Done: v.Done})
	}
	return detail, nil
}

// loadDependencies 按依赖列表的顺序读出任务依赖的任务，已经不存在的跳过。
// 依赖最多 maxWaitsOn 个，所以逐个按主键读取的开销有限。cards 可以是事务内的存储，也可以是普通存储。
func loadDependencies(ctx context.Context, cards CardStorer, card *Card) ([]*Card, error) {
	deps := make([]*Card, 0, len(card.WaitsOn))
	for _, id := range card.WaitsOn {
		dep, err := cards.GetByID(ctx, id)
		if orm.IsErrRecordNotFound(err) {
			continue
		}
		if err != nil {
			return nil, wrapDB("读取依赖的任务", err)
		}
		deps = append(deps, dep)
	}
	return deps, nil
}
