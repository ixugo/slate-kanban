package desktop

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"slate-kanban/internal/core/kanban"
)

const (
	// reviewApproved 标记一条审查通过的评论，评论末尾带 "Slate review: 起点..已审提交" 审批行。
	// 合并失败后任务仍停在审查列，下次派发看到它就直接重试合并，不再重新审查。
	reviewApproved = "<!-- slate-desktop:review-approved -->"
	// mergeBlocked 标记桌面端写的"合并被挡住"说明。它只是提示，不改变审批是否有效。
	mergeBlocked = "<!-- slate-desktop:merge-blocked -->"
	// mergeConflictPrefix 标记"审批通过但合并冲突"的评论，后面跟当时项目分支的提交，以 " -->" 结尾。
	// 研发把分支变基到该提交之后，才算解决了冲突。
	mergeConflictPrefix = "<!-- slate-desktop:merge-conflict:"
	approvalLinePrefix  = "Slate review: "
	// integrateTimeout 是一次合并脚本的上限。合并一旦开始不跟随派发取消，避免留下半截状态，所以要有自己的超时。
	integrateTimeout = 5 * time.Minute
	// maxMergeReasonRunes 限制写进评论的失败原因长度，避免脚本或 Git 的长输出刷屏。
	maxMergeReasonRunes = 600
)

// approvalLineRE 匹配审批行，取出起点和已审提交各自的前六位。
var approvalLineRE = regexp.MustCompile(`(?m)^` + approvalLinePrefix + `([0-9a-f]{6})\.\.([0-9a-f]{6})$`)

// approvalLine 生成审批行。合并脚本凭这一行确认评论审的就是这段提交。
func approvalLine(base, head string) string {
	return approvalLinePrefix + base[:6] + ".." + head[:6]
}

// reviewedMergeBase 判断审查通过的分支是否需要由桌面端合并，需要时返回审查范围的起点。
// 只有专属分支（slate/<编号>-xxx）才合并；起点取项目当前分支与任务分支的共同祖先。
// 分支没有领先的提交（例如只回复评论的任务，或已经合入）时不需要合并，返回 false。
func reviewedMergeBase(ctx context.Context, project, key, branch, head string) (string, bool, error) {
	pattern := regexp.MustCompile(`^slate/` + regexp.QuoteMeta(strings.ToLower(key)) + `-[a-z0-9][a-z0-9-]*$`)
	if !pattern.MatchString(branch) {
		return "", false, nil
	}
	base, err := gitOutput(ctx, project, "merge-base", "HEAD", head)
	if err != nil {
		return "", false, err
	}
	ahead, err := gitOutput(ctx, project, "rev-list", "--count", base+".."+head)
	if err != nil {
		return "", false, err
	}
	return base, ahead != "0", nil
}

// pendingApproval 找出还没合并的审查通过评论。从最新的评论往回找：
// 人的评论和桌面端写的"合并被挡住"说明不影响审批；遇到别的桌面端标记
// （研发交接、重新审查、待核对留言等）说明审批已经被后续流程取代，返回 false。
func pendingApproval(card *kanban.CardDetail) (id int64, base, head string, ok bool) {
	for i := len(card.Comments) - 1; i >= 0; i-- {
		comment := card.Comments[i]
		if comment.AuthorKind != string(kanban.ActorAgent) {
			continue
		}
		switch commentMarker(comment) {
		case mergeBlocked:
			continue
		case reviewApproved:
			match := approvalLineRE.FindStringSubmatch(comment.Body)
			if match == nil {
				return 0, "", "", false
			}
			return comment.ID, match[1], match[2], true
		default:
			return 0, "", "", false
		}
	}
	return 0, "", "", false
}

// rebaseTarget 返回研发必须变基到的提交：任务被合并冲突退回、且之后还没有新的研发交接时有值。
// 研发改写了分支历史，原先"旧提交仍是祖先"的检查不再成立，改为检查这个提交是新分支的祖先。
func rebaseTarget(card *kanban.CardDetail) string {
	for i := len(card.Comments) - 1; i >= 0; i-- {
		comment := card.Comments[i]
		if comment.AuthorKind != string(kanban.ActorAgent) {
			continue
		}
		marker := commentMarker(comment)
		if marker == completedDevelopment {
			return ""
		}
		if tip, ok := strings.CutPrefix(marker, mergeConflictPrefix); ok {
			if tip, ok = strings.CutSuffix(tip, " -->"); ok {
				return tip
			}
		}
	}
	return ""
}

// completeReviewPassed 处理审查通过的结论：需要合并的分支先留下带审批行的评论再合并，
// 不需要合并的任务沿用原来的做法，留证据后直接移入完成列。
func (d *dispatchRun) completeReviewPassed(ctx context.Context, job dispatchJob, card *kanban.CardDetail, summary, head string, text desktopText) error {
	base, merge, err := reviewedMergeBase(ctx, job.project, card.Key, card.Branch, head)
	if err != nil {
		return d.recordDispatchFailure(ctx, job, err, text)
	}
	if !merge {
		if _, err := d.core.AddCommentWithExt(ctx, job.actor(), job.id, summary, markerExt(completedReview)); err != nil {
			return err
		}
		_, err := d.core.MoveCard(ctx, job.actor(), job.id, kanban.MoveCardInput{ColumnID: job.flow.done})
		return err
	}
	comment, err := d.core.AddCommentWithExt(ctx, job.actor(), job.id, summary+"\n\n"+approvalLine(base, head), markerExt(reviewApproved))
	if err != nil {
		return err
	}
	return d.integrateApproved(ctx, job, comment.ID, base, head, text)
}

