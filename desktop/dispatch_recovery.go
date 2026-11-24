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

func desktopDispatchRecovery(card *kanban.CardDetail, review bool) dispatchRecovery {
	job := dispatchJob{review: review}
	anyHuman := false
	var confirmations []string
	for i := len(card.Comments) - 1; i >= 0; i-- {
		comment := card.Comments[i]
		if comment.AuthorKind == string(kanban.ActorHuman) {
			anyHuman = true
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
				if anyHuman {
					return dispatchAvailable
				}
				return dispatchAwaitingConfirmation
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
