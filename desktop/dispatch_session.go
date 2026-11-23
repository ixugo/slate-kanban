package desktop

import (
	"crypto/sha256"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"uuid"
)

type dispatchSession struct {
	dir  string
	name string
}

func taskDispatchSession(root string, job dispatchJob, key string) (dispatchSession, error) {
	if root == "" {
		return dispatchSession{}, nil
	}
	dirKey, err := keyForBranch(strings.ToLower(key))
	if err != nil {
		return dispatchSession{}, err
	}
	stage := "development"
	if job.review {
		stage = "review"
	}
	project := sha256.Sum256([]byte(filepath.Clean(job.project)))
	dir := filepath.Join(root, fmt.Sprintf("%x", project[:8]), dirKey, stage, job.host)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return dispatchSession{}, err
	}
	return dispatchSession{dir: dir, name: key + " · " + stage}, nil
}

func (s dispatchSession) priorThread() (string, error) {
	if s.dir == "" {
		return "", nil
	}
	raw, err := os.ReadFile(filepath.Join(s.dir, "thread-id"))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(string(raw))
	if _, err := uuid.Parse(id); err != nil {
		return "", fmt.Errorf("stored host session ID is invalid: %w", err)
	}
	return id, nil
}

func (s dispatchSession) rememberThread(id string) error {
	if s.dir == "" || id == "" {
		return nil
	}
	if _, err := uuid.Parse(id); err != nil {
		return fmt.Errorf("host returned an invalid session ID: %w", err)
	}
	prev, err := s.priorThread()
	if err != nil {
		return err
	}
	if prev != "" {
		if prev != id {
			return errUnconfirmedRun
		}
		return nil
	}
	tmp, err := os.CreateTemp(s.dir, ".thread-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.WriteString(id)
	if err = errors.Join(err, tmp.Close()); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(s.dir, "thread-id"))
}

func (s dispatchSession) piArgs(host string) ([]string, error) {
	if s.dir == "" {
		return []string{"--no-session"}, nil
	}
	directory := filepath.Join(s.dir, "conversations")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	args := []string{"--session-dir", directory}
	if len(entries) > 0 {
		args = append(args, "--continue")
	}
	if host == "pi" {
		args = append(args, "--name", s.name)
	}
	return args, nil
}

func (s dispatchSession) hasConversation(host string) bool {
	if host == "codex" || host == "claude" {
		id, err := s.priorThread()
		return err == nil && id != ""
	}
	entries, err := os.ReadDir(filepath.Join(s.dir, "conversations"))
	return err == nil && len(entries) > 0
}

func codexThreadID(events string) string {
	for line := range strings.SplitSeq(events, "\n") {
		var event struct {
			Type     string `json:"type"`
			ThreadID string `json:"thread_id"`
		}
		if json.Unmarshal([]byte(line), &event) == nil && event.Type == "thread.started" {
			return event.ThreadID
		}
	}
	return ""
}
