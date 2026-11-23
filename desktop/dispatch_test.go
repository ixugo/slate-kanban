package desktop

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"slate-kanban/internal/app"
	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
	"slate-kanban/internal/data"
)

func newDispatchTestBoard(t *testing.T, opts ...kanban.Option) (kanban.Core, kanban.Actor, *kanban.BoardDetail, dispatchFlow) {
	t.Helper()
	ctx := t.Context()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(dir, "project")
	if err := os.Mkdir(project, 0o700); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{{"init", "-b", "main"}, {"-c", "user.name=Test", "-c", "user.email=test@example.local", "commit", "--allow-empty", "-m", "Initial"}} {
		if _, err := gitOutput(ctx, project, args...); err != nil {
			t.Fatal(err)
		}
	}

	home := filepath.Join(dir, "home")
	if err := os.Mkdir(home, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	configDir := filepath.Join(dir, "data")
	cfg, closeConfig, err := app.OpenConfig(configDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeConfig)
	database := filepath.Join(dir, "dispatch.sqlite")
	cfg.Data.Database.Dsn = database
	actualHome, err := os.UserHomeDir()
	if err != nil || actualHome != home || !filepath.IsAbs(database) || filepath.Dir(cfg.Data.Database.Dsn) != dir || cfg.Runtime.ConfigDir != configDir {
		t.Fatal("desktop dispatch test configuration escaped its temporary directory")
	}
	db, closeDB, err := data.SetupDB(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeDB)
	if _, err := os.Lstat(database); err != nil {
		t.Fatalf("expected isolated SQLite database: %v", err)
	}
	core := kanban.NewCore(kanbandb.NewDB(db).AutoMigrate(true), opts...)
	human := kanban.Actor{Kind: kanban.ActorHuman, Name: "tester"}
	board, err := core.CreateBoard(ctx, human, kanban.CreateBoardInput{Name: "Desktop workflow", Slug: "DISP", ProjectPath: project})
	if err != nil {
		t.Fatal(err)
	}
	flow, err := templateFlow(board.Columns)
	if err != nil {
		t.Fatal(err)
	}
	return core, human, board, flow
}

func TestDesktopDispatchRunsDevelopmentThenReview(t *testing.T) {
	ctx := t.Context()
	core, human, board, flow := newDispatchTestBoard(t)
	card, err := core.CreateCard(ctx, human, board.ID, kanban.CreateCardInput{Title: "Implement", ColumnID: flow.todo})
	if err != nil {
		t.Fatal(err)
	}
	protected, err := core.CreateCard(ctx, human, board.ID, kanban.CreateCardInput{Title: "Manual", ColumnID: flow.dev, HumanOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	competing, err := core.CreateCard(ctx, human, board.ID, kanban.CreateCardInput{Title: "Claimed elsewhere", ColumnID: flow.dev, EngineerID: board.Engineers[1].ID})
	if err != nil {
		t.Fatal(err)
	}
	other := kanban.Actor{Kind: kanban.ActorAgent, Name: "other-machine"}
	if _, err := core.ClaimCard(ctx, other, competing.ID, kanban.ClaimCardInput{EngineerID: board.Engineers[1].ID}); err != nil {
		t.Fatal(err)
	}
	blocked, err := core.CreateCard(ctx, human, board.ID, kanban.CreateCardInput{Title: "Unconfirmed", ColumnID: flow.dev, EngineerID: board.Engineers[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	prevDesktop := kanban.Actor{Kind: kanban.ActorAgent, Name: "desktop-previous"}
	if _, err := core.ClaimCard(ctx, prevDesktop, blocked.ID, kanban.ClaimCardInput{EngineerID: board.Engineers[0].ID}); err != nil {
		t.Fatal(err)
	}
	const previousCode = "prior-recovery-code"
	marker := (dispatchJob{}).retryPrefix() + previousCode + " -->"
	if _, err := core.AddComment(ctx, prevDesktop, blocked.ID, "Prior run outcome unknown.\n\n"+marker); err != nil {
		t.Fatal(err)
	}
	if err := core.ReleaseCard(ctx, prevDesktop, blocked.ID); err != nil {
		t.Fatal(err)
	}

	var calls atomic.Int32
	var concurrent atomic.Int32
	host := func(ctx context.Context, id, dir, prompt string, readOnly bool, _ dispatchSession) (dispatchReply, error) {
		if concurrent.Add(1) != 1 {
			t.Error("same workflow card dispatched concurrently")
		}
		defer concurrent.Add(-1)
		calls.Add(1)
		if strings.Contains(prompt, `"title":"Unconfirmed"`) {
			return dispatchReply{Outcome: "clarify", Summary: "Human comment triggered reevaluation"}, nil
		}
		if !strings.Contains(prompt, `"title":"Implement"`) {
			t.Error("unexpected card dispatched")
		}
		if strings.Contains(prompt, "Outcome must be plan or clarify.") {
			if !readOnly {
				t.Error("planning host was given write capability")
			}
			return dispatchReply{Outcome: "plan", Summary: "Isolated plan"}, nil
		}
		if id == "pi" {
			if !readOnly {
				t.Error("formal review host was given write capability")
			}
			return dispatchReply{Outcome: "passed", Summary: "Review evidence"}, nil
		}
		if err := os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("implementation"), 0o600); err != nil {
			return dispatchReply{}, err
		}
		for _, args := range [][]string{{"add", "feature.txt"}, {"-c", "user.name=Test", "-c", "user.email=test@example.local", "commit", "-m", "Implement"}} {
			if _, err := gitOutput(ctx, dir, args...); err != nil {
				return dispatchReply{}, err
			}
		}
		return dispatchReply{Outcome: "ready", Summary: "Isolated implementation", Items: "- [x] Implementation"}, nil
	}
	hosts := map[int64]string{
		board.Engineers[0].ID: "codex",
		board.Engineers[1].ID: "claude",
		board.Engineers[2].ID: "pi",
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	dispatch := dispatchRun{core: core, config: dispatchConfig{BoardID: board.ID, Hosts: hosts}, actor: desktopDispatchActor(), host: host, update: func(int64, engineerStatus) {}}
	snapshot, err := core.BoardSnapshot(ctx, board.ID)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		defer close(done)
		dispatch.run(runCtx, snapshot)
	}()
	t.Cleanup(func() { cancel(); <-done })
	waitDesktopCheck(t, func() bool {
		got, err := core.GetCardDetail(ctx, card.ID)
		return err == nil && got.ColumnID == flow.review && !got.ClaimActive &&
			strings.Contains(got.Description, "- [x] Implementation") && desktopDispatchRecovery(got, true) == dispatchAwaitingConfirmation
	})
	if got := calls.Load(); got != 4 {
		t.Fatalf("wanted the development and review phases only, got %d host calls", got)
	}
	for _, check := range []struct{ id, column int64 }{{protected.ID, flow.dev}, {competing.ID, flow.dev}, {blocked.ID, flow.dev}} {
		got, err := core.GetCardDetail(ctx, check.id)
		if err != nil || got.ColumnID != check.column {
			t.Fatalf("protected card changed: %v (%v)", got, err)
		}
	}

	if _, err := core.ClaimCard(ctx, prevDesktop, blocked.ID, kanban.ClaimCardInput{EngineerID: board.Engineers[0].ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := core.AddComment(ctx, prevDesktop, blocked.ID, "Another agent comment should not unblock."); err != nil {
		t.Fatal(err)
	}
	if err := core.ReleaseCard(ctx, prevDesktop, blocked.ID); err != nil {
		t.Fatal(err)
	}
	time.Sleep(40 * time.Millisecond)
	if got := calls.Load(); got != 4 {
		t.Fatalf("agent comment unexpectedly triggered work: %d calls", got)
	}
	if _, err := core.AddComment(ctx, human, blocked.ID, retryCommandPrefix+previousCode); err != nil {
		t.Fatal(err)
	}
	waitDesktopCheck(t, func() bool {
		got, err := core.GetCardDetail(ctx, blocked.ID)
		return err == nil && got.ColumnID == flow.inbox && !got.ClaimActive
	})
}

func waitDesktopCheck(t *testing.T, check func() bool) {
	t.Helper()
	timeout := time.After(8 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if check() {
			return
		}
		select {
		case <-timeout:
			t.Fatal("desktop dispatch did not reach the expected state")
		case <-ticker.C:
		}
	}
}

func TestDesktopDispatchBackoffAndCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	var attempts int
	run := dispatchRun{
		host: func(context.Context, string, string, string, bool, dispatchSession) (dispatchReply, error) {
			attempts++
			return dispatchReply{}, errProviderBackoff
		},
		update: func(_ int64, status engineerStatus) {
			if status.State != "unstable" {
				t.Fatalf("wrong status: %v", status)
			}
			if attempts == 2 {
				if time.Until(status.Retry) < 4*time.Minute {
					t.Fatalf("missing five-minute retry: %v", status)
				}
				cancel()
			}
		},
	}
	_, _, err := run.hostWithBackoff(ctx, dispatchJob{}, t.TempDir(), "", true, dispatchSession{}, nil)
	if attempts != 2 || !errors.Is(err, context.Canceled) {
		t.Fatalf("wrong retry behavior: %d %v", attempts, err)
	}
}

func TestDesktopDispatchBridgeRejectsCrossOrigin(t *testing.T) {
	d := &desktopDispatch{path: "/.slate-desktop/test"}
	handler := d.wrap("http://localhost:7070", http.NotFoundHandler())
	req := httptest.NewRequest(http.MethodPut, d.path, strings.NewReader(`{"enabled":true}`))
	req.Header.Set("Origin", "https://external.example")
	req.Header.Set("X-Slate-Desktop", "dispatch")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("cross-origin request returned %d", res.Code)
	}
}

func TestDesktopDispatchQueueSaturationUsesCurrentCards(t *testing.T) {
	const count = dispatchQueueSize + 2
	snapshot := &kanban.Snapshot{BoardDetail: &kanban.BoardDetail{Board: &kanban.Board{}}}
	flow := dispatchFlow{inbox: 1, todo: 2, dev: 3, review: 4, done: 5}
	hosts := make(map[int64]string)
	for i := range count {
		id := int64(i + 1)
		hosts[id] = "codex"
		snapshot.Engineers = append(snapshot.Engineers, &kanban.Engineer{ID: id, Kind: kanban.EngineerDev})
		snapshot.Cards = append(snapshot.Cards, &kanban.CardView{Card: &kanban.Card{ID: id, ColumnID: flow.dev, EngineerID: id}})
	}
	d := dispatchRun{config: dispatchConfig{Hosts: hosts}, dev: make(chan dispatchJob, dispatchQueueSize), review: make(chan dispatchJob, dispatchQueueSize)}
	active := make(map[int64]int64)
	d.schedule(snapshot, flow, active, nil)
	if len(d.dev) != dispatchQueueSize || len(active) != dispatchQueueSize {
		t.Fatalf("unexpected reservation: %d queued, %d active", len(d.dev), len(active))
	}

	finished := <-d.dev
	delete(active, finished.engineer.ID)
	snapshot.Cards[0].ColumnID = flow.done
	snapshot.Cards[dispatchQueueSize].HumanOnly = true // moved to manual while the queue was full
	d.schedule(snapshot, flow, active, nil)
	if active[count] != count || active[dispatchQueueSize+1] != 0 {
		t.Fatalf("saturation did not reconcile the current queue: %v", active)
	}
}

func TestDesktopDispatchHostAdaptersWithExecutables(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable fixtures are POSIX shell scripts")
	}
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+"/usr/bin"+string(os.PathListSeparator)+"/bin")
	reply := `{"outcome":"plan","summary":"Fixture","items":""}`
	fixtures := map[string]string{
		"codex": `#!/bin/sh
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--output-last-message" ]; then
    printf '%s' '` + reply + `' > "$2"
    exit 0
  fi
  shift
done
exit 1
`,
		"claude": `#!/bin/sh
printf '%s' '{"is_error":false,"result":"{\"outcome\":\"plan\",\"summary\":\"Fixture\",\"items\":\"\"}"}'
`,
		"pi": `#!/bin/sh
for arg in "$@"; do
  case "$arg" in
    @*) test -f "${arg#@}" || exit 1; printf '%s' '` + reply + `'; exit 0 ;;
  esac
done
exit 1
`,
	}
	fixtures["omp"] = fixtures["pi"]
	for name, content := range fixtures {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, host := range []string{"codex", "claude", "pi", "on-my-pi"} {
		t.Run(host, func(t *testing.T) {
			output, err := localDispatchHost(t.Context(), host, dir, "test prompt", true, dispatchSession{})
			if err != nil || output.Outcome != "plan" || output.Summary != "Fixture" {
				t.Fatalf("invalid %s receipt: %#v (%v)", host, output, err)
			}
		})
	}
}

func TestDesktopDispatchResumesOnlySameTaskAndStage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable fixtures are POSIX shell scripts")
	}
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+"/usr/bin"+string(os.PathListSeparator)+"/bin")
	// Emulate the actual CLI formats and require the second invocation to
	// address the first invocation's persisted thread or scoped session.
	const thread = "8f15b56c-cdc3-45e5-8feb-e3b8dd62fd33"
	codex := `#!/bin/sh
resume=no
last=''
readonly=no
while [ "$#" -gt 0 ]; do
  case "$1" in
    resume) resume=yes ;;
    'sandbox_mode="read-only"') readonly=yes ;;
    --output-last-message) last="$2"; shift ;;
    ` + thread + `) test "$resume" = yes || exit 2 ;;
  esac
  shift
done
test "$readonly" = yes || exit 2
printf '%s\n' '{"type":"thread.started","thread_id":"` + thread + `"}'
printf '{"outcome":"ready","summary":"%s","items":""}' "$resume" > "$last"
`
	claude := `#!/bin/sh
resume=no
id=''
mode=''
while [ "$#" -gt 0 ]; do
  case "$1" in
    --resume) resume=yes; id="$2"; shift ;;
    --session-id) id="$2"; shift ;;
    --permission-mode) mode="$2"; shift ;;
    --name) test "$2" = "DISP-42 · review" || exit 2; shift ;;
  esac
  shift
done
test -n "$id" && test "$mode" = plan || exit 2
printf '{"is_error":false,"session_id":"%s","result":"{\\"outcome\\":\\"ready\\",\\"summary\\":\\"%s\\",\\"items\\":\\"\\"}"}' "$id" "$resume"
`
	pi := `#!/bin/sh
resume=no
scope=''
tools=''
while [ "$#" -gt 0 ]; do
  case "$1" in
    --continue) resume=yes ;;
    --session-dir) scope="$2"; shift ;;
    --tools) tools="$2"; shift ;;
    --name) test "$2" = "DISP-42 · review" || exit 2; shift ;;
  esac
  shift
done
test -d "$scope" && test "$tools" = read,grep,find,ls || exit 2
if [ "$resume" = yes ]; then
  test -f "$scope/prior" || exit 2
else
  test ! -f "$scope/prior" || exit 2
  touch "$scope/prior"
fi
printf '{"outcome":"ready","summary":"%s","items":""}' "$resume"
`
	for name, content := range map[string]string{"codex": codex, "claude": claude, "pi": pi, "omp": pi} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, host := range []string{"codex", "claude", "pi", "on-my-pi"} {
		t.Run(host, func(t *testing.T) {
			job := dispatchJob{host: host, project: dir, review: true}
			root := filepath.Join(dir, "sessions")
			run := func(want string) {
				session, err := taskDispatchSession(root, job, "DISP-42")
				if err != nil {
					t.Fatal(err)
				}
				reply, err := localDispatchHost(t.Context(), host, dir, "review", true, session)
				if err != nil || reply.Summary != want {
					t.Fatalf("expected resume=%s, got %#v (%v)", want, reply, err)
				}
			}
			run("no")
			run("yes")
			review, err := taskDispatchSession(root, job, "DISP-42")
			if err != nil {
				t.Fatal(err)
			}
			job.review = false
			dev, err := taskDispatchSession(root, job, "DISP-42")
			if err != nil {
				t.Fatal(err)
			}
			if dev.dir == review.dir || dev.name == review.name {
				t.Fatal("review would resume a development session")
			}
		})
	}
}

func TestDesktopDispatchRechecksHumanCommentAfterPlanning(t *testing.T) {
	ctx := t.Context()
	core, human, board, flow := newDispatchTestBoard(t)
	card, err := core.CreateCard(ctx, human, board.ID, kanban.CreateCardInput{Title: "Plan change", ColumnID: flow.dev})
	if err != nil {
		t.Fatal(err)
	}
	const latestComment = "Account for the new requirement."
	watched := &commentObservedCore{dispatchCore: core, cardID: card.ID, seen: make(chan struct{})}
	seen := watched.seen
	var count atomic.Int32
	host := func(ctx context.Context, _, _, prompt string, readOnly bool, _ dispatchSession) (dispatchReply, error) {
		if !readOnly {
			t.Error("a stale plan reached execution")
			return dispatchReply{}, errUnconfirmedRun
		}
		if count.Add(1) > 1 {
			if !strings.Contains(prompt, latestComment) {
				t.Error("replanning omitted the new human comment")
			}
			return dispatchReply{Outcome: "clarify", Summary: "Updated clarification"}, nil
		}
		if _, err := core.AddComment(ctx, human, card.ID, latestComment); err != nil {
			return dispatchReply{}, err
		}
		select {
		case <-seen:
		case <-ctx.Done():
			return dispatchReply{}, ctx.Err()
		}
		return dispatchReply{Outcome: "plan", Summary: "Stale plan"}, nil
	}
	startDispatchFixture(t, watched, board, host)
	waitDesktopCheck(t, func() bool {
		current, err := core.GetCardDetail(ctx, card.ID)
		return err == nil && current.ColumnID == flow.inbox && !current.ClaimActive
	})
	current, err := core.GetCardDetail(ctx, card.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, comment := range current.Comments {
		if strings.Contains(comment.Body, "Stale plan") {
			t.Fatal("stale plan was published")
		}
	}
	if count.Load() != 2 {
		t.Fatalf("expected one replan, got %d host calls", count.Load())
	}
}

func TestDesktopDispatchRetainsUnconfirmedExecutionAfterHumanComment(t *testing.T) {
	for _, tc := range []struct {
		name string
		stop bool
	}{{"host error", false}, {"graceful stop", true}} {
		t.Run(tc.name, func(t *testing.T) {
			verifyUnconfirmedDispatch(t, tc.stop)
		})
	}
}

func verifyUnconfirmedDispatch(t *testing.T, stopAfterComment bool) {
	t.Helper()
	ctx := t.Context()
	core, human, board, flow := newDispatchTestBoard(t)
	card, err := core.CreateCard(ctx, human, board.ID, kanban.CreateCardInput{Title: "Uncertain implementation", ColumnID: flow.dev})
	if err != nil {
		t.Fatal(err)
	}
	watched := &commentObservedCore{dispatchCore: core, cardID: card.ID, seen: make(chan struct{})}
	seen := watched.seen
	reached := make(chan struct{})
	var count atomic.Int32
	host := func(ctx context.Context, _, _, _ string, readOnly bool, _ dispatchSession) (dispatchReply, error) {
		attempt := count.Add(1)
		if attempt == 1 && readOnly {
			return dispatchReply{Outcome: "plan", Summary: "Prepared plan"}, nil
		}
		if attempt != 2 || readOnly {
			t.Error("unconfirmed execution repeated")
			return dispatchReply{}, errUnconfirmedRun
		}
		if _, err := core.AddComment(ctx, human, card.ID, "Follow-up while the host is executing."); err != nil {
			return dispatchReply{}, err
		}
		select {
		case <-seen:
		case <-ctx.Done():
			return dispatchReply{}, ctx.Err()
		}
		close(reached)
		if stopAfterComment {
			<-ctx.Done()
			return dispatchReply{}, ctx.Err()
		}
		return dispatchReply{}, errUnconfirmedRun
	}
	stop := startDispatchFixture(t, watched, board, host)
	if stopAfterComment {
		select {
		case <-reached:
		case <-time.After(3 * time.Second):
			t.Fatal("host did not read the follow-up event")
		}
		stop()
	}
	waitDesktopCheck(t, func() bool {
		current, err := core.GetCardDetail(ctx, card.ID)
		return err == nil && len(current.Comments) >= 4 && !current.ClaimActive && desktopDispatchRecovery(current, false) == dispatchAwaitingConfirmation
	})
	if count.Load() != 2 {
		t.Fatalf("expected one plan and one execution, got %d host calls", count.Load())
	}
}

func startDispatchFixture(t *testing.T, core dispatchCore, board *kanban.BoardDetail, host dispatchHost) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	snapshot, err := core.BoardSnapshot(ctx, board.ID)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	done := make(chan struct{})
	dispatch := dispatchRun{
		core: core, config: dispatchConfig{BoardID: board.ID, Hosts: map[int64]string{board.Engineers[0].ID: "codex"}},
		actor: desktopDispatchActor(), host: host, update: func(int64, engineerStatus) {},
	}
	go func() {
		defer close(done)
		dispatch.run(ctx, snapshot)
	}()
	stop := func() { cancel(); <-done }
	t.Cleanup(stop)
	return stop
}

// The fixture holds the host until the scheduler has refreshed its snapshot
// after the human comment. This drives the event-before-worker race directly.
type commentObservedCore struct {
	dispatchCore
	cardID int64
	seen   chan struct{}
	dirty  bool
}

func (c *commentObservedCore) ListEvents(ctx context.Context, boardID, seq int64, limit int) (*kanban.EventPage, error) {
	page, err := c.dispatchCore.ListEvents(ctx, boardID, seq, limit)
	if err == nil && c.seen != nil {
		for _, event := range page.Events {
			if event.CardID == c.cardID && event.Type == kanban.EventCommentAdded && event.ActorKind == string(kanban.ActorHuman) {
				c.dirty = true
			}
		}
	}
	return page, err
}

func (c *commentObservedCore) BoardSnapshot(ctx context.Context, boardID int64) (*kanban.Snapshot, error) {
	snapshot, err := c.dispatchCore.BoardSnapshot(ctx, boardID)
	if err == nil && c.seen != nil && c.dirty {
		close(c.seen)
		c.seen = nil
	}
	return snapshot, err
}

func TestDesktopDispatchKeepsCrashedExecutionBlocked(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		name := "explicit start marker"
		if legacy {
			name = "legacy marker"
		}
		t.Run(name, func(t *testing.T) { verifyCrashRecovery(t, legacy) })
	}
}

