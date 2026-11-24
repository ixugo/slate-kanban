package kanban

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/ixugo/goddd/pkg/orm"
)

const (
	// maxCommentLen 评论正文的最大字符数。
	maxCommentLen = 20000
	// commentSnippetLen 事件里评论摘要的最大字符数，完整正文要通过读取任务获得，避免事件表膨胀。
	commentSnippetLen = 200
	// maxCommentExtLen 评论扩展字段的最大字符数。扩展字段只放几个短标识，不是第二份正文。
	maxCommentExtLen = 2000
)

// CommentStorer 是任务评论的数据持久化接口。评论只能新增和删除，不能修改。
type CommentStorer interface {
	// WithTx 返回绑定到事务的副本，事务内的所有读写都必须用它。
	WithTx(orm.Tx) CommentStorer
	// Create 新增一条评论。
	Create(context.Context, *Comment) error
	// Delete 按主键删除，幂等：评论不存在也返回成功。
	Delete(context.Context, *Comment) error
	// GetByID 按主键读取，不存在时返回可用 orm.IsErrRecordNotFound 判断的错误。
	GetByID(context.Context, int64) (*Comment, error)
	// ListByCard 返回任务的评论，按时间从早到晚。
	ListByCard(context.Context, int64) ([]*Comment, error)
	// ListByBoard 返回看板下全部评论，按任务主键、评论主键升序，导出时一次取回避免 N+1。
	ListByBoard(context.Context, int64) ([]*Comment, error)
	// DeleteByCard 删除某个任务的全部评论，用于删除任务。
	DeleteByCard(context.Context, int64) error
	// DeleteByBoard 删除看板的全部评论，用于删除看板。
	DeleteByBoard(context.Context, int64) error
}

// AddComment 在任务下写一条评论。人和 agent 都可以写，agent 不必持有这个任务，
// 这样 agent 能随时回复人留下的问题；评论只能新增和删除，不能修改。
// 正文去掉首尾空白后必须有 1 到 20000 个字符。
func (c Core) AddComment(ctx context.Context, actor Actor, cardID int64, body string) (*Comment, error) {
	return c.AddCommentWithExt(ctx, actor, cardID, body, "")
}

// AddCommentWithExt 在 AddComment 的基础上带一个扩展字段，只供服务内部的调用方使用，
// HTTP 和 MCP 的写评论入口不接收扩展字段，所以外部写不出带扩展字段的评论。
// 扩展字段必须是 JSON 对象文本，网页不显示，读取评论时随正文一起返回。
// 带扩展字段时正文可以为空：这样的评论只是一条内部标识，不写活动事件，网页也不显示。
func (c Core) AddCommentWithExt(ctx context.Context, actor Actor, cardID int64, body, ext string) (*Comment, error) {
	if err := requireWriter(actor); err != nil {
		return nil, err
	}
	body = strings.TrimSpace(body)
	if err := checkCommentExt(ext); err != nil {
		return nil, err
	}
	if body != "" || ext == "" {
		if err := checkText("评论内容", body, 1, maxCommentLen); err != nil {
			return nil, err
		}
	}
	boardID, err := c.boardOfCard(ctx, cardID)
	if err != nil {
		return nil, err
	}
	var out *Comment
	err = c.run(ctx, boardID, func(u *unit) error {
		card, err := u.loadCard(ctx, boardID, cardID)
		if err != nil {
			return err
		}
		if out, err = u.addComment(ctx, actor, card, body, ext); err != nil {
			return err
		}
		return u.saveRenewedLease(ctx, actor, card)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// checkCommentExt 校验扩展字段：空表示没有，否则必须是不超过上限的 JSON 对象文本。
func checkCommentExt(ext string) error {
	if ext == "" {
		return nil
	}
	if err := checkText("评论扩展字段", ext, 0, maxCommentExtLen); err != nil {
		return err
	}
	var object map[string]any
	if err := json.Unmarshal([]byte(ext), &object); err != nil || object == nil {
		return invalidf("评论扩展字段必须是 JSON 对象")
	}
	return nil
}

// addComment 在事务里保存评论并追加 comment.added 事件，事件里只放评论的摘要。
// 正文为空的评论只是内部标识，不写事件，免得活动里出现一条没有内容的“评论了”。
func (u *unit) addComment(ctx context.Context, actor Actor, card *Card, body, ext string) (*Comment, error) {
	if err := checkCardUnarchived(card); err != nil {
		return nil, err
	}
	now := u.now()
	comment := &Comment{
		BoardID: card.BoardID, CardID: card.ID, AuthorKind: string(actor.Kind), Author: actor.Name,
		Body: body, Ext: ext, CreatedAt: now, UpdatedAt: now,
	}
	if err := u.comment.Create(ctx, comment); err != nil {
		return nil, wrapDB("保存评论", err)
	}
	if body == "" {
		return comment, nil
	}
	if err := u.commitImages(ctx, card.BoardID, body); err != nil {
		return nil, err
	}
	err := u.emit(ctx, actor, newEvent{
		BoardID: card.BoardID, CardID: card.ID, Type: EventCommentAdded,
		Payload: map[string]any{
			"key": card.Key, "comment_id": comment.ID, "author": comment.Author,
			"author_kind": comment.AuthorKind, "snippet": snippet(body, commentSnippetLen),
		},
	})
	if err != nil {
		return nil, err
	}
	return comment, nil
}

// snippet 取文本的前 n 个字符（按字符而不是字节，不会把中文截成乱码）。
func snippet(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

// DeleteComment 删除一条评论，只允许人操作。删除是幂等的：评论已经不存在也返回成功，并且不写事件。
func (c Core) DeleteComment(ctx context.Context, actor Actor, commentID int64) error {
	if err := requireHuman(actor); err != nil {
		return err
	}
	comment, err := c.store.Comment().GetByID(ctx, commentID)
	if orm.IsErrRecordNotFound(err) {
		return nil
	}
	if err != nil {
		return wrapDB("读取评论", err)
	}
	return c.run(ctx, comment.BoardID, func(u *unit) error {
		current, err := u.comment.GetByID(ctx, commentID)
		if orm.IsErrRecordNotFound(err) {
			return nil // 在读取评论和锁看板之间，别人已经删掉了
		}
		if err != nil {
			return wrapDB("读取评论", err)
		}
		card, err := u.loadCard(ctx, current.BoardID, current.CardID)
		if err != nil {
			return err
		}
		if err := checkCardUnarchived(card); err != nil {
			return err
		}
		if err := u.comment.Delete(ctx, current); err != nil {
			return wrapDB("删除评论", err)
		}
		if err := u.releaseImages(ctx, current.BoardID, attachmentTokens(current.Body)); err != nil {
			return err
		}
		return u.emit(ctx, actor, newEvent{
			BoardID: current.BoardID, CardID: card.ID, Type: EventCommentDeleted,
			Payload: map[string]any{"key": card.Key, "comment_id": current.ID},
		})
	})
}

// saveRenewedLease 持有者在自己的任务下写评论时顺延租约并保存。续租不改变任务内容，所以不涨版本号也不写事件。
func (u *unit) saveRenewedLease(ctx context.Context, actor Actor, card *Card) error {
	if !u.renewLease(actor, card) {
		return nil
	}
	if err := u.card.Save(ctx, card); err != nil {
		return wrapDB("顺延租约", err)
	}
	return nil
}
