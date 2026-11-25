package desktop

import (
	"context"
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
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
			if readOnly {
				t.Error("formal review host cannot build or test in its disposable checkout")
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
	dispatch := dispatchRun{core: core, config: dispatchConfig{BoardID: board.ID, Hosts: hosts}, host: host, update: func(int64, engineerStatus) {}}
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
		// A passed review completes the card; the review evidence stays as a comment.
		return err == nil && got.ColumnID == flow.done && !got.ClaimActive &&
			strings.Contains(got.Description, "- [x] Implementation") && lastDesktopMarker(got) == completedReview
	})
	if got := calls.Load(); got != 4 {
		t.Fatalf("wanted the development and review phases only, got %d host calls", got)
	}
	finished, err := core.GetCardDetail(ctx, card.ID)
	if err != nil {
		t.Fatal(err)
	}
	visible, markers := 0, 0
	for _, c := range finished.Comments {
		if strings.Contains(c.Body, "Desktop") || strings.Contains(c.Body, "slate-desktop:") {
			t.Errorf("desktop comment carries auxiliary text: %q", c.Body)
		}
		if c.Body != "" {
			visible++
		}
		if commentMarker(c) != "" && c.Ext != "" {
			markers++
		}
	}
	if visible < 3 || markers < 4 {
		t.Errorf("expected plan, handoff and review text plus start and handoff and review markers, got %d visible comments, %d markers", visible, markers)
	}
	claims, err := core.ListActivity(ctx, board.ID, kanban.ActivityQuery{
		ActivityFilter: kanban.ActivityFilter{Types: []string{kanban.EventCardClaimed}},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantNames := map[int64]string{
		board.Engineers[0].ID: board.Engineers[0].Name,
		board.Engineers[2].ID: board.Engineers[2].Name,
	}
	desktopEngineers := make(map[int64]int)
	ordinaryClaim := false
	for _, event := range claims.Events {
		var payload struct {
			EngineerID   int64   `json:"engineer_id"`
			EngineerName *string `json:"engineer_name"`
		}
		if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
			t.Fatal(err)
		}
		switch event.CardID {
		case card.ID:
			want, known := wantNames[payload.EngineerID]
			if event.Actor != want || !known || payload.EngineerName == nil || *payload.EngineerName != want {
				t.Errorf("desktop claim lost the actor or engineer display name: %+v", event)
			}
			desktopEngineers[payload.EngineerID]++
		case competing.ID:
			if event.Actor != other.Name || payload.EngineerName == nil || *payload.EngineerName != "" {
				t.Errorf("ordinary claim should keep the actor display name: %+v", event)
			}
			ordinaryClaim = true
		}
	}
	for id, name := range wantNames {
		if got := desktopEngineers[id]; got != 1 {
			t.Errorf("expected exactly one claim by %s for its stage, got %d", name, got)
		}
	}
	if len(desktopEngineers) != len(wantNames) || !ordinaryClaim {
		t.Fatalf("missing claim evidence: desktop engineers %v, ordinary claim %v", desktopEngineers, ordinaryClaim)
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

func TestDesktopDispatchClaimDoesNotStartTwiceOrReacquireOnHeartbeat(t *testing.T) {
	ctx := t.Context()
	now := time.Now()
	core, human, board, flow := newDispatchTestBoard(t, kanban.WithClock(func() time.Time { return now }))
	card, err := core.CreateCard(ctx, human, board.ID, kanban.CreateCardInput{Title: "Lease", ColumnID: flow.dev})
	if err != nil {
		t.Fatal(err)
	}
	job := dispatchJob{engineer: *board.Engineers[0]}
	actor := job.actor()
	claim := kanban.ClaimCardInput{EngineerID: job.engineer.ID, LeaseMinutes: 5, DesktopDispatch: true}
	first, err := core.ClaimCard(ctx, actor, card.ID, claim)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := core.ClaimCard(ctx, actor, card.ID, claim); !errors.Is(err, kanban.ErrCardClaimed) {
		t.Fatalf("second dispatch by the same engineer should be rejected: %v", err)
	}

	renew := kanban.ClaimCardInput{EngineerID: job.engineer.ID, LeaseMinutes: 10, RenewOnly: true}
	latest, err := core.ClaimCard(ctx, actor, card.ID, renew)
	if err != nil {
		t.Fatal(err)
	}
	if latest.Version != first.Version || !latest.ClaimExpiresAt.After(*first.ClaimExpiresAt) {
		t.Fatalf("heartbeat should only extend the lease: %+v", latest)
	}

	now = *latest.ClaimExpiresAt
	if _, err := core.ClaimCard(ctx, actor, card.ID, renew); !errors.Is(err, kanban.ErrNotHolder) {
		t.Fatalf("heartbeat should not reacquire an expired lease: %v", err)
	}
	activity, err := core.ListActivity(ctx, board.ID, kanban.ActivityQuery{
		ActivityFilter: kanban.ActivityFilter{CardID: card.ID, Types: []string{kanban.EventCardClaimed}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(activity.Events); got != 1 {
		t.Fatalf("expected one claim event, got %d", got)
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

// 可写的运行要在沙箱里构建和测试，而 Go 默认的构建缓存在用户目录下，沙箱不让写，go test 会直接失败；
// 所以可写运行拿到一个放在系统临时目录里的私有 GOCACHE（运行时存在且可写，结束后删除）；只读运行不改环境。
func TestLocalDispatchHostGivesWritableRunsAPrivateGoCache(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable fixtures are POSIX shell scripts")
	}
	bin, checkout := t.TempDir(), t.TempDir()
	t.Setenv("HOME", bin)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+"/usr/bin"+string(os.PathListSeparator)+"/bin")
	const userCache = "/user/own/go-cache"
	t.Setenv("GOCACHE", userCache)
	script := `#!/bin/sh
state=missing
if [ -d "$GOCACHE" ]; then
  state=dir
  touch "$GOCACHE/probe" || exit 1
fi
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--output-last-message" ]; then
    printf '{"outcome":"plan","summary":"%s %s","items":""}' "$GOCACHE" "$state" > "$2"
    exit 0
  fi
  shift
done
exit 1
`
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	run := func(readOnly bool) (cache, state string) {
		t.Helper()
		reply, err := localDispatchHost(t.Context(), "codex", checkout, "prompt", readOnly, dispatchSession{})
		if err != nil {
			t.Fatal(err)
		}
		cache, state, _ = strings.Cut(reply.Summary, " ")
		return cache, state
	}
	if cache, state := run(true); cache != userCache || state != "missing" {
		t.Fatalf("a read-only run must keep the caller's environment, got %q %q", cache, state)
	}
	cache, state := run(false)
	if state != "dir" || cache == userCache || strings.HasPrefix(cache, checkout) {
		t.Fatalf("a writable run needs an existing private cache outside the checkout, got %q %q", cache, state)
	}
	if _, err := os.Stat(cache); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the private cache was not removed after the run: %v", err)
	}
}

func TestDesktopDispatchFindsVersionedHomebrewNode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable fixtures are POSIX shell scripts")
	}
	core, human, board, flow := newDispatchTestBoard(t)
	card, err := core.CreateCard(t.Context(), human, board.ID, kanban.CreateCardInput{Title: "Implement", ColumnID: flow.dev})
	if err != nil {
		t.Fatal(err)
	}
	prefix := t.TempDir()
	bin := filepath.Join(prefix, "bin")
	nodeBin := filepath.Join(prefix, "opt", "node@22", "bin")
	for _, dir := range []string{bin, nodeBin} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	launcher := `#!/bin/sh
case ":$PATH:" in
  *":$SLATE_FIXTURE_NODE_DIR:"*) exec "$SLATE_FIXTURE_NODE_DIR/node" "$@" ;;
esac
exit 127
`
	launcherRuntime := `#!/bin/sh
case "$*" in
  *read-only*) reply='{"outcome":"plan","summary":"Setup plan","items":""}' ;;
  *)
    printf '%s' 'dispatch integration' > feature.txt
    git add feature.txt && git -c user.name=Test -c user.email=test@example.local commit -m Implement || exit 1
    reply='{"outcome":"ready","summary":"Setup complete","items":"- [x] Implementation"}' ;;
esac
while [ "$#" -gt 1 ]; do
  if [ "$1" = '--output-last-message' ]; then
    printf '%s' "$reply" > "$2"
    exit 0
  fi
  shift
done
exit 1
`
	for file, script := range map[string]string{filepath.Join(bin, "codex"): launcher, filepath.Join(nodeBin, "node"): launcherRuntime} {
		if err := os.WriteFile(file, []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", strings.Join([]string{bin, "/usr/bin", "/bin"}, string(os.PathListSeparator)))
	t.Setenv("SLATE_FIXTURE_NODE_DIR", nodeBin)
	run := dispatchRun{core: core, host: localDispatchHost}
	job := dispatchJob{id: card.ID, engineer: *board.Engineers[0], host: "codex", flow: flow, project: board.ProjectPath}
	if err := run.handle(t.Context(), job); err != nil {
		t.Fatalf("restricted PATH dispatch failed: %v", err)
	}
	got, err := core.GetCardDetail(t.Context(), card.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ColumnID != flow.review || got.ClaimActive || got.Branch == "" || !strings.Contains(got.Description, "- [x] Implementation") {
		t.Fatalf("expected an unclaimed task with a branch and task items in review: %+v", got.Card)
	}
	if commit, err := gitOutput(t.Context(), board.ProjectPath, "show", got.Branch+":feature.txt"); err != nil || commit != "dispatch integration" {
		t.Fatalf("branch content %q, error %v", commit, err)
	}
}

func TestDesktopDispatchExplainsPreparationFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable fixtures are POSIX shell scripts")
	}
	for _, tt := range []struct {
		name, script, want string
		worktree           bool
	}{
		{
			name: "missing-node", want: "找不到 Node.js",
			script: `printf '%s\n' 'env: node: No such file or directory' 'private provider diagnostic' >&2; exit 127`,
		},
		{
			name: "invalid-plan", want: "未生成可用的任务方案",
			script: `printf '%s\n' 'private provider diagnostic' >&2; exit 1`,
		},
		{
			name: "worktree", want: "无法准备任务工作树", worktree: true,
			script: `while [ "$#" -gt 1 ]; do
  if [ "$1" = '--output-last-message' ]; then
    printf '%s' '{"outcome":"plan","summary":"Plan accepted","items":""}' > "$2"
    exit 0
  fi
  shift
done
exit 1`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			core, human, board, flow := newDispatchTestBoard(t)
			card, err := core.CreateCard(t.Context(), human, board.ID, kanban.CreateCardInput{Title: "Implement", ColumnID: flow.dev})
			if err != nil {
				t.Fatal(err)
			}
			if tt.worktree {
				dir, err := taskWorktreePath(board.ProjectPath, card.Key)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(dir, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			bin := t.TempDir()
			t.Setenv("PATH", strings.Join([]string{bin, "/usr/bin", "/bin"}, string(os.PathListSeparator)))
			if err := os.WriteFile(filepath.Join(bin, "codex"), []byte("#!/bin/sh\n"+tt.script+"\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			run := dispatchRun{core: core, host: localDispatchHost, text: func() desktopText { return desktopTexts["zh-CN"] }}
			job := dispatchJob{id: card.ID, engineer: *board.Engineers[0], host: "codex", flow: flow, project: board.ProjectPath}
			if err := run.handle(t.Context(), job); err == nil {
				t.Fatal("expected preparation failure")
			}
			got, err := core.GetCardDetail(t.Context(), card.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.ClaimActive || got.ColumnID != flow.dev || got.Branch != "" {
				t.Fatalf("expected unclaimed, unchanged development task: %+v", got.Card)
			}
			if len(got.Comments) == 0 {
				t.Fatal("expected a preparation failure comment")
			}
			comment := got.Comments[len(got.Comments)-1].Body
			if !strings.Contains(comment, tt.want) || strings.Contains(comment, "private provider diagnostic") {
				t.Fatalf("unexpected failure comment: %q", comment)
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
		host: host, update: func(int64, engineerStatus) {},
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
			if code = job.retryCode(commentMarker(card.Comments[i])); code != "" {
				return code != previous
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
		core: core,
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

// 宿主改完文件但沙箱不允许它提交时，桌面端要代为提交；没有任何交付则仍视为失败。
func TestDispatchCommitCommitsPendingChanges(t *testing.T) {
	ctx := t.Context()
	repo := t.TempDir()
	t.Setenv("HOME", repo)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, args := range [][]string{{"init", "-b", "task"}, {"-c", "user.name=T", "-c", "user.email=t@e.l", "commit", "--allow-empty", "-m", "Initial"}} {
		if _, err := gitOutput(ctx, repo, args...); err != nil {
			t.Fatal(err)
		}
	}
	before, err := gitOutput(ctx, repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dispatchCommit(ctx, repo, "task", before, "SLATE-1 nothing"); err == nil {
		t.Fatal("clean tree without a new commit was accepted")
	}
	if _, err := dispatchCommit(ctx, repo, "other", before, "SLATE-1 wrong branch"); err == nil {
		t.Fatal("wrong branch was accepted")
	}
	if err := os.WriteFile(filepath.Join(repo, "feature.txt"), []byte("done\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	head, err := dispatchCommit(ctx, repo, "task", before, "SLATE-1 feature")
	if err != nil || head == before {
		t.Fatalf("pending change was not committed: head=%q err=%v", head, err)
	}
	if status, _ := gitOutput(ctx, repo, "status", "--porcelain"); status != "" {
		t.Fatalf("worktree still dirty: %q", status)
	}
	if subject, _ := gitOutput(ctx, repo, "log", "-1", "--format=%s"); subject != "SLATE-1 feature" {
		t.Fatalf("unexpected subject %q", subject)
	}
}

// 宿主声明任务不需要改仓库时，分支、提交和工作树都原样不动才算数；任何一项变了都不能当作无改动交付。
func TestDispatchUnchanged(t *testing.T) {
	ctx := t.Context()
	git := func(t *testing.T, repo string, args ...string) {
		t.Helper()
		if _, err := gitOutput(ctx, repo, append([]string{"-c", "user.name=T", "-c", "user.email=t@e.l"}, args...)...); err != nil {
			t.Fatal(err)
		}
	}
	write := func(t *testing.T, repo, name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for name, tt := range map[string]struct {
		change  func(t *testing.T, repo string)
		wantErr bool
	}{
		"原样不动":    {func(*testing.T, string) {}, false},
		"新增未跟踪文件": {func(t *testing.T, repo string) { write(t, repo, "new.txt", "new\n") }, true},
		"修改已跟踪文件": {func(t *testing.T, repo string) { write(t, repo, "base.txt", "changed\n") }, true},
		"多出一个提交":  {func(t *testing.T, repo string) { git(t, repo, "commit", "--allow-empty", "-m", "Extra") }, true},
		"换到别的分支":  {func(t *testing.T, repo string) { git(t, repo, "switch", "-c", "other") }, true},
	} {
		t.Run(name, func(t *testing.T) {
			repo := t.TempDir()
			t.Setenv("HOME", repo)
			t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			git(t, repo, "init", "-b", "task")
			write(t, repo, "base.txt", "base\n")
			git(t, repo, "add", "base.txt")
			git(t, repo, "commit", "-m", "Initial")
			before, err := gitOutput(ctx, repo, "rev-parse", "HEAD")
			if err != nil {
				t.Fatal(err)
			}
			tt.change(t, repo)
			if err := dispatchUnchanged(ctx, repo, "task", before); (err != nil) != tt.wantErr {
				t.Fatalf("wantErr=%v got %v", tt.wantErr, err)
			}
		})
	}
}

// 审查副本是游离 HEAD 的检出：提交没变且工作树干净才算原样不动，不要求停在某个分支上；
// 被 .gitignore 忽略的文件不算改动，其余任何新增、修改和提交都不行。
func TestCheckoutUnchanged(t *testing.T) {
	ctx := t.Context()
	git := func(t *testing.T, repo string, args ...string) {
		t.Helper()
		if _, err := gitOutput(ctx, repo, append([]string{"-c", "user.name=T", "-c", "user.email=t@e.l"}, args...)...); err != nil {
			t.Fatal(err)
		}
	}
	write := func(t *testing.T, repo, name, content string) {
		t.Helper()
		path := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for name, tt := range map[string]struct {
		change  func(t *testing.T, repo string)
		wantErr bool
	}{
		"原样不动":       {func(*testing.T, string) {}, false},
		"只多出被忽略的文件":  {func(t *testing.T, repo string) { write(t, repo, "build.log", "ignored\n") }, false},
		"新增未跟踪文件":    {func(t *testing.T, repo string) { write(t, repo, "new.txt", "new\n") }, true},
		"未跟踪文件在新目录里": {func(t *testing.T, repo string) { write(t, repo, "cache/new.txt", "new\n") }, true},
		"修改已跟踪文件":    {func(t *testing.T, repo string) { write(t, repo, "base.txt", "changed\n") }, true},
		"多出一个提交":     {func(t *testing.T, repo string) { git(t, repo, "commit", "--allow-empty", "-m", "Extra") }, true},
	} {
		t.Run(name, func(t *testing.T) {
			repo := t.TempDir()
			t.Setenv("HOME", repo)
			t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			git(t, repo, "init", "-b", "task")
			write(t, repo, "base.txt", "base\n")
			write(t, repo, ".gitignore", "*.log\n")
			git(t, repo, "add", "base.txt", ".gitignore")
			git(t, repo, "commit", "-m", "Initial")
			git(t, repo, "checkout", "--detach")
			before, err := gitOutput(ctx, repo, "rev-parse", "HEAD")
			if err != nil {
				t.Fatal(err)
			}
			tt.change(t, repo)
			if err := checkoutUnchanged(ctx, repo, before); (err != nil) != tt.wantErr {
				t.Fatalf("wantErr=%v got %v", tt.wantErr, err)
			}
		})
	}
}

// 提交说明只取标题首行，并限制在固定长度内。
func TestDispatchCommitMessage(t *testing.T) {
	if got := dispatchCommitMessage("SLATE-9", "标题\n第二行"); got != "SLATE-9 标题" {
		t.Fatalf("unexpected message %q", got)
	}
	if got := dispatchCommitMessage("SLATE-9", strings.Repeat("长", 200)); len([]rune(got)) != maxCommitSubjectRunes {
		t.Fatalf("message not truncated: %d runes", len([]rune(got)))
	}
}

// 保存被拒绝时，桥接接口要返回稳定的原因标识，前端据此显示具体提示。
func TestDesktopDispatchBridgeReportsRejectionCode(t *testing.T) {
	ctx := t.Context()
	core, human, board, _ := newDispatchTestBoard(t)
	noPath, err := core.CreateBoard(ctx, human, kanban.CreateBoardInput{Name: "No path", Slug: "NOPATH"})
	if err != nil {
		t.Fatal(err)
	}
	simple, err := core.CreateBoard(ctx, human, kanban.CreateBoardInput{Name: "Simple", Slug: "SIMPLE", Template: "simple", ProjectPath: board.ProjectPath})
	if err != nil {
		t.Fatal(err)
	}
	d := newDesktopDispatch(core, t.TempDir(), nil)
	t.Cleanup(d.close)
	handler := d.wrap("http://localhost:7070", http.NotFoundHandler())
	put := func(cfg dispatchConfig) (int, map[string]string) {
		raw, err := json.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPut, d.path, strings.NewReader(string(raw)))
		req.Header.Set("Origin", "http://localhost:7070")
		req.Header.Set("X-Slate-Desktop", "dispatch")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		var body map[string]string
		_ = json.Unmarshal(res.Body.Bytes(), &body)
		return res.Code, body
	}
	engineer := func(b *kanban.BoardDetail) int64 { return b.Engineers[0].ID }
	for name, test := range map[string]struct {
		cfg  dispatchConfig
		code string
	}{
		"missing board":   {dispatchConfig{Enabled: true}, "board"},
		"no project path": {dispatchConfig{BoardID: noPath.ID, Hosts: map[int64]string{engineer(noPath): "codex"}, Enabled: true}, "project"},
		"no host":         {dispatchConfig{BoardID: board.ID, Hosts: map[int64]string{}, Enabled: true}, "host"},
		"unknown host":    {dispatchConfig{BoardID: board.ID, Hosts: map[int64]string{engineer(board): "missing-host"}, Enabled: true}, "host"},
		"other engineer":  {dispatchConfig{BoardID: board.ID, Hosts: map[int64]string{engineer(noPath): "codex"}, Enabled: true}, "engineer"},
		"not a template":  {dispatchConfig{BoardID: simple.ID, Hosts: map[int64]string{engineer(simple): "codex"}, Enabled: true}, "template"},
	} {
		if code, body := put(test.cfg); code != http.StatusBadRequest || body["code"] != test.code {
			t.Errorf("%s: status %d body %v, want code %q", name, code, body, test.code)
		}
	}
}

// lastDesktopMarker 返回任务最后一条评论带的派发标记，没有评论时返回空串。
func lastDesktopMarker(card *kanban.CardDetail) string {
	if len(card.Comments) == 0 {
		return ""
	}
	return commentMarker(card.Comments[len(card.Comments)-1])
}

// lastDesktopComment 返回任务最后一条评论的正文，没有评论时返回空串。
func lastDesktopComment(card *kanban.CardDetail) string {
	if len(card.Comments) == 0 {
		return ""
	}
	return card.Comments[len(card.Comments)-1].Body
}

// 审查结论 passed 时桌面端把任务移入完成列；waiting 表示要人补验，
// 留言后把任务移进收件箱等人处理，不停放在审查列。
func TestDesktopDispatchReviewOutcomeMovesCard(t *testing.T) {
	for _, tt := range []struct {
		outcome string
		column  func(dispatchFlow) int64
	}{
		{"passed", func(f dispatchFlow) int64 { return f.done }},
		{"waiting", func(f dispatchFlow) int64 { return f.inbox }},
	} {
		t.Run(tt.outcome, func(t *testing.T) {
			ctx := t.Context()
			core, human, board, flow := newDispatchTestBoard(t)
			if _, err := gitOutput(ctx, board.ProjectPath, "branch", "task-branch"); err != nil {
				t.Fatal(err)
			}
			card, err := core.CreateCard(ctx, human, board.ID, kanban.CreateCardInput{Title: "Review me", ColumnID: flow.review, Branch: "task-branch"})
			if err != nil {
				t.Fatal(err)
			}
			host := func(_ context.Context, _, _, _ string, readOnly bool, _ dispatchSession) (dispatchReply, error) {
				if !readOnly {
					t.Error("review planning host was given write capability")
				}
				return dispatchReply{Outcome: "plan", Summary: "Review plan"}, nil
			}
			formal := false
			run := dispatchRun{core: core, sessions: t.TempDir()}
			run.host = func(ctx context.Context, id, dir, prompt string, readOnly bool, s dispatchSession) (dispatchReply, error) {
				if !strings.Contains(prompt, "Outcome must be plan or clarify.") {
					formal = true
					if readOnly {
						t.Error("review host cannot build or test in its disposable checkout")
					}
					return dispatchReply{Outcome: tt.outcome, Summary: "Evidence"}, nil
				}
				return host(ctx, id, dir, prompt, readOnly, s)
			}
			job := dispatchJob{id: card.ID, review: true, engineer: *board.Engineers[2], host: "pi", flow: flow, project: board.ProjectPath}
			if err := run.handle(ctx, job); err != nil {
				t.Fatal(err)
			}
			got, err := core.GetCardDetail(ctx, card.ID)
			if err != nil || !formal || got.ColumnID != tt.column(flow) || got.ClaimActive ||
				lastDesktopMarker(got) != completedReview || lastDesktopComment(got) != "Evidence" {
				t.Fatalf("unexpected review result: formal=%v column=%d claim=%v err=%v", formal, got.ColumnID, got.ClaimActive, err)
			}
		})
	}
}

// newReviewHost 返回审查用的假宿主：规划阶段回 plan，正式审查阶段按顺序回放 outcomes
// （用完后一直回最后一个），同时记下正式审查被调用的次数，用来核对任务有没有被（重新）审查。
func newReviewHost(outcomes ...string) (dispatchHost, *atomic.Int32) {
	var formal atomic.Int32
	host := func(_ context.Context, _, _, prompt string, _ bool, _ dispatchSession) (dispatchReply, error) {
		if strings.Contains(prompt, "Outcome must be plan or clarify.") {
			return dispatchReply{Outcome: "plan", Summary: "Review plan"}, nil
		}
		n := int(formal.Add(1))
		return dispatchReply{Outcome: outcomes[min(n, len(outcomes))-1], Summary: "Evidence"}, nil
	}
	return host, &formal
}

// 审查给出 waiting 后任务移进收件箱；人把它放回审查列，不用再留任何评论，
// 桌面端就会重新审查，这次通过则任务进入完成列。
func TestDesktopDispatchReviewRunsAgainWhenHumanRequeues(t *testing.T) {
	ctx := t.Context()
	core, human, board, flow := newDispatchTestBoard(t)
	if _, err := gitOutput(ctx, board.ProjectPath, "branch", "task-branch"); err != nil {
		t.Fatal(err)
	}
	card, err := core.CreateCard(ctx, human, board.ID, kanban.CreateCardInput{Title: "Review me", ColumnID: flow.review, Branch: "task-branch"})
	if err != nil {
		t.Fatal(err)
	}
	host, formal := newReviewHost("waiting", "passed")
	run := dispatchRun{core: core, sessions: t.TempDir(), host: host}
	job := dispatchJob{id: card.ID, review: true, engineer: *board.Engineers[2], host: "pi", flow: flow, project: board.ProjectPath}

	if err := run.handle(ctx, job); err != nil {
		t.Fatal(err)
	}
	got, err := core.GetCardDetail(ctx, card.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ColumnID != flow.inbox {
		t.Fatalf("a waiting review must hand the card to the inbox: column=%d, want %d", got.ColumnID, flow.inbox)
	}
	if _, err := core.MoveCard(ctx, human, card.ID, kanban.MoveCardInput{ColumnID: flow.review}); err != nil {
		t.Fatal(err)
	}
	if err := run.handle(ctx, job); err != nil {
		t.Fatalf("a card the human put back into review must be reviewed again without a comment: %v", err)
	}
	got, err = core.GetCardDetail(ctx, card.ID)
	if err != nil {
		t.Fatal(err)
	}
	if formal.Load() != 2 || got.ColumnID != flow.done || got.ClaimActive {
		t.Fatalf("unexpected second review: formal=%d column=%d claim=%v", formal.Load(), got.ColumnID, got.ClaimActive)
	}
}

// 旧版本留下的停放状态：任务在审查列，最后一条派发标记是已审查，之后没有人评论。
// 升级后桌面端要自己把它拉起来重审，而不是显示待核对、等人评论。
func TestDesktopDispatchReviewPullsCardParkedByOlderVersion(t *testing.T) {
	ctx := t.Context()
	core, human, board, flow := newDispatchTestBoard(t)
	if _, err := gitOutput(ctx, board.ProjectPath, "branch", "task-branch"); err != nil {
		t.Fatal(err)
	}
	card, err := core.CreateCard(ctx, human, board.ID, kanban.CreateCardInput{Title: "Review me", ColumnID: flow.review, Branch: "task-branch"})
	if err != nil {
		t.Fatal(err)
	}
	reviewer := kanban.Actor{Kind: kanban.ActorAgent, Name: board.Engineers[2].Name}
	if _, err := core.AddCommentWithExt(ctx, reviewer, card.ID, "Earlier evidence", markerExt(completedReview)); err != nil {
		t.Fatal(err)
	}
	host, formal := newReviewHost("passed")
	run := dispatchRun{core: core, sessions: t.TempDir(), host: host}
	job := dispatchJob{id: card.ID, review: true, engineer: *board.Engineers[2], host: "pi", flow: flow, project: board.ProjectPath}

	if err := run.handle(ctx, job); err != nil {
		t.Fatalf("a review parked by an older version was not pulled: %v", err)
	}
	got, err := core.GetCardDetail(ctx, card.ID)
	if err != nil {
		t.Fatal(err)
	}
	if formal.Load() != 1 || got.ColumnID != flow.done || got.ClaimActive {
		t.Fatalf("unexpected pulled review: formal=%d column=%d claim=%v", formal.Load(), got.ColumnID, got.ClaimActive)
	}
}

// 审查留下结论后，任务只要回到审查列就能重审，不再等人评论确认；
// 研发的恢复判断不看审查结论；审查中途中断留下的待核对留言仍要等恢复码。
func TestDesktopDispatchReviewDoesNotWaitForHumanComment(t *testing.T) {
	reviewJob := dispatchJob{review: true}
	devJob := dispatchJob{}
	agent := func(body, marker string) *kanban.Comment {
		return &kanban.Comment{AuthorKind: string(kanban.ActorAgent), Author: "Atlas", Body: body, Ext: markerExt(marker)}
	}
	human := &kanban.Comment{AuthorKind: string(kanban.ActorHuman), Author: "me", Body: "看过了"}
	for name, tt := range map[string]struct {
		review   bool
		comments []*kanban.Comment
		want     dispatchRecovery
	}{
		"审查已留结论后无人评论也能重审":    {true, []*kanban.Comment{agent("Evidence", completedReview)}, dispatchAvailable},
		"审查已留结论后有人评论也能重审":    {true, []*kanban.Comment{agent("Evidence", completedReview), human}, dispatchAvailable},
		"审查中途中断的待核对留言仍要等恢复码": {true, []*kanban.Comment{agent("Evidence", completedReview), agent("notice", reviewJob.retryPrefix()+"abc -->")}, dispatchAwaitingConfirmation},
		"研发的恢复判断不看审查结论":      {false, []*kanban.Comment{agent("", devJob.startedMarker()), agent("Evidence", completedReview)}, dispatchNeedsRecoveryNotice},
	} {
		if got := desktopDispatchRecovery(&kanban.CardDetail{Comments: tt.comments}, tt.review); got != tt.want {
			t.Errorf("%s: got %v, want %v", name, got, tt.want)
		}
	}
}

// renewCountingCore 统计桌面端后台续租的次数（只续租的领取请求），用来核对租约有没有在续。
type renewCountingCore struct {
	dispatchCore
	renewals atomic.Int32
}

// ClaimCard 先数一下只续租的请求，再交给真正的实现。
func (c *renewCountingCore) ClaimCard(ctx context.Context, actor kanban.Actor, id int64, in kanban.ClaimCardInput) (*kanban.CardDetail, error) {
	if in.RenewOnly {
		c.renewals.Add(1)
	}
	return c.dispatchCore.ClaimCard(ctx, actor, id, in)
}

// stallGitCommonDir 在 PATH 最前面放一个 git 包装脚本：查 git 公共目录时先睡 delay 秒再交给真 git，
// 模拟「访问项目目录被系统授权弹窗卡住」。
func stallGitCommonDir(t *testing.T, delay string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("包装脚本只在类 Unix 系统上可用")
	}
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\ncase \"$*\" in\n*--git-common-dir*) sleep " + delay + " ;;\nesac\nexec '" + strings.ReplaceAll(realGit, "'", `'\''`) + "' \"$@\"\n"
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// 第一次访问项目目录就可能被系统卡很久（例如 macOS 弹出文稿目录授权，人不在就一直等）。
// 后台续租必须在这之前就启动，否则卡住期间租约悄悄过期，一恢复整次派发就被中止。
func TestDesktopDispatchRenewsLeaseWhileProjectAccessIsBlocked(t *testing.T) {
	ctx := t.Context()
	// 时钟拨慢 25 分钟：领取后租约只剩约 5 分钟，后台马上需要续租。
	core, human, board, flow := newDispatchTestBoard(t, kanban.WithClock(func() time.Time { return time.Now().Add(-25 * time.Minute) }))
	if _, err := gitOutput(ctx, board.ProjectPath, "branch", "task-branch"); err != nil {
		t.Fatal(err)
	}
	card, err := core.CreateCard(ctx, human, board.ID, kanban.CreateCardInput{Title: "Review me", ColumnID: flow.review, Branch: "task-branch"})
	if err != nil {
		t.Fatal(err)
	}
	stallGitCommonDir(t, "0.5")
	counting := &renewCountingCore{dispatchCore: core}
	run := dispatchRun{core: counting, sessions: t.TempDir(), renewEvery: 20 * time.Millisecond}
	run.host = func(_ context.Context, _, _, prompt string, _ bool, _ dispatchSession) (dispatchReply, error) {
		if strings.Contains(prompt, "Outcome must be plan or clarify.") {
			if counting.renewals.Load() == 0 {
				t.Error("lease was not renewed while the first project access was blocked")
			}
			return dispatchReply{Outcome: "plan", Summary: "Review plan"}, nil
		}
		return dispatchReply{Outcome: "passed", Summary: "Evidence"}, nil
	}
	job := dispatchJob{id: card.ID, review: true, engineer: *board.Engineers[2], host: "pi", flow: flow, project: board.ProjectPath}
	if err := run.handle(ctx, job); err != nil {
		t.Fatal(err)
	}
}

// 规划阶段的检查碰上上下文已取消（例如租约失效，后台续租中止了本次派发）时，
// 按「任务在规划期间变了」处理，调度器会马上重新安排；否则审查任务会被当成已处理而停放，要人去触发。
func TestDesktopDispatchPlanCheckTreatsCancelAsSuperseded(t *testing.T) {
	ctx := t.Context()
	core, human, board, flow := newDispatchTestBoard(t)
	card, err := core.CreateCard(ctx, human, board.ID, kanban.CreateCardInput{Title: "Review me", ColumnID: flow.review})
	if err != nil {
		t.Fatal(err)
	}
	job := dispatchJob{id: card.ID, review: true, engineer: *board.Engineers[2], host: "pi", flow: flow, project: board.ProjectPath}
	if _, err := core.ClaimCard(ctx, job.actor(), card.ID, kanban.ClaimCardInput{EngineerID: job.engineer.ID, LeaseMinutes: dispatchLeaseMinutes}); err != nil {
		t.Fatal(err)
	}
	planned, err := core.GetCardDetail(ctx, card.ID)
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	run := dispatchRun{core: core}
	if _, err := run.checkDispatchPlan(canceled, job, planned); !errors.Is(err, errDispatchPlanSuperseded) {
		t.Fatalf("a canceled plan check must count as superseded, got %v", err)
	}
}

// 审查副本对宿主是可写的，跑完后桌面端要核对副本原样不动：留下文件或多出提交，
// 都说明审查过程不可信，不能据此把任务移入完成列；任务留在审查列，并留下待人核对的恢复留言。
func TestDesktopDispatchReviewRejectsChangedCheckout(t *testing.T) {
	for name, change := range map[string]func(t *testing.T, dir string){
		"留下未跟踪文件": func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(dir, "stray.txt"), []byte("left behind"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"多出提交": func(t *testing.T, dir string) {
			if _, err := gitOutput(t.Context(), dir, "-c", "user.name=Test", "-c", "user.email=test@example.local", "commit", "--allow-empty", "-m", "Extra"); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()
			core, human, board, flow := newDispatchTestBoard(t)
			if _, err := gitOutput(ctx, board.ProjectPath, "branch", "task-branch"); err != nil {
				t.Fatal(err)
			}
			card, err := core.CreateCard(ctx, human, board.ID, kanban.CreateCardInput{Title: "Review me", ColumnID: flow.review, Branch: "task-branch"})
			if err != nil {
				t.Fatal(err)
			}
			run := dispatchRun{core: core, sessions: t.TempDir()}
			run.host = func(_ context.Context, _, dir, prompt string, _ bool, _ dispatchSession) (dispatchReply, error) {
				if strings.Contains(prompt, "Outcome must be plan or clarify.") {
					return dispatchReply{Outcome: "plan", Summary: "Review plan"}, nil
				}
				change(t, dir)
				return dispatchReply{Outcome: "passed", Summary: "Evidence"}, nil
			}
			job := dispatchJob{id: card.ID, review: true, engineer: *board.Engineers[2], host: "pi", flow: flow, project: board.ProjectPath}
			if err := run.handle(ctx, job); err == nil {
				t.Fatal("a review that changed its checkout was accepted")
			}
			got, err := core.GetCardDetail(ctx, card.ID)
			if err != nil || got.ColumnID != flow.review || got.ClaimActive || desktopDispatchRecovery(got, true) != dispatchAwaitingConfirmation {
				t.Fatalf("unexpected state after rejected review: err=%v card=%+v", err, got)
			}
		})
	}
}

// 新版本把派发标记放在评论扩展字段里，恢复判断要和旧版正文末行的标记得到同样的结果；
// 扩展字段存在时不再回头读正文，正文里的标记文字不能冒充。
func TestDesktopDispatchRecoveryReadsExtMarker(t *testing.T) {
	job := dispatchJob{}
	agent := string(kanban.ActorAgent)
	comment := func(body, marker string) *kanban.Comment {
		c := &kanban.Comment{AuthorKind: agent, Author: "Atlas", Body: body}
		if marker != "" {
			c.Ext = markerExt(marker)
		}
		return c
	}
	human := &kanban.Comment{AuthorKind: string(kanban.ActorHuman), Author: "me", Body: retryCommandPrefix + "abc"}
	for name, tt := range map[string]struct {
		comments []*kanban.Comment
		want     dispatchRecovery
	}{
		"扩展字段里的开始标记": {[]*kanban.Comment{comment("", job.startedMarker())}, dispatchNeedsRecoveryNotice},
		"扩展字段里的暂缓标记": {[]*kanban.Comment{comment("", job.startedMarker()), comment("", job.deferredMarker())}, dispatchAvailable},
		"待核对留言等人确认":  {[]*kanban.Comment{comment("notice", job.retryPrefix()+"abc"+" -->")}, dispatchAwaitingConfirmation},
		"人确认后可以重试":   {[]*kanban.Comment{comment("notice", job.retryPrefix()+"abc"+" -->"), human}, dispatchAvailable},
		"正文末行不能冒充标记": {[]*kanban.Comment{comment("hello\n"+completedDevelopment, job.startedMarker())}, dispatchNeedsRecoveryNotice},
	} {
		if got := desktopDispatchRecovery(&kanban.CardDetail{Comments: tt.comments}, false); got != tt.want {
			t.Errorf("%s: got %v, want %v", name, got, tt.want)
		}
	}
}

// countCardEvents 统计看板活动里某个任务的某类事件有几条，用来核对领取、释放有没有重复。
func countCardEvents(t *testing.T, core kanban.Core, boardID, cardID int64, eventType string) int {
	t.Helper()
	page, err := core.ListActivity(t.Context(), boardID, kanban.ActivityQuery{
		ActivityFilter: kanban.ActivityFilter{Types: []string{eventType}, CardID: cardID},
	})
	if err != nil {
		t.Fatal(err)
	}
	return len(page.Events)
}

// 任务不需要改仓库时（例如只要求在评论区输出内容），宿主回 reported 也能正常交接：
// 摘要原样发成交接评论，任务进入审查列，分支上没有新提交；从待办开始的整个过程只领取一次，也没有中途释放。
func TestDesktopDispatchReportedOutcomeHandsOffWithoutCommit(t *testing.T) {
	ctx := t.Context()
	core, human, board, flow := newDispatchTestBoard(t)
	card, err := core.CreateCard(ctx, human, board.ID, kanban.CreateCardInput{Title: "在评论区输出当前时间", ColumnID: flow.todo})
	if err != nil {
		t.Fatal(err)
	}
	const answer = "当前时间：2026-11-25 09:52:20（北京时间）"
	run := dispatchRun{core: core, sessions: t.TempDir(), text: func() desktopText { return desktopTexts["zh-CN"] }}
	run.host = func(_ context.Context, _, _, _ string, readOnly bool, _ dispatchSession) (dispatchReply, error) {
		if readOnly {
			return dispatchReply{Outcome: "plan", Summary: "Plan"}, nil
		}
		return dispatchReply{Outcome: "reported", Summary: answer, Items: "- [x] 输出当前时间"}, nil
	}
	job := dispatchJob{id: card.ID, engineer: *board.Engineers[0], host: "codex", flow: flow, project: board.ProjectPath}
	if err := run.handle(ctx, job); err != nil {
		t.Fatal(err)
	}
	got, err := core.GetCardDetail(ctx, card.ID)
	if err != nil || got.ColumnID != flow.review || got.ClaimActive || got.Branch == "" ||
		lastDesktopMarker(got) != completedDevelopment || lastDesktopComment(got) != answer ||
		!strings.Contains(got.Description, "## 任务项\n\n- [x] 输出当前时间") {
		t.Fatalf("unexpected hand-off: err=%v card=%+v", err, got)
	}
	base, baseErr := gitOutput(ctx, board.ProjectPath, "rev-parse", "main")
	tip, tipErr := gitOutput(ctx, board.ProjectPath, "rev-parse", got.Branch)
	if baseErr != nil || tipErr != nil || base != tip {
		t.Fatalf("reported hand-off must not add commits: main=%q branch=%q errs=%v %v", base, tip, baseErr, tipErr)
	}
	if claimed := countCardEvents(t, core, board.ID, card.ID, kanban.EventCardClaimed); claimed != 1 {
		t.Errorf("want exactly one claim event, got %d", claimed)
	}
	if released := countCardEvents(t, core, board.ID, card.ID, kanban.EventCardReleased); released != 0 {
		t.Errorf("hand-off must not write a release event, got %d", released)
	}
}

// 宿主声称不需要改仓库，工作树里却留下了文件：结果不可信，不能进审查，
// 任务留在进行中，桌面端留下待人核对的恢复留言。
func TestDesktopDispatchReportedOutcomeRejectsChangedWorktree(t *testing.T) {
	ctx := t.Context()
	core, human, board, flow := newDispatchTestBoard(t)
	card, err := core.CreateCard(ctx, human, board.ID, kanban.CreateCardInput{Title: "Report only", ColumnID: flow.dev})
	if err != nil {
		t.Fatal(err)
	}
	run := dispatchRun{core: core, sessions: t.TempDir()}
	run.host = func(_ context.Context, _, dir, _ string, readOnly bool, _ dispatchSession) (dispatchReply, error) {
		if readOnly {
			return dispatchReply{Outcome: "plan", Summary: "Plan"}, nil
		}
		if err := os.WriteFile(filepath.Join(dir, "stray.txt"), []byte("left behind"), 0o600); err != nil {
			return dispatchReply{}, err
		}
		return dispatchReply{Outcome: "reported", Summary: "Nothing to change", Items: "- [x] Report"}, nil
	}
	job := dispatchJob{id: card.ID, engineer: *board.Engineers[0], host: "codex", flow: flow, project: board.ProjectPath}
	if err := run.handle(ctx, job); err == nil {
		t.Fatal("a reported hand-off with a changed worktree was accepted")
	}
	got, err := core.GetCardDetail(ctx, card.ID)
	if err != nil || got.ColumnID != flow.dev || got.ClaimActive || desktopDispatchRecovery(got, false) != dispatchAwaitingConfirmation {
		t.Fatalf("unexpected state after rejected report: err=%v card=%+v", err, got)
	}
}

// 提示词要把结果怎么回到看板讲清楚：摘要会被桌面端发成评论、不改仓库的任务回 reported、
// 实时信息自己用命令取、评审遇到无改动交接时去卡片上核对；规划阶段的结论仍只能是 plan 或 clarify。
func TestDispatchPromptExplainsHowResultsReachTheBoard(t *testing.T) {
	engineer := kanban.Engineer{Name: "Atlas", Prompt: "Be careful."}
	card := &kanban.CardDetail{CardView: &kanban.CardView{Card: &kanban.Card{Key: "DISP-1", Title: "Report time"}}}
	for name, tt := range map[string]struct {
		job      dispatchJob
		planning bool
		want     []string
		missing  []string
	}{
		"development": {
			job: dispatchJob{engineer: engineer},
			want: []string{
				"Outcome must be ready or reported.", "posts your summary on the card as a comment",
				"a shell command like date", "return reported", "ready or reported result, items has one Markdown checkbox",
			},
			missing: []string{"Outcome must be plan or clarify."},
		},
		"planning": {
			job: dispatchJob{engineer: engineer}, planning: true,
			want:    []string{"Outcome must be plan or clarify.", "posts your summary on the card as a comment"},
			missing: []string{"return reported"},
		},
		"review": {
			job: dispatchJob{engineer: engineer, review: true},
			want: []string{
				"Outcome must be passed, failed or waiting.", "no repository change", "posts your summary on the card as a comment",
				"disposable checkout", "build and run tests", "rejects the review",
			},
			missing: []string{"Outcome must be plan or clarify.", "return reported", "read-only"},
		},
	} {
		prompt := dispatchPrompt(tt.job, card, "", tt.planning, desktopTexts["en"].dispatchOutputLanguage)
		for _, want := range tt.want {
			if !strings.Contains(prompt, want) {
				t.Errorf("%s prompt lacks %q", name, want)
			}
		}
		for _, missing := range tt.missing {
			if strings.Contains(prompt, missing) {
				t.Errorf("%s prompt must not contain %q", name, missing)
			}
		}
	}
}
