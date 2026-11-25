package desktop

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"slate-kanban/internal/core/kanban"
)

func mergeTestGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	got, err := gitOutput(t.Context(), dir, args...)
	if err != nil {
		t.Fatalf("git %v in %s: %v", args, dir, err)
	}
	return got
}

func mergeTestCommit(t *testing.T, dir, file, content string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	mergeTestGit(t, dir, "add", "--", file)
	mergeTestGit(t, dir, "commit", "-m", file)
	return mergeTestGit(t, dir, "rev-parse", "HEAD")
}

func newMergeTestCard(t *testing.T, core kanban.Core, human kanban.Actor, board *kanban.BoardDetail, flow dispatchFlow) (*kanban.Card, string) {
	t.Helper()
	card, err := core.CreateCard(t.Context(), human, board.ID, kanban.CreateCardInput{Title: "Local integration", ColumnID: flow.review})
	if err != nil {
		t.Fatal(err)
	}
	branch := "slate/" + strings.ToLower(card.Key) + "-task"
	card, err = core.UpdateCard(t.Context(), human, card.ID, kanban.UpdateCardInput{Branch: &branch})
	if err != nil {
		t.Fatal(err)
	}
	dir, err := taskWorktreePath(board.ProjectPath, card.Key)
	if err != nil {
		t.Fatal(err)
	}
	mergeTestGit(t, board.ProjectPath, "worktree", "add", "-b", branch, "--", dir, "HEAD")
	return card, dir
}

func setupMergeTest(t *testing.T) (kanban.Core, kanban.Actor, *kanban.BoardDetail, dispatchFlow) {
	t.Helper()
	core, human, board, flow := newDispatchTestBoard(t)
	mergeTestGit(t, board.ProjectPath, "config", "user.name", "Test")
	mergeTestGit(t, board.ProjectPath, "config", "user.email", "test@example.local")
	mergeTestGit(t, board.ProjectPath, "config", "commit.gpgsign", "false")
	return core, human, board, flow
}

func TestDesktopDispatchReviewIntegratesApprovedBranch(t *testing.T) {
	core, human, board, flow := setupMergeTest(t)
	card, source := newMergeTestCard(t, core, human, board, flow)
	mergeTestCommit(t, source, "feature.txt", "approved\n")
	mainBefore := mergeTestCommit(t, board.ProjectPath, "main.txt", "independent change\n")
	host, calls := newReviewHost("passed")
	run := dispatchRun{core: core, host: host, sessions: t.TempDir()}
	job := dispatchJob{id: card.ID, review: true, engineer: *board.Engineers[2], host: "pi", flow: flow, project: board.ProjectPath}
	if err := run.handle(t.Context(), job); err != nil {
		t.Fatal(err)
	}
	got, err := core.GetCardDetail(t.Context(), card.ID)
	if err != nil {
		t.Fatal(err)
	}
	head := mergeTestGit(t, board.ProjectPath, "rev-parse", "HEAD")
	if got.ColumnID != flow.done || !slices.Equal(got.MergedCommits, []string{head[:6]}) || got.ClaimActive {
		t.Fatalf("want integrated completion, got column=%d merged=%v claim=%v", got.ColumnID, got.MergedCommits, got.ClaimActive)
	}
	if calls.Load() != 1 || head == mainBefore || mergeTestGit(t, board.ProjectPath, "rev-parse", "HEAD^") != mainBefore {
		t.Fatalf("want one review and a linear integration, calls=%d head=%s", calls.Load(), head)
	}
	for file, want := range map[string]string{"feature.txt": "approved\n", "main.txt": "independent change\n"} {
		data, err := os.ReadFile(filepath.Join(board.ProjectPath, file))
		if err != nil || string(data) != want {
			t.Fatalf("%s: %q, %v", file, data, err)
		}
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) || mergeTestGit(t, board.ProjectPath, "branch", "--list", card.Branch) != "" {
		t.Fatalf("integrated worktree or branch was not cleaned: %v", err)
	}
}

func TestDesktopDispatchReviewRetainsDirtyMainAndResumes(t *testing.T) {
	core, human, board, flow := setupMergeTest(t)
	card, source := newMergeTestCard(t, core, human, board, flow)
	mergeTestCommit(t, source, "feature.txt", "approved\n")
	before := mergeTestGit(t, board.ProjectPath, "rev-parse", "HEAD")
	dirty := filepath.Join(board.ProjectPath, "local.txt")
	if err := os.WriteFile(dirty, []byte("local work"), 0o600); err != nil {
		t.Fatal(err)
	}
	host, calls := newReviewHost("passed")
	run := dispatchRun{core: core, host: host, sessions: t.TempDir()}
	job := dispatchJob{id: card.ID, review: true, engineer: *board.Engineers[2], host: "pi", flow: flow, project: board.ProjectPath}
	if err := run.handle(t.Context(), job); err == nil {
		t.Fatal("dirty main checkout must block integration")
	}
	got, err := core.GetCardDetail(t.Context(), card.ID)
	if err != nil || got.ColumnID != flow.review || len(got.MergedCommits) != 0 || mergeTestGit(t, board.ProjectPath, "rev-parse", "HEAD") != before {
		t.Fatalf("blocked merge advanced task or main: %v, %v", got, err)
	}
	if data, err := os.ReadFile(dirty); err != nil || string(data) != "local work" {
		t.Fatalf("local changes not preserved: %q %v", data, err)
	}
	if err := os.Remove(dirty); err != nil {
		t.Fatal(err)
	}
	if err := run.handle(t.Context(), job); err != nil {
		t.Fatal(err)
	}
	got, err = core.GetCardDetail(t.Context(), card.ID)
	if err != nil || got.ColumnID != flow.done || len(got.MergedCommits) != 1 || calls.Load() != 1 {
		t.Fatalf("approved merge was not resumed without another model call: %v, %v, calls=%d", got, err, calls.Load())
	}
}

