package kanban

import (
	"context"
	"regexp"
	"strings"
)

// FinishCard 完成交接：写一条总结评论，记录分支和链接，把任务移到目标列的指定位置（默认末尾），清除领取。
// 这些都在同一个事务里，任何一步失败整体回滚，不会出现总结写了但任务还留在进行中的半成品。
// 人可以完成任何任务；agent 只能完成自己持有的任务。目标列不指定时用当前列设置的下一列，
// agent 显式指定的目标列不能是开启了 agent 领取的列，否则它能把任务重新放回队列，绕过人的评审。
func (c Core) FinishCard(ctx context.Context, actor Actor, cardID int64, in FinishCardInput) (*CardDetail, error) {
	return c.FinishCardWithExt(ctx, actor, cardID, in, "")
}

// FinishCardWithExt 在 FinishCard 的基础上给总结评论带扩展字段，规则见 AddCommentWithExt。
// 只供服务内部的调用方使用，HTTP 和 MCP 的完成交接入口不接收扩展字段。
func (c Core) FinishCardWithExt(ctx context.Context, actor Actor, cardID int64, in FinishCardInput, ext string) (*CardDetail, error) {
	if err := requireWriter(actor); err != nil {
		return nil, err
	}
	if err := checkCommentExt(ext); err != nil {
		return nil, err
	}
	summary, err := finishSummary(in)
	if err != nil {
		return nil, err
	}
	if err := checkText("总结", summary, 1, maxCommentLen); err != nil {
		return nil, err
	}
	boardID, err := c.boardOfCard(ctx, cardID)
	if err != nil {
		return nil, err
	}
	err = c.run(ctx, boardID, func(u *unit) error {
		card, err := u.loadCard(ctx, boardID, cardID)
		if err != nil {
			return err
		}
		return u.finishCard(ctx, actor, card, summary, ext, in)
	})
	if err != nil {
		return nil, err
	}
	return c.GetCardDetail(ctx, cardID)
}

// finishCard 在事务里依次完成：检查权限、确定目标列、写总结评论、写分支和链接、换列并清除领取。
func (u *unit) finishCard(ctx context.Context, actor Actor, card *Card, summary, ext string, in FinishCardInput) error {
	if err := checkCardWritable(actor, card); err != nil {
		return err
	}
	from, to, err := u.finishTarget(ctx, card, in.ColumnID)
	if err != nil {
		return err
	}
	if in.Version != nil && card.Version != *in.Version {
		return ErrCardVersionConflict
	}
	if len(in.MergedCommits) > 0 {
		if err := u.checkReviewedMerge(ctx, actor, card, from, to, in); err != nil {
			return err
		}
		card.MergedCommits = in.MergedCommits
	}
	if _, err := u.addComment(ctx, actor, card, summary, ext); err != nil {
		return err
	}
	if err := u.recordLinks(ctx, actor, card, in); err != nil {
		return err
	}
	return u.handOver(ctx, actor, card, from, to, in)
}

var shortCommit = regexp.MustCompile(`^[0-9a-f]{6}$`)

// finishSummary 把合并提交一并写入审查评论，和卡片的状态使用同一份数据。
func finishSummary(in FinishCardInput) (string, error) {
	summary := strings.TrimSpace(in.Summary)
	if err := checkText("总结", summary, 1, maxCommentLen); err != nil {
		return "", err
	}
	if len(in.MergedCommits) > maxCommentLen/10 {
		return "", invalidf("合并提交过多")
	}
	if len(in.MergedCommits) == 0 {
		return summary, nil
	}
	for _, hash := range in.MergedCommits {
		if !shortCommit.MatchString(hash) {
			return "", invalidf("合并提交需填写哈希前六位小写十六进制")
		}
	}
	summary += "\n\n合并提交：`" + strings.Join(in.MergedCommits, "`, `") + "`"
	return summary, checkText("总结（含合并提交）", summary, 1, maxCommentLen)
}