// resumeApproved 在任务带着未合并的审批回到审查列时，不重新审查，直接重试合并。
// 返回 false 表示审批已失效（分支又有了新提交），调用方应当走一次完整的重新审查。
func (d *dispatchRun) resumeApproved(ctx context.Context, job dispatchJob, card *kanban.CardDetail, text desktopText) (bool, error) {
	id, base6, head6, ok := pendingApproval(card)
	if !ok || card.Branch == "" {
		return false, nil
	}
	head, err := gitOutput(ctx, job.project, "rev-parse", "--verify", "refs/heads/"+card.Branch+"^{commit}")
	if err != nil || !strings.HasPrefix(head, head6) {
		return false, nil
	}
	base, merge, err := reviewedMergeBase(ctx, job.project, card.Key, card.Branch, head)
	if err != nil || !merge || !strings.HasPrefix(base, base6) {
		return false, nil
	}
	return true, d.integrateApproved(ctx, job, id, base, head, text)
}

// integrateApproved 用内置脚本把审查过的提交合入项目当前分支，再带着合并提交把任务移入完成列。
// 合并不动用模型：脚本先在临时工作树里逐个 cherry-pick，成功后才快进项目分支；
// 主工作区有未提交修改等问题会原样保留现场，只在卡片上说明原因。
// 提交有冲突时，把任务退回「进行中」，让研发变基并解决冲突，之后重新审查再合并。
func (d *dispatchRun) integrateApproved(ctx context.Context, job dispatchJob, approval int64, base, head string, text desktopText) error {
	card, err := d.checkJob(ctx, job)
	if err != nil {
		return err
	}
	// 合并发布期间不能被取消打断，否则可能留下半截状态；脚本自带的回执保证重启后能续办。
	run, stop := context.WithTimeout(context.WithoutCancel(ctx), integrateTimeout)
	defer stop()
	output, err := runIntegrate(run, job.project, card, approval, "apply", "--base", base, "--reviewed-head", head)
	if err != nil {
		return d.recordMergeFailure(ctx, job, err, text)
	}
	var receipt struct {
		MergedCommits []string `json:"merged_commits"`
	}
	if err := json.Unmarshal(output, &receipt); err != nil || len(receipt.MergedCommits) == 0 {
		return d.recordMergeFailure(ctx, job, errors.New("integration returned no merged commits"), text)
	}
	merged, err := d.core.FinishCardWithExt(ctx, job.actor(), job.id, kanban.FinishCardInput{
		Summary: text.dispatchMerged, MergedCommits: receipt.MergedCommits, Version: &card.Version,
	}, markerExt(completedReview))
	if err != nil {
		return err
	}
	d.cleanupMerged(ctx, job, merged)
	return nil
}

// cleanupMerged 在合并记入看板后清理任务工作树和分支。清理失败不影响已完成的合并，
// 卡片详情里的清理入口会显示阻断原因，并支持重试，所以这里只记日志。
func (d *dispatchRun) cleanupMerged(ctx context.Context, job dispatchJob, card *kanban.CardDetail) {
	run, stop := context.WithTimeout(context.WithoutCancel(ctx), integrateTimeout)
	defer stop()
	if _, err := runIntegrate(run, job.project, card, 0, "cleanup-retry"); err != nil {
		slog.Warn("desktop dispatch could not clean the merged task worktree", "card", card.Key, "error", err)
	}
}

// recordMergeFailure 处理合并失败：冲突时退回研发，其余情况在卡片上留下原因并保留审查状态。
// 返回的错误会让调度器暂停这张任务，直到任务有新的变化。
func (d *dispatchRun) recordMergeFailure(ctx context.Context, job dispatchJob, cause error, text desktopText) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if _, err := d.checkJob(ctx, job); err != nil {
		return errors.Join(cause, err)
	}
	if isMergeConflict(cause) {
		// 冲突是正常流转而不是故障：退回研发后由研发自动接手，不暂停这张任务。
		return d.returnConflictToDevelopment(ctx, job, text)
	}
	reason := cause.Error()
	if runes := []rune(reason); len(runes) > maxMergeReasonRunes {
		reason = string(runes[:maxMergeReasonRunes])
	}
	_, err := d.core.AddCommentWithExt(ctx, job.actor(), job.id, fmt.Sprintf(text.dispatchMergeBlocked, reason), markerExt(mergeBlocked))
	return errors.Join(cause, err)
}

// isMergeConflict 判断脚本失败是否因为 cherry-pick 冲突。脚本把 Git 的原话放在错误里。
func isMergeConflict(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "cherry-pick") && strings.Contains(message, "conflict")
}

// returnConflictToDevelopment 记下项目分支当前的提交并把任务退回「进行中」，
// 研发据此把分支变基过去解决冲突，交接后会重新审查。
func (d *dispatchRun) returnConflictToDevelopment(ctx context.Context, job dispatchJob, text desktopText) error {
	tip, err := gitOutput(ctx, job.project, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	name, err := gitOutput(ctx, job.project, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return err
	}
	body := fmt.Sprintf(text.dispatchMergeConflict, name, tip[:12])
	if _, err := d.core.AddCommentWithExt(ctx, job.actor(), job.id, body, markerExt(mergeConflictPrefix+tip+" -->")); err != nil {
		return err
	}
	_, err = d.core.MoveCard(ctx, job.actor(), job.id, kanban.MoveCardInput{ColumnID: job.flow.dev, Top: true})
	return err
}