func verifyCrashRecovery(t *testing.T, legacy bool) {
	t.Helper()
	ctx := t.Context()
	now := time.Now()
	core, human, board, flow := newDispatchTestBoard(t, kanban.WithClock(func() time.Time { return now }))
	card, err := core.CreateCard(ctx, human, board.ID, kanban.CreateCardInput{Title: "Crash recovery", ColumnID: flow.dev})
	if err != nil {
		t.Fatal(err)
	}
	previous := kanban.Actor{Kind: kanban.ActorAgent, Name: "desktop-prior"}
	job := dispatchJob{id: card.ID, engineer: *board.Engineers[0]}
	if _, err := core.ClaimCard(ctx, previous, card.ID, kanban.ClaimCardInput{EngineerID: job.engineer.ID, LeaseMinutes: 30}); err != nil {
		t.Fatal(err)
	}
	marker := job.startedMarker()
	if legacy {
		marker = pendingDevelopment
	}
	if _, err := core.AddComment(ctx, previous, card.ID, "Desktop dispatch started.\n\n"+marker); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	if _, err := core.AddComment(ctx, human, card.ID, "Ordinary details while the host was working."); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Hour) // The old worker never ran its cleanup.
	var calls atomic.Int32
	host := func(_ context.Context, _, _, _ string, readOnly bool, _ dispatchSession) (dispatchReply, error) {
		calls.Add(1)
		if readOnly {
			return dispatchReply{Outcome: "plan", Summary: "Authorized recovery"}, nil
		}
		return dispatchReply{}, errUnconfirmedRun
	}
	startDispatchFixture(t, core, board, host)
	code := waitForRecoveryCode(t, core, job, "")
	for _, candidate := range []struct {
		actor kanban.Actor
		text  string
	}{
		{human, "Ordinary details after the crash."},
		{human, retryCommandPrefix + "old-code"},
		{previous, retryCommandPrefix + code},
	} {
		if _, err := core.AddComment(ctx, candidate.actor, card.ID, candidate.text); err != nil {
			t.Fatal(err)
		}
	}
	current, err := core.GetCardDetail(ctx, card.ID)
	if err != nil || desktopDispatchRecovery(current, false) != dispatchAwaitingConfirmation {
		t.Fatalf("non-confirming comments removed the block: %v", err)
	}
	if calls.Load() != 0 {
		t.Fatal("host started before a valid human confirmation")
	}
	if _, err := core.AddComment(ctx, human, card.ID, retryCommandPrefix+code); err != nil {
		t.Fatal(err)
	}
	if next := waitForRecoveryCode(t, core, job, code); next == code || calls.Load() != 2 {
		t.Fatalf("expected a single retry with a fresh recovery code, got %d host calls", calls.Load())
	}
	// Earlier authorizations do not carry over after the new attempt fails.
	if _, err := core.AddComment(ctx, human, card.ID, retryCommandPrefix+code); err != nil {
		t.Fatal(err)
	}
	current, err = core.GetCardDetail(ctx, card.ID)
	if err != nil || desktopDispatchRecovery(current, false) != dispatchAwaitingConfirmation {
		t.Fatalf("stale authorization unlocked a newer attempt: %v", err)
	}
}

