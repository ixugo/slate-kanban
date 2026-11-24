package desktop

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"

	"slate-kanban/internal/core/kanban"
)

const (
	pendingDevelopment   = "<!-- slate-desktop:development-unconfirmed -->"
	pendingReview        = "<!-- slate-desktop:review-unconfirmed -->"
	completedDevelopment = "<!-- slate-desktop:development-confirmed -->"
	completedReview      = "<!-- slate-desktop:reviewed -->"
	retryCommandPrefix   = "Slate desktop retry: "
	// markerExtKey 是评论扩展字段里存放派发标记的键。
	markerExtKey = "desktop_marker"
)

// markerExt 把派发标记写成评论扩展字段的 JSON 文本。
// 标记只含可见 ASCII 字符，%q 的输出就是合法的 JSON 字符串。
func markerExt(marker string) string {
	return fmt.Sprintf(`{%q:%q}`, markerExtKey, marker)
}

// commentMarker 取出一条评论带的派发标记。新评论存在扩展字段里；
// 旧版本把标记写在正文最后一行，仍然要认，否则升级前遗留的待核对状态会丢。
func commentMarker(comment *kanban.Comment) string {
	if comment.Ext != "" {
		var ext map[string]any
		if json.Unmarshal([]byte(comment.Ext), &ext) == nil {
			if marker, ok := ext[markerExtKey].(string); ok {
				return marker
			}
		}
		return ""
	}
	if _, last, ok := strings.CutLast(comment.Body, "\n"); ok {
		return last
	}
	return comment.Body
}

type dispatchRecovery uint8

const (
	dispatchAvailable dispatchRecovery = iota
	dispatchAwaitingConfirmation
	dispatchNeedsRecoveryNotice
)

// desktopDispatchRecovery 判断一张任务现在能不能由桌面端派发。
// 从最新的评论往回找本阶段的派发标记：
//   - 研发已交接，或本阶段留过暂缓标记：可以派发；
//   - 本阶段只有开始标记：上一轮没确认结果，要先发待核对留言；
//   - 本阶段有待核对留言：人贴出对应恢复码之前一直等人确认；
//   - 审查已留过结论：任务又出现在审查列，说明是人放回来的，直接重审，不要求人再评论。
//     审查结论为待补验时任务会移进收件箱，不会停在审查列等人。
//
// 找不到任何标记也可以派发。
func desktopDispatchRecovery(card *kanban.CardDetail, review bool) dispatchRecovery {
	job := dispatchJob{review: review}
	var confirmations []string
	for i := len(card.Comments) - 1; i >= 0; i-- {
		comment := card.Comments[i]
		if comment.AuthorKind == string(kanban.ActorHuman) {
			if code, ok := strings.CutPrefix(strings.TrimSpace(comment.Body), retryCommandPrefix); ok {
				confirmations = append(confirmations, code)
			}
			continue
		}
		if comment.AuthorKind != string(kanban.ActorAgent) {
			continue
		}
		marker := commentMarker(comment)
		switch marker {
		case completedDevelopment, job.deferredMarker():
			return dispatchAvailable
		case job.startedMarker(), job.legacyPendingMarker():
			// Older versions used one marker for both start and failure.
			// Issue a new recovery notice after the lease ends in either case.
			return dispatchNeedsRecoveryNotice
		case completedReview:
			if review {
				return dispatchAvailable
			}
		}
		if code := job.retryCode(marker); code != "" {
			for _, confirmation := range confirmations {
				if confirmation == code {
					return dispatchAvailable
				}
			}
			return dispatchAwaitingConfirmation
		}
	}
	return dispatchAvailable
}

func (d *dispatchRun) publishDispatchRecovery(ctx context.Context, job dispatchJob, reason string) error {
	code := strings.ToLower(rand.Text())
	body := reason + "\n\nConfirm the prior host has stopped, then inspect the task worktree and results. To authorize another dispatch, post this exact text as a new human comment:\n\n`" +
		retryCommandPrefix + code + "`"
	_, err := d.core.AddCommentWithExt(ctx, job.actor(), job.id, body, markerExt(job.retryPrefix()+code+" -->"))
	return err
}

func (j dispatchJob) stageName() string {
	if j.review {
		return "review"
	}
	return "development"
}

func (j dispatchJob) startedMarker() string {
	return "<!-- slate-desktop:" + j.stageName() + "-started -->"
}

func (j dispatchJob) deferredMarker() string {
	return "<!-- slate-desktop:" + j.stageName() + "-deferred -->"
}

func (j dispatchJob) retryPrefix() string {
	return "<!-- slate-desktop:" + j.stageName() + "-retry:"
}

func (j dispatchJob) retryCode(marker string) string {
	code, ok := strings.CutPrefix(marker, j.retryPrefix())
	if !ok {
		return ""
	}
	code, ok = strings.CutSuffix(code, " -->")
	if !ok || code == "" {
		return ""
	}
	return code
}

func (j dispatchJob) legacyPendingMarker() string {
	if j.review {
		return pendingReview
	}
	return pendingDevelopment
}
