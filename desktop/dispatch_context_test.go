package desktop

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"slate-kanban/internal/core/kanban"
)

func createContextCard(t *testing.T, core kanban.Core, actor kanban.Actor, boardID int64, in kanban.CreateCardInput) *kanban.Card {
	t.Helper()
	card, err := core.CreateCard(t.Context(), actor, boardID, in)
	if err != nil {
		t.Fatal(err)
	}
	return card
}

func contextPromptData(t *testing.T, prompt, prefix string) string {
	t.Helper()
	_, rest, ok := strings.Cut(prompt, prefix)
	if !ok {
		t.Fatalf("prompt missing %q", prefix)
	}
	line, _, _ := strings.Cut(rest, "\n")
	return line
}

// 从归档卡 Fork 出来的任务应先找回历史。没有引用但关键词命中的卡也参与筛选，
// 同板无关卡和其他看板的卡不能混入上下文，筛选后的上下文在规划和正式执行时保持一致。
func TestDesktopDispatchRetrievesRelatedContextBeforeMoving(t *testing.T) {
	ctx := t.Context()
	core, human, board, flow := newDispatchTestBoard(t)
	previous := createContextCard(t, core, human, board.ID, kanban.CreateCardInput{
		Title: "第一版消息回执", Description: "只处理在线连接", ColumnID: flow.done,
	})
	const completed = "在线 ACK 已实现；尚未处理离线重播。"
	comment, err := core.AddComment(ctx, kanban.Actor{Kind: kanban.ActorAgent, Name: "Previous engineer"}, previous.ID, completed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := core.ArchiveCard(ctx, human, previous.ID); err != nil {
		t.Fatal(err)
	}
	dependency := createContextCard(t, core, human, board.ID, kanban.CreateCardInput{Title: "准备协议", ColumnID: flow.done})
	current, err := core.ForkCard(ctx, human, previous.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := core.UpdateCard(ctx, human, current.ID, kanban.UpdateCardInput{
		Title: new("离线重连补齐"), WaitsOn: new([]int64{dependency.ID}),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := core.MoveCard(ctx, human, current.ID, kanban.MoveCardInput{ColumnID: flow.todo}); err != nil {
		t.Fatal(err)
	}
	const requirement = "补充说明：检索 RetryLedger 和重连日志。"
	if _, err := core.AddComment(ctx, human, current.ID, requirement); err != nil {
		t.Fatal(err)
	}
	reverseRef := createContextCard(t, core, human, board.ID, kanban.CreateCardInput{
		Title: "反向引用", Description: "另一个后续需求见 " + strings.ToLower(current.Key), ColumnID: flow.inbox,
	})
	reverseDep := createContextCard(t, core, human, board.ID, kanban.CreateCardInput{
		Title: "后续依赖", WaitsOn: []int64{current.ID}, ColumnID: flow.inbox,
	})
	commentOnly := createContextCard(t, core, human, board.ID, kanban.CreateCardInput{Title: "另一份记录", ColumnID: flow.done})
	const supplement = "RetryLedger 可能有遗漏，需要复核。"
	extra, err := core.AddComment(ctx, human, commentOnly.ID, supplement)
	if err != nil {
		t.Fatal(err)
	}
	tagOnly := createContextCard(t, core, human, board.ID, kanban.CreateCardInput{
		Title: "不相关的标签命中", Tags: []string{"RetryLedger"}, ColumnID: flow.inbox,
	})
	titleOnly := createContextCard(t, core, human, board.ID, kanban.CreateCardInput{Title: "retryledger 辅助工具", ColumnID: flow.inbox})
	descOnly := createContextCard(t, core, human, board.ID, kanban.CreateCardInput{Title: "环境问题", Description: "重连日志", ColumnID: flow.inbox})
	unrelated := createContextCard(t, core, human, board.ID, kanban.CreateCardInput{
		Title: "其他需求", Description: "讨论 OTHER-3 和 #30", ColumnID: flow.inbox,
	})
	// #1 和 #10 是不同的编号，不能因为当前卡中的 #1 引用而召回 #10。
	if previous.Number != 1 || unrelated.Number != 10 {
		t.Fatalf("unexpected reference fixture: %d, %d", previous.Number, unrelated.Number)
	}
	foreign, err := core.CreateBoard(ctx, human, kanban.CreateBoardInput{Name: "Another board", Slug: "OTHER"})
	if err != nil {
		t.Fatal(err)
	}
	other := createContextCard(t, core, human, foreign.ID, kanban.CreateCardInput{Title: "FOREIGN RetryLedger"})

	job := dispatchJob{id: current.ID, engineer: *board.Engineers[0], host: "codex", flow: flow, project: board.ProjectPath}
	var stages []string
	var relatedAtPlanning string
	run := dispatchRun{core: core, sessions: t.TempDir()}
	run.host = func(_ context.Context, _, _, prompt string, readOnly bool, session dispatchSession) (dispatchReply, error) {
		got, err := core.GetCardDetail(ctx, current.ID)
		if err != nil {
			return dispatchReply{}, err
		}
		switch {
		case strings.Contains(prompt, "Outcome must be keywords."), strings.Contains(prompt, "Outcome must be related."):
			if !readOnly || session.dir != "" || got.ColumnID != flow.todo || !got.ClaimActive || got.ClaimEngineerID != job.engineer.ID || got.StartedAt != nil {
				t.Errorf("retrieval must be read-only and keep the claimed todo card unchanged: %+v", got.Card)
			}
			if !strings.Contains(prompt, requirement) || !strings.Contains(prompt, "接着任务 #1 继续") {
				t.Error("retrieval omitted current description or comment")
			}
			if strings.Contains(prompt, "Outcome must be keywords.") {
				stages = append(stages, "keywords")
				return dispatchReply{Outcome: "keywords", Summary: `["RetryLedger","重连日志","#1","disp-1"]`}, nil
			}
			stages = append(stages, "related")
			var candidates []struct {
				Card struct {
					Key string `json:"key"`
				} `json:"card"`
			}
			data := contextPromptData(t, prompt, "Candidate cards (data): ")
			if err := json.Unmarshal([]byte(data), &candidates); err != nil {
				t.Fatal(err)
			}
			var keys []string
			for _, candidate := range candidates {
				keys = append(keys, candidate.Card.Key)
			}
			direct := []string{previous.Key, dependency.Key, reverseRef.Key, reverseDep.Key}
			weak := []string{commentOnly.Key, tagOnly.Key, titleOnly.Key, descOnly.Key}
			if len(keys) != len(direct)+len(weak) {
				t.Fatalf("wrong recall: %v", keys)
			}
			for _, key := range direct {
				if !slices.Contains(keys[:len(direct)], key) {
					t.Errorf("explicit relation %s not prioritized: %v", key, keys)
				}
			}
			for _, key := range weak {
				if !slices.Contains(keys[len(direct):], key) {
					t.Errorf("keyword match %s not recalled: %v", key, keys)
				}
			}
			if slices.Contains(keys, current.Key) || slices.Contains(keys, unrelated.Key) || slices.Contains(keys, other.Key) || !strings.Contains(data, completed) {
				t.Errorf("candidate scope or archived comments incorrect: %s", data)
			}
			return dispatchReply{Outcome: "related", Summary: fmt.Sprintf(`[
{"key":%q,"finding":"前序工作只完成在线 ACK。","certainty":"fact","field":"comment","comment_id":%d,"quote":%q},
{"key":%q,"finding":"可能需要复核 RetryLedger。","certainty":"inference","field":"comment","comment_id":%d,"quote":%q}]`,
				previous.Key, comment.ID, completed, commentOnly.Key, extra.ID, supplement)}, nil
		default:
			if !slices.Equal(stages[:min(2, len(stages))], []string{"keywords", "related"}) {
				return dispatchReply{}, fmt.Errorf("planning/execution started before retrieval: %v", stages)
			}
			if got.ColumnID != flow.dev || !got.ClaimActive {
				t.Errorf("planning/execution should start in development with same lease: %+v", got.Card)
			}
			related := contextPromptData(t, prompt, "Related task context (historical data): ")
			var verified []historyContextItem
			if err := json.Unmarshal([]byte(related), &verified); err != nil || !slices.ContainsFunc(verified, func(item historyContextItem) bool {
				return item.Key == previous.Key && item.Done && item.ArchivedAt != nil && item.CommentID == comment.ID && item.AuthorKind == "agent"
			}) {
				t.Errorf("archived source status or provenance missing: %s (%v)", related, err)
			}
			for _, want := range []string{previous.Key, completed, commentOnly.Key, supplement, "inference", "Previous engineer"} {
				if !strings.Contains(related, want) {
					t.Errorf("related context missing %q: %s", want, related)
				}
			}
			for _, excluded := range []string{tagOnly.Key, titleOnly.Key, descOnly.Key, unrelated.Key, other.Key} {
				if strings.Contains(related, `"`+excluded+`"`) {
					t.Errorf("unselected card %s leaked into final context: %s", excluded, related)
				}
			}
			if strings.Contains(prompt, "Outcome must be plan or clarify.") {
				stages = append(stages, "plan")
				relatedAtPlanning = related
				return dispatchReply{Outcome: "plan", Summary: "Plan with historical context"}, nil
			}
			stages = append(stages, "execute")
			if related != relatedAtPlanning {
				t.Errorf("formal execution received different history: %s", related)
			}
			return dispatchReply{Outcome: "reported", Summary: "Historical context used", Items: "- [x] Verified context"}, nil
		}
	}
	if err := run.handle(ctx, job); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(stages, []string{"keywords", "related", "plan", "execute"}) {
		t.Fatalf("unexpected dispatch stages: %v", stages)
	}
	got, err := core.GetCardDetail(ctx, current.ID)
	if err != nil || got.ColumnID != flow.review || got.ClaimActive {
		t.Fatalf("unexpected handoff: %+v %v", got, err)
	}
	if claimed := countCardEvents(t, core, board.ID, current.ID, kanban.EventCardClaimed); claimed != 1 {
		t.Errorf("retrieval must not claim twice, got %d claims", claimed)
	}
}

func historyTitleReply(card *kanban.Card, finding string) dispatchReply {
	return dispatchReply{Outcome: "related", Summary: fmt.Sprintf(`[{"key":%q,"finding":%q,"certainty":"fact","field":"title","quote":%q}]`, card.Key, finding, card.Title)}
}

type historyExportFailureCore struct {
	dispatchCore
	err error
}

func (c historyExportFailureCore) ExportBoard(context.Context, kanban.Actor, int64) (*kanban.BoardExport, error) {
	return nil, c.err
}

type historyExportHookCore struct {
	dispatchCore
	beforeExport func() error
}

func (c historyExportHookCore) ExportBoard(ctx context.Context, actor kanban.Actor, boardID int64) (*kanban.BoardExport, error) {
	if err := c.beforeExport(); err != nil {
		return nil, err
	}
	return c.dispatchCore.ExportBoard(ctx, actor, boardID)
}

func TestDesktopDispatchHistoryFailureKeepsTaskInTodo(t *testing.T) {
	for _, failure := range []string{"database", "keywords", "filter", "fabricated-source"} {
		t.Run(failure, func(t *testing.T) {
			ctx := t.Context()
			core, human, board, flow := newDispatchTestBoard(t)
			previous := createContextCard(t, core, human, board.ID, kanban.CreateCardInput{Title: "Shared feature", ColumnID: flow.done})
			card := createContextCard(t, core, human, board.ID, kanban.CreateCardInput{Title: "Shared follow-up", ColumnID: flow.todo})
			const diagnostic = "private diagnostic"
			sourceErr := errors.New(diagnostic)
			run := dispatchRun{core: core, text: func() desktopText { return desktopTexts["zh-CN"] }}
			if failure == "database" {
				run.core = historyExportFailureCore{dispatchCore: core, err: sourceErr}
			}
			run.host = func(_ context.Context, _, _, prompt string, readOnly bool, _ dispatchSession) (dispatchReply, error) {
				if !readOnly || failure == "database" {
					t.Error("host should not run here")
					return dispatchReply{}, sourceErr
				}
				if strings.Contains(prompt, "Outcome must be keywords.") {
					if failure == "keywords" {
						return dispatchReply{Outcome: "keywords", Summary: "not a JSON array"}, nil
					}
					return dispatchReply{Outcome: "keywords", Summary: `["shared"]`}, nil
				}
				if strings.Contains(prompt, "Outcome must be related.") && failure != "keywords" {
					if failure == "filter" {
						return dispatchReply{}, sourceErr
					}
					return dispatchReply{Outcome: "related", Summary: fmt.Sprintf(`[{"key":%q,"finding":"Unsubstantiated result","certainty":"fact","field":"description","quote":"fabricated quote"}]`, previous.Key)}, nil
				}
				t.Error("planning or execution ran despite failed retrieval")
				return dispatchReply{}, sourceErr
			}
			job := dispatchJob{id: card.ID, engineer: *board.Engineers[0], host: "codex", flow: flow, project: board.ProjectPath}
			if err := run.handle(ctx, job); err == nil || errors.Is(err, errDispatchPlanSuperseded) {
				t.Fatalf("expected a preparation failure, got %v", err)
			}
			got, err := core.GetCardDetail(ctx, card.ID)
			if err != nil || got.ColumnID != flow.todo || got.ClaimActive || got.StartedAt != nil || got.Branch != "" {
				t.Fatalf("failed retrieval moved or changed the task: %+v %v", got, err)
			}
			if gotComment := lastDesktopComment(got); !strings.Contains(gotComment, "无法检索到可靠的关联任务上下文") || strings.Contains(gotComment, diagnostic) || desktopDispatchRecovery(got, false) != dispatchAvailable {
				t.Errorf("expected a safe preparation failure comment, got %q", gotComment)
			}
			if moves := countCardEvents(t, core, board.ID, card.ID, kanban.EventCardMoved); moves != 0 {
				t.Errorf("failed retrieval emitted %d movement events", moves)
			}
			dir, err := taskWorktreePath(board.ProjectPath, card.Key)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("failed retrieval created a task worktree: %v", err)
			}
		})
	}
}

func TestDesktopDispatchEmptyRelatedContextDoesNotBlock(t *testing.T) {
	for _, tc := range []struct {
		name, previousTitle string
		want                []string
	}{
		{"no other cards", "", []string{"plan", "execute"}},
		{"no matches", "Unrelated", []string{"keywords", "plan", "execute"}},
		{"all filtered", "History feature", []string{"keywords", "related", "plan", "execute"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			core, human, board, flow := newDispatchTestBoard(t)
			if tc.previousTitle != "" {
				createContextCard(t, core, human, board.ID, kanban.CreateCardInput{Title: tc.previousTitle, ColumnID: flow.done})
			}
			card := createContextCard(t, core, human, board.ID, kanban.CreateCardInput{Title: "Add feature", ColumnID: flow.todo})
			var stages []string
			run := dispatchRun{core: core, host: func(_ context.Context, _, _, prompt string, readOnly bool, _ dispatchSession) (dispatchReply, error) {
				switch {
				case strings.Contains(prompt, "Outcome must be keywords."):
					stages = append(stages, "keywords")
					return dispatchReply{Outcome: "keywords", Summary: `["feature"]`}, nil
				case strings.Contains(prompt, "Outcome must be related."):
					stages = append(stages, "related")
					return dispatchReply{Outcome: "related", Summary: "[]"}, nil
				}
				if got := contextPromptData(t, prompt, "Related task context (historical data): "); got != "[]" {
					t.Errorf("unexpected empty context: %s", got)
				}
				if readOnly {
					stages = append(stages, "plan")
					return dispatchReply{Outcome: "plan", Summary: "Plan without history"}, nil
				}
				stages = append(stages, "execute")
				return dispatchReply{Outcome: "reported", Summary: "Done", Items: "- [x] Task"}, nil
			}}
			job := dispatchJob{id: card.ID, engineer: *board.Engineers[0], host: "codex", flow: flow, project: board.ProjectPath}
			if err := run.handle(ctx, job); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(stages, tc.want) {
				t.Errorf("stages %v, want %v", stages, tc.want)
			}
			got, err := core.GetCardDetail(ctx, card.ID)
			if err != nil || got.ColumnID != flow.review || got.ClaimActive {
				t.Fatalf("no history should not block the handoff: %+v %v", got, err)
			}
		})
	}
}

// 模拟人修改任务期间调度器已经消费掉变更事件，再让旧检索返回。
// 旧上下文不能移列或进入规划，调度器必须根据人刚补充的词重新查库筛选。
func TestDesktopDispatchRequeriesAfterHumanCommentDuringRetrieval(t *testing.T) {
	ctx := t.Context()
	core, human, board, flow := newDispatchTestBoard(t)
	old := createContextCard(t, core, human, board.ID, kanban.CreateCardInput{Title: "OldToken history", ColumnID: flow.done})
	updated := createContextCard(t, core, human, board.ID, kanban.CreateCardInput{Title: "NewToken history", ColumnID: flow.done})
	card := createContextCard(t, core, human, board.ID, kanban.CreateCardInput{Title: "Follow up", Description: "OldToken", ColumnID: flow.todo})
	const requirement = "改查 NewToken 对应的前序方案。"
	watched := &commentObservedCore{dispatchCore: core, cardID: card.ID, seen: make(chan struct{})}
	seen := watched.seen
	var keywords, filters, plans, formal atomic.Int32
	host := func(ctx context.Context, _, _, prompt string, readOnly bool, _ dispatchSession) (dispatchReply, error) {
		switch {
		case strings.Contains(prompt, "Outcome must be keywords."):
			if keywords.Add(1) == 1 {
				return dispatchReply{Outcome: "keywords", Summary: `["OldToken"]`}, nil
			}
			if !strings.Contains(prompt, requirement) {
				t.Error("new retrieval did not see the human comment")
			}
			return dispatchReply{Outcome: "keywords", Summary: `["NewToken"]`}, nil
		case strings.Contains(prompt, "Outcome must be related."):
			got, err := core.GetCardDetail(ctx, card.ID)
			if err != nil || got.ColumnID != flow.todo {
				t.Errorf("card moved before retrieval: %+v %v", got, err)
			}
			if filters.Add(1) == 1 {
				if _, err := core.AddComment(ctx, human, card.ID, requirement); err != nil {
					return dispatchReply{}, err
				}
				select {
				case <-seen:
				case <-ctx.Done():
					return dispatchReply{}, ctx.Err()
				}
				return historyTitleReply(old, "OLD result"), nil
			}
			if !strings.Contains(prompt, updated.Title) || strings.Contains(prompt, old.Title) {
				t.Error("second recall did not use the new keyword")
			}
			return historyTitleReply(updated, "NEW result"), nil
		default:
			if !strings.Contains(prompt, "NEW result") || strings.Contains(prompt, "OLD result") {
				t.Error("a superseded retrieval result reached the engineer")
			}
			if readOnly {
				plans.Add(1)
				return dispatchReply{Outcome: "plan", Summary: "Fresh plan"}, nil
			}
			formal.Add(1)
			return dispatchReply{Outcome: "reported", Summary: "Done", Items: "- [x] Report"}, nil
		}
	}
	startDispatchFixture(t, watched, board, host)
	waitDesktopCheck(t, func() bool {
		got, err := core.GetCardDetail(ctx, card.ID)
		return err == nil && got.ColumnID == flow.review && !got.ClaimActive
	})
	if keywords.Load() != 2 || filters.Load() != 2 || plans.Load() != 1 || formal.Load() != 1 {
		t.Fatalf("expected a discarded retrieval and one successful dispatch, got %d/%d/%d/%d", keywords.Load(), filters.Load(), plans.Load(), formal.Load())
	}
}

func TestDesktopDispatchSupersedesHistoryAfterTaskOrLeaseChanges(t *testing.T) {
	for _, change := range []string{"description", "delete earlier comment", "agent comment", "lease"} {
		t.Run(change, func(t *testing.T) {
			ctx := t.Context()
			core, human, board, flow := newDispatchTestBoard(t)
			previous := createContextCard(t, core, human, board.ID, kanban.CreateCardInput{Title: "Feature history", ColumnID: flow.done})
			card := createContextCard(t, core, human, board.ID, kanban.CreateCardInput{Title: "Feature follow-up", ColumnID: flow.todo})
			earlier, err := core.AddComment(ctx, human, card.ID, "Earlier requirement")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := core.AddComment(ctx, human, card.ID, "Last requirement"); err != nil {
				t.Fatal(err)
			}
			job := dispatchJob{id: card.ID, engineer: *board.Engineers[0], host: "codex", flow: flow, project: board.ProjectPath}
			other := kanban.Actor{Kind: kanban.ActorAgent, Name: "another agent"}
			run := dispatchRun{core: core, host: func(_ context.Context, _, _, prompt string, _ bool, _ dispatchSession) (dispatchReply, error) {
				if strings.Contains(prompt, "Outcome must be keywords.") {
					return dispatchReply{Outcome: "keywords", Summary: `["feature"]`}, nil
				}
				if !strings.Contains(prompt, "Outcome must be related.") {
					return dispatchReply{}, errors.New("stale retrieval reached planning")
				}
				switch change {
				case "description":
					if _, err := core.UpdateCard(ctx, human, card.ID, kanban.UpdateCardInput{Description: new("New description")}); err != nil {
						t.Fatal(err)
					}
				case "delete earlier comment":
					if err := core.DeleteComment(ctx, human, earlier.ID); err != nil {
						t.Fatal(err)
					}
				case "agent comment":
					if _, err := core.AddComment(ctx, other, card.ID, "Additional context"); err != nil {
						t.Fatal(err)
					}
				case "lease":
					if err := core.ReleaseCard(ctx, human, card.ID); err != nil {
						t.Fatal(err)
					}
					if _, err := core.ClaimCard(ctx, other, card.ID, kanban.ClaimCardInput{EngineerID: job.engineer.ID}); err != nil {
						t.Fatal(err)
					}
				}
				return historyTitleReply(previous, "Stale result"), nil
			}}
			if err := run.handle(ctx, job); !errors.Is(err, errDispatchPlanSuperseded) {
				t.Fatalf("expected the stale retrieval to be discarded, got %v", err)
			}
			got, err := core.GetCardDetail(ctx, card.ID)
			if err != nil || got.ColumnID != flow.todo || got.StartedAt != nil || got.Branch != "" || desktopDispatchRecovery(got, false) != dispatchAvailable {
				t.Fatalf("stale retrieval changed task state: %+v %v", got, err)
			}
			if change == "lease" {
				if !got.ClaimActive || got.ClaimedBy != other.Name {
					t.Errorf("the new holder lost its claim: %+v", got.Card)
				}
			} else if got.ClaimActive {
				t.Error("stale dispatch leaked a claim")
			}
		})
	}
}

func TestDesktopDispatchRenewsLeaseThroughoutHistoryRetrieval(t *testing.T) {
	ctx := t.Context()
	core, human, board, flow := newDispatchTestBoard(t, kanban.WithClock(func() time.Time { return time.Now().Add(-25 * time.Minute) }))
	previous := createContextCard(t, core, human, board.ID, kanban.CreateCardInput{Title: "Feature history", ColumnID: flow.done})
	card := createContextCard(t, core, human, board.ID, kanban.CreateCardInput{Title: "Feature follow-up", ColumnID: flow.todo})
	counting := &renewCountingCore{dispatchCore: core}
	waitRenewal := func() error {
		before, err := core.GetCardDetail(ctx, card.ID)
		if err != nil || before.ClaimExpiresAt == nil {
			return fmt.Errorf("missing retrieval lease: %v", err)
		}
		count := counting.renewals.Load()
		waitDesktopCheck(t, func() bool {
			got, err := core.GetCardDetail(ctx, card.ID)
			return err == nil && got.ClaimActive && got.ColumnID == flow.todo && got.ClaimExpiresAt.After(*before.ClaimExpiresAt) && counting.renewals.Load() > count
		})
		return nil
	}
	// 模拟数据库读取和两个只读宿主都被阻塞：在每个阶段都必须能收到后续的真实续租。
	run := dispatchRun{core: historyExportHookCore{dispatchCore: counting, beforeExport: waitRenewal}, renewEvery: 10 * time.Millisecond}
	run.host = func(_ context.Context, _, _, prompt string, _ bool, _ dispatchSession) (dispatchReply, error) {
		if strings.Contains(prompt, "Outcome must be keywords.") || strings.Contains(prompt, "Outcome must be related.") {
			if err := waitRenewal(); err != nil {
				return dispatchReply{}, err
			}
			if strings.Contains(prompt, "Outcome must be keywords.") {
				return dispatchReply{Outcome: "keywords", Summary: `["feature"]`}, nil
			}
			return historyTitleReply(previous, "History"), nil
		}
		return dispatchReply{Outcome: "clarify", Summary: "Need clarification"}, nil
	}
	job := dispatchJob{id: card.ID, engineer: *board.Engineers[0], host: "codex", flow: flow, project: board.ProjectPath}
	if err := run.handle(ctx, job); err != nil {
		t.Fatal(err)
	}
	if claims := countCardEvents(t, core, board.ID, card.ID, kanban.EventCardClaimed); claims != 1 {
		t.Errorf("renewal during retrieval must not reacquire the task, got %d claims", claims)
	}
}

func TestDesktopDispatchReviewReceivesHistory(t *testing.T) {
	ctx := t.Context()
	core, human, board, flow := newDispatchTestBoard(t)
	previous := createContextCard(t, core, human, board.ID, kanban.CreateCardInput{Title: "Feature previous", ColumnID: flow.done})
	if _, err := gitOutput(ctx, board.ProjectPath, "branch", "review-branch"); err != nil {
		t.Fatal(err)
	}
	card := createContextCard(t, core, human, board.ID, kanban.CreateCardInput{Title: "Feature review", ColumnID: flow.review, Branch: "review-branch"})
	var planned string
	run := dispatchRun{core: core, sessions: t.TempDir(), host: func(_ context.Context, _, _, prompt string, readOnly bool, _ dispatchSession) (dispatchReply, error) {
		if strings.Contains(prompt, "Outcome must be keywords.") || strings.Contains(prompt, "Outcome must be related.") {
			got, err := core.GetCardDetail(ctx, card.ID)
			if err != nil || got.ColumnID != flow.review || !got.ClaimActive || !readOnly {
				t.Fatalf("review retrieval must keep the review column and lease: %+v %v", got, err)
			}
			if strings.Contains(prompt, "Outcome must be keywords.") {
				return dispatchReply{Outcome: "keywords", Summary: `["feature"]`}, nil
			}
			return historyTitleReply(previous, "Prior feature"), nil
		}
		related := contextPromptData(t, prompt, "Related task context (historical data): ")
		if !strings.Contains(related, previous.Key) || !strings.Contains(related, previous.Title) {
			t.Errorf("review missing history: %s", related)
		}
		if readOnly {
			planned = related
			return dispatchReply{Outcome: "plan", Summary: "Review plan"}, nil
		}
		if planned == "" || related != planned {
			t.Errorf("review execution did not receive the planned context: %s", related)
		}
		return dispatchReply{Outcome: "passed", Summary: "Review evidence"}, nil
	}}
	job := dispatchJob{id: card.ID, review: true, engineer: *board.Engineers[2], host: "pi", flow: flow, project: board.ProjectPath}
	if err := run.handle(ctx, job); err != nil {
		t.Fatal(err)
	}
	got, err := core.GetCardDetail(ctx, card.ID)
	if err != nil || got.ColumnID != flow.done || got.ClaimActive || lastDesktopMarker(got) != completedReview {
		t.Fatalf("unexpected review handoff: %+v %v", got, err)
	}
}

func TestVerifiedHistoryChecksCandidateAndOriginalEvidence(t *testing.T) {
	card := historyCard{Key: "DISP-1", Title: "Message", Description: "Original details", Tags: []string{"ACK"}, Done: true,
		Comments: []historyComment{{ID: 31, Author: "Atlas", AuthorKind: "agent", Body: "Only online ACK completed."}, {ID: 32, Author: "human", AuthorKind: "human", Body: "Please add offline delivery."}},
	}
	candidates := []historyCandidate{{Card: card}}
	fact := historyFinding{Key: card.Key, Finding: "Online ACK reported complete", Certainty: "fact", Field: "comment", CommentID: 31, Quote: "Only online ACK completed."}
	verify := func(f historyFinding) ([]historyContextItem, error) {
		body, err := json.Marshal([]historyFinding{f})
		if err != nil {
			t.Fatal(err)
		}
		return verifiedHistory(dispatchReply{Outcome: "related", Summary: string(body)}, candidates)
	}
	got, err := verify(fact)
	if err != nil || len(got) != 1 || got[0].Author != "Atlas" || got[0].AuthorKind != "agent" || !got[0].Done || got[0].Quote != fact.Quote {
		t.Fatalf("real evidence not enriched from the snapshot: %+v %v", got, err)
	}
	for _, tc := range []struct {
		name   string
		change func(*historyFinding)
	}{
		{"different card", func(f *historyFinding) { f.Key = "OTHER-1" }},
		{"fabricated quote", func(f *historyFinding) { f.Quote = "Offline delivery completed." }},
		{"wrong comment", func(f *historyFinding) { f.CommentID = 32 }},
		{"unknown certainty", func(f *historyFinding) { f.Certainty = "maybe" }},
		{"empty evidence", func(f *historyFinding) { f.Quote = " " }},
		{"wrong field", func(f *historyFinding) { f.Field = "description" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			invalid := fact
			tc.change(&invalid)
			if _, err := verify(invalid); !errors.Is(err, errUnconfirmedRun) {
				t.Errorf("invalid evidence accepted: %+v (%v)", invalid, err)
			}
		})
	}
}

func TestHistoryBatchesPreserveAllRecalledText(t *testing.T) {
	// 单张卡也可能有很多评论，长卡应单独成批而不是被截断或者丢掉。
	large := strings.Repeat("long history ", historyBatchBytes/4)
	candidates := []historyCandidate{
		{Card: historyCard{Key: "DISP-1", Description: "first"}},
		{Card: historyCard{Key: "DISP-2", Comments: []historyComment{{Body: large}}}},
		{Card: historyCard{Key: "DISP-3", Description: "last"}},
	}
	batches, err := historyBatches(candidates)
	if err != nil || len(batches) != 3 {
		t.Fatalf("expected all three batches, got %d (%v)", len(batches), err)
	}
	flat := slices.Concat(batches...)
	if len(flat) != len(candidates) || flat[0].Card.Key != candidates[0].Card.Key || flat[1].Card.Comments[0].Body != large || flat[2].Card.Key != candidates[2].Card.Key {
		t.Fatal("batching lost or reordered evidence")
	}
}