type mergeFinishFailure struct {
	dispatchCore
	fail bool
}

func (c *mergeFinishFailure) FinishCardWithExt(ctx context.Context, actor kanban.Actor, id int64, in kanban.FinishCardInput, ext string) (*kanban.CardDetail, error) {
	if c.fail && len(in.MergedCommits) > 0 {
		c.fail = false
		return nil, context.DeadlineExceeded
	}
	return c.dispatchCore.FinishCardWithExt(ctx, actor, id, in, ext)
}

func TestDesktopDispatchReviewResumesPublishedReceipt(t *testing.T) {
	core, human, board, flow := setupMergeTest(t)
	card, source := newMergeTestCard(t, core, human, board, flow)
	mergeTestCommit(t, source, "feature.txt", "approved\n")
	host, calls := newReviewHost("passed")
	run := dispatchRun{core: &mergeFinishFailure{dispatchCore: core, fail: true}, host: host, sessions: t.TempDir()}
	job := dispatchJob{id: card.ID, review: true, engineer: *board.Engineers[2], host: "pi", flow: flow, project: board.ProjectPath}
	if err := run.handle(t.Context(), job); err == nil {
		t.Fatal("simulated handoff failure was not reported")
	}
	published := mergeTestGit(t, board.ProjectPath, "rev-parse", "HEAD")
	got, err := core.GetCardDetail(t.Context(), card.ID)
	if err != nil || got.ColumnID != flow.review || len(got.MergedCommits) != 0 {
		t.Fatalf("failed board handoff advanced card: %v, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(board.ProjectPath, "feature.txt")); err != nil {
		t.Fatal("Git integration did not reach main before failed handoff", err)
	}
	restarted := dispatchRun{core: core, host: host, sessions: run.sessions}
	if err := restarted.handle(t.Context(), job); err != nil {
		t.Fatal(err)
	}
	got, err = core.GetCardDetail(t.Context(), card.ID)
	if err != nil || got.ColumnID != flow.done || !slices.Equal(got.MergedCommits, []string{published[:6]}) || calls.Load() != 1 ||
		mergeTestGit(t, board.ProjectPath, "rev-parse", "HEAD") != published {
		t.Fatalf("published approval was not resumed exactly once: %v, %v, calls=%d", got, err, calls.Load())
	}
}

// 项目分支和任务分支改了同一处时，程序不会硬合并：任务带着冲突说明退回「进行中」，
// 项目分支原样不动，也不会再调用模型审查。
func TestDesktopDispatchReviewReturnsConflictToDevelopment(t *testing.T) {
	core, human, board, flow := setupMergeTest(t)
	card, source := newMergeTestCard(t, core, human, board, flow)
	mergeTestCommit(t, source, "feature.txt", "branch\n")
	mainBefore := mergeTestCommit(t, board.ProjectPath, "feature.txt", "main\n")
	host, calls := newReviewHost("passed")
	run := dispatchRun{core: core, host: host, sessions: t.TempDir()}
	job := dispatchJob{id: card.ID, review: true, engineer: *board.Engineers[2], host: "pi", flow: flow, project: board.ProjectPath}
	if err := run.handle(t.Context(), job); err != nil {
		t.Fatal(err)
	}
	got, err := core.GetCardDetail(t.Context(), card.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ColumnID != flow.dev || len(got.MergedCommits) != 0 || got.ClaimActive || calls.Load() != 1 {
		t.Fatalf("conflict must return to development: column=%d merged=%v claim=%v calls=%d", got.ColumnID, got.MergedCommits, got.ClaimActive, calls.Load())
	}
	if mergeTestGit(t, board.ProjectPath, "rev-parse", "HEAD") != mainBefore || mergeTestGit(t, board.ProjectPath, "status", "--porcelain") != "" {
		t.Fatal("a conflicting integration must leave the project checkout untouched")
	}
	if !strings.Contains(lastDesktopComment(got), mainBefore[:12]) || rebaseTarget(got) != mainBefore {
		t.Fatalf("conflict comment does not name the commit to rebase onto: %q", lastDesktopComment(got))
	}
}

// 冲突退回后，研发把分支变基到项目分支再交接，审查通过后程序合并成功。
// 变基改写了历史，研发交接不能再要求旧提交仍是祖先。
func TestDesktopDispatchConflictIsResolvedByRebaseAndMerged(t *testing.T) {
	core, human, board, flow := setupMergeTest(t)
	card, source := newMergeTestCard(t, core, human, board, flow)
	mergeTestCommit(t, source, "feature.txt", "branch\n")
	mainTip := mergeTestCommit(t, board.ProjectPath, "feature.txt", "main\n")
	var rebased bool
	host := func(ctx context.Context, _, dir, prompt string, _ bool, _ dispatchSession) (dispatchReply, error) {
		switch {
		case strings.Contains(prompt, "Outcome must be plan or clarify."):
			return dispatchReply{Outcome: "plan", Summary: "Plan"}, nil
		case strings.Contains(prompt, "Outcome must be ready or reported."):
			rebased = true
			_, err := gitOutput(ctx, dir, "rebase", "-X", "theirs", mainTip)
			return dispatchReply{Outcome: "ready", Summary: "Rebased", Items: "- [x] resolved"}, err
		}
		return dispatchReply{Outcome: "passed", Summary: "Evidence"}, nil
	}
	run := dispatchRun{core: core, host: host, sessions: t.TempDir()}
	review := dispatchJob{id: card.ID, review: true, engineer: *board.Engineers[2], host: "pi", flow: flow, project: board.ProjectPath}
	if err := run.handle(t.Context(), review); err != nil {
		t.Fatal(err)
	}
	develop := dispatchJob{id: card.ID, engineer: *board.Engineers[0], host: "pi", flow: flow, project: board.ProjectPath}
	if err := run.handle(t.Context(), develop); err != nil {
		t.Fatal(err)
	}
	got, err := core.GetCardDetail(t.Context(), card.ID)
	if err != nil || !rebased || got.ColumnID != flow.review {
		t.Fatalf("rebased development was not handed off to review: column=%d rebased=%v err=%v", got.ColumnID, rebased, err)
	}
	if err := run.handle(t.Context(), review); err != nil {
		t.Fatal(err)
	}
	got, err = core.GetCardDetail(t.Context(), card.ID)
	if err != nil || got.ColumnID != flow.done || len(got.MergedCommits) != 1 {
		t.Fatalf("resolved branch was not merged: %v, %v", got, err)
	}
	if data, err := os.ReadFile(filepath.Join(board.ProjectPath, "feature.txt")); err != nil || string(data) != "branch\n" {
		t.Fatalf("resolved content missing from the project branch: %q, %v", data, err)
	}
}

// 不是专属分支、或分支没有领先提交的任务不合并，沿用原来的做法直接完成，卡片不带合并记录。
func TestDesktopDispatchReviewSkipsIntegrationWithoutNewCommits(t *testing.T) {
	core, human, board, flow := setupMergeTest(t)
	card, _ := newMergeTestCard(t, core, human, board, flow)
	host, _ := newReviewHost("passed")
	run := dispatchRun{core: core, host: host, sessions: t.TempDir()}
	job := dispatchJob{id: card.ID, review: true, engineer: *board.Engineers[2], host: "pi", flow: flow, project: board.ProjectPath}
	if err := run.handle(t.Context(), job); err != nil {
		t.Fatal(err)
	}
	got, err := core.GetCardDetail(t.Context(), card.ID)
	if err != nil || got.ColumnID != flow.done || len(got.MergedCommits) != 0 {
		t.Fatalf("task without commits should complete without a merge record: %v, %v", got, err)
	}
}

// 审批之后任务分支又有了新提交，旧审批作废，必须重新审查而不是合并没审过的代码。
func TestDesktopDispatchReviewReviewsAgainWhenBranchMovedAfterApproval(t *testing.T) {
	core, human, board, flow := setupMergeTest(t)
	card, source := newMergeTestCard(t, core, human, board, flow)
	mergeTestCommit(t, source, "feature.txt", "approved\n")
	dirty := filepath.Join(board.ProjectPath, "local.txt")
	if err := os.WriteFile(dirty, []byte("local work"), 0o600); err != nil {
		t.Fatal(err)
	}
	host, calls := newReviewHost("passed")
	run := dispatchRun{core: core, host: host, sessions: t.TempDir()}
	job := dispatchJob{id: card.ID, review: true, engineer: *board.Engineers[2], host: "pi", flow: flow, project: board.ProjectPath}
	if err := run.handle(t.Context(), job); err == nil {
		t.Fatal("dirty main checkout must block integration")
	}
	if err := os.Remove(dirty); err != nil {
		t.Fatal(err)
	}
	mergeTestCommit(t, source, "more.txt", "unreviewed\n")
	if err := run.handle(t.Context(), job); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("a moved branch must be reviewed again, formal reviews = %d", calls.Load())
	}
}