func waitForRecoveryCode(t *testing.T, core kanban.Core, job dispatchJob, previous string) string {
	t.Helper()
	var code string
	waitDesktopCheck(t, func() bool {
		card, err := core.GetCardDetail(t.Context(), job.id)
		if err != nil || card.ClaimActive || desktopDispatchRecovery(card, job.review) != dispatchAwaitingConfirmation {
			return false
		}
		for i := len(card.Comments) - 1; i >= 0; i-- {
			if _, last, _ := strings.CutLast(card.Comments[i].Body, "\n"); last != "" {
				if code = job.retryCode(last); code != "" {
					return code != previous
				}
			}
		}
		return false
	})
	return code
}

func TestDesktopDispatchProviderRefusalDoesNotRequireRecovery(t *testing.T) {
	for _, dirty := range []bool{false, true} {
		name := "unchanged worktree"
		if dirty {
			name = "changed worktree"
		}
		t.Run(name, func(t *testing.T) { verifyProviderRefusal(t, dirty) })
	}
}

func verifyProviderRefusal(t *testing.T, dirty bool) {
	t.Helper()
	core, human, board, flow := newDispatchTestBoard(t)
	card, err := core.CreateCard(t.Context(), human, board.ID, kanban.CreateCardInput{Title: "Provider limit", ColumnID: flow.dev})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var attempts int
	run := dispatchRun{
		core: core, actor: desktopDispatchActor(),
		host: func(_ context.Context, _, dir, _ string, readOnly bool, _ dispatchSession) (dispatchReply, error) {
			if readOnly {
				return dispatchReply{Outcome: "plan", Summary: "Plan"}, nil
			}
			attempts++
			if dirty {
				if err := os.WriteFile(filepath.Join(dir, "partial.txt"), []byte("partial"), 0o600); err != nil {
					return dispatchReply{}, err
				}
			}
			return dispatchReply{}, errProviderBackoff
		},
		update: func(_ int64, status engineerStatus) {
			if status.State == "unstable" && attempts == 2 {
				if time.Until(status.Retry) < 4*time.Minute {
					t.Errorf("missing formal retry delay: %v", status)
				}
				cancel()
			}
		},
	}
	job := dispatchJob{id: card.ID, engineer: *board.Engineers[0], host: "codex", flow: flow, project: board.ProjectPath}
	err = run.handle(ctx, job)
	wantAttempts, wantState, wantErr := 2, dispatchAvailable, error(context.Canceled)
	if dirty {
		wantAttempts, wantState, wantErr = 1, dispatchAwaitingConfirmation, errUnconfirmedRun
	}
	if attempts != wantAttempts || !errors.Is(err, wantErr) {
		t.Fatalf("attempts=%d err=%v", attempts, err)
	}
	got, err := core.GetCardDetail(t.Context(), card.ID)
	if err != nil || got.ClaimActive || desktopDispatchRecovery(got, false) != wantState {
		t.Fatalf("incorrect state after provider refusal: %v", err)
	}
}