func (u *unit) checkReviewedMerge(ctx context.Context, actor Actor, card *Card, from, to *Column, in FinishCardInput) error {
	if err := requireAgent(actor); err != nil {
		return err
	}
	if !claimActive(card, u.now()) {
		return ErrNotHolder
	}
	if in.Version == nil {
		return invalidf("记录合并需要提供审查时读到的 version")
	}
	if from.IsDone || !to.IsDone || from.NextColumnID != to.ID {
		return invalidf("记录合并时目标必须是审查列的下一列且标记为完成")
	}
	if in.Branch != "" || !strings.HasPrefix(card.Branch, "slate/"+strings.ToLower(card.Key)+"-") {
		return invalidf("只能记录卡片已有的专属 Slate 分支")
	}
	engineers, err := u.engineer.ListByBoard(ctx, card.BoardID)
	if err != nil {
		return wrapDB("读取工程师名册", err)
	}
	for _, engineer := range engineers {
		if engineer.ID == card.ClaimEngineerID && engineer.Kind == EngineerReview {
			return nil
		}
	}
	return invalidf("只有使用测试工程师名额的持卡 agent 可以记录合并")
}

// finishTarget 确定交接后任务去哪一列，同时返回任务当前所在的列。
// 指定了目标列就用它；没指定就用当前列设置的下一列，没有设置返回 400 并提示怎么办。
func (u *unit) finishTarget(ctx context.Context, card *Card, columnID int64) (from, to *Column, err error) {
	cols, err := u.column.ListByBoard(ctx, card.BoardID)
	if err != nil {
		return nil, nil, wrapDB("读取列", err)
	}
	if from = columnByID(cols, card.ColumnID); from == nil {
		return nil, nil, ErrColumnNotFound
	}
	if columnID == 0 {
		if to = columnByID(cols, from.NextColumnID); to == nil {
			return nil, nil, invalidf("「%s」列没有设置下一列，请指定 column_id，或者让人在列设置里指定下一列", from.Name)
		}
		return from, to, nil
	}
	if to = columnByID(cols, columnID); to == nil {
		return nil, nil, invalidf("目标列不存在，或者不属于这个看板")
	}
	return from, to, nil
}

// recordLinks 把交接时给出的分支和链接写到任务上，并按字段追加 card.updated 事件。
// 这里只改内存里的任务，随后的 handOver 会统一保存并涨一次版本号。
func (u *unit) recordLinks(ctx context.Context, actor Actor, card *Card, in FinishCardInput) error {
	var patch UpdateCardInput
	if in.Branch != "" {
		patch.Branch = &in.Branch
	}
	if in.URL != "" {
		patch.URL = &in.URL
	}
	before := *card
	changed, err := patch.apply(card)
	if err != nil || len(changed) == 0 {
		return err
	}
	return u.emitCardUpdates(ctx, actor, card, changed, 0, &before)
}

// handOver 把任务放到目标列的指定位置（默认列尾，in.Top 为真时列头）并清除领取。换了列时 card.moved 已经表达了"领取结束"，
// 所以不再单独写 card.released；留在原列时没有别的事件，才补一条 card.released。
func (u *unit) handOver(ctx context.Context, actor Actor, card *Card, from, to *Column, in FinishCardInput) error {
	if from.ID == to.ID {
		if err := u.clearClaim(ctx, actor, card, releaseReleased); err != nil {
			return err
		}
		return u.saveCard(ctx, actor, card)
	}
	others, err := u.cardsExcept(ctx, to.ID, card.ID)
	if err != nil {
		return err
	}
	pos, _, err := u.placeCard(ctx, others, topRef(others, in.Top, in.BeforeID, in.AfterID), in.AfterID)
	if err != nil {
		return err
	}
	card.ClaimedBy, card.ClaimExpiresAt, card.ClaimEngineerID = "", nil, 0
	card.ColumnID, card.Position = to.ID, pos
	return u.commitMove(ctx, actor, card, from, to)
}
