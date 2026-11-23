package desktop

import (
	"context"
	"crypto/rand"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/skills"
)

type worktreeCleanup struct {
	path string
}

func newWorktreeCleanup() worktreeCleanup {
	return worktreeCleanup{path: "/.slate-desktop/" + rand.Text() + "/worktree-cleanup"}
}

func (c worktreeCleanup) script(origin string) string {
	return fmt.Sprintf(`(() => {
		if (location.origin !== %q) return;
		window.slateLocalCleanup = %q;
		window.dispatchEvent(new Event('slate-local-cleanup'));
	})()`, origin, c.path)
}

type cleanupRunner func(context.Context, int64, string, string) ([]byte, error)

func wrapCleanup(origin, endpoint string, run cleanupRunner, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != endpoint {
			next.ServeHTTP(w, r)
			return
		}
		peer, _, _ := net.SplitHostPort(r.RemoteAddr)
		if r.Method != http.MethodPost || r.Header.Get("Origin") != origin || !net.ParseIP(peer).IsLoopback() {
			http.NotFound(w, r)
			return
		}
		var input struct {
			CardID int64  `json:"card_id"`
			Action string `json:"action"`
			Token  string `json:"preview_token"`
		}
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1024))
		if err != nil || json.Unmarshal(data, &input) != nil || input.CardID <= 0 {
			http.Error(w, "Invalid cleanup request", http.StatusBadRequest)
			return
		}
		switch input.Action {
		case "cleanup-status", "cleanup-preview", "cleanup-retry":
			if input.Token != "" {
				http.Error(w, "Unexpected cleanup confirmation", http.StatusBadRequest)
				return
			}
		case "cleanup-archive":
			if input.Token == "" {
				http.Error(w, "Missing cleanup confirmation", http.StatusBadRequest)
				return
			}
		default:
			http.Error(w, "Unknown cleanup action", http.StatusBadRequest)
			return
		}
		result, err := run(r.Context(), input.CardID, input.Action, input.Token)
		w.Header().Set("Cache-Control", "no-store")
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(result)
	})
}

func runWorktreeCleanup(core kanban.Core) cleanupRunner {
	return func(ctx context.Context, cardID int64, action, token string) ([]byte, error) {
		card, err := core.GetCardDetail(ctx, cardID)
		if err != nil {
			return nil, err
		}
		if !card.Done || len(card.MergedCommits) == 0 || !strings.HasPrefix(card.Branch, "slate/"+strings.ToLower(card.Key)+"-") {
			return json.Marshal(map[string]string{"status": "unknown"})
		}
		board, err := core.GetBoard(ctx, card.BoardID)
		if err != nil {
			return nil, err
		}
		if board.ProjectPath == "" {
			return json.Marshal(map[string]string{"status": "unknown"})
		}
		return invokeCleanup(ctx, board.ProjectPath, card, action, token)
	}
}

func invokeCleanup(ctx context.Context, project string, card *kanban.CardDetail, action, token string) ([]byte, error) {
	python, err := exec.LookPath("python3")
	if err != nil && runtime.GOOS == "windows" {
		python, err = exec.LookPath("python")
	}
	if err != nil {
		return nil, errors.New("Python 3.9+ is required for local worktree cleanup")
	}
	dir, err := os.MkdirTemp("", "slate-worktree-cleanup-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	script, proof := filepath.Join(dir, "integrate.py"), filepath.Join(dir, "card.json")
	if err := os.WriteFile(script, []byte(skills.IntegrateScript()), 0o600); err != nil {
		return nil, err
	}
	data, err := json.Marshal(card)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(proof, data, 0o600); err != nil {
		return nil, err
	}
	args := []string{"-I", script, "--project", project, "--card-id", fmt.Sprint(card.ID),
		"--key", card.Key, "--branch", card.Branch, "--card-json", proof, action}
	if action == "cleanup-archive" {
		args = append(args, "--preview-token", token)
	}
	if action == "cleanup-archive" || action == "cleanup-retry" {
		ctx = context.WithoutCancel(ctx)
	}
	command := exec.CommandContext(ctx, python, args...)
	command.Dir = dir
	result, err := command.Output()
	if exit, ok := errors.AsType[*exec.ExitError](err); ok && len(exit.Stderr) > 0 {
		return nil, errors.New(strings.TrimSpace(string(exit.Stderr)))
	}
	return result, err
}
