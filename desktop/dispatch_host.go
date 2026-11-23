package desktop

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"uuid"
)

type hostSpec struct {
	id, name string
	names    []string
}

var desktopHosts = []hostSpec{
	{"claude", "Claude Code", []string{"claude"}},
	{"codex", "Codex", []string{"codex"}},
	{"pi", "Pi", []string{"pi"}},
	{"on-my-pi", "Oh My Pi", []string{"omp", "on-my-pi"}},
}

type availableHost struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Available bool   `json:"available"`
}

func lookupDesktopHost(id string) (string, error) {
	for _, spec := range desktopHosts {
		if spec.id != id {
			continue
		}
		for _, name := range spec.names {
			if executable, err := exec.LookPath(name); err == nil {
				return executable, nil
			}
			// Finder-launched applications may have a minimal PATH.
			for _, dir := range desktopToolDirs() {
				if executable, err := exec.LookPath(filepath.Join(dir, name)); err == nil {
					return executable, nil
				}
			}
		}
	}
	return "", errors.New("selected host executable is unavailable")
}

func desktopToolDirs() []string {
	dirs := []string{"/opt/homebrew/bin", "/usr/local/bin"}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".local", "bin"), filepath.Join(home, ".npm-global", "bin"))
	}
	return dirs
}

type dispatchReply struct {
	Outcome string `json:"outcome"`
	Summary string `json:"summary"`
	Items   string `json:"items"`
}

type dispatchHost func(ctx context.Context, host, dir, prompt string, readOnly bool, session dispatchSession) (dispatchReply, error)

var errProviderBackoff = errors.New("provider rate limit or usage quota")
var errUnconfirmedRun = errors.New("host result could not be confirmed")

func localDispatchHost(ctx context.Context, host, dir, prompt string, readOnly bool, session dispatchSession) (dispatchReply, error) {
	executable, err := lookupDesktopHost(host)
	if err != nil {
		return dispatchReply{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Minute)
	defer cancel()
	var args []string
	var finalFile, promptFile string
	threadID, err := session.priorThread()
	if err != nil {
		return dispatchReply{}, err
	}
	if session.name != "" {
		prompt = "Slate task session: " + session.name + "\n\n" + prompt
	}
	switch host {
	case "codex":
		tmp, err := os.MkdirTemp("", "slate-desktop-receipt-")
		if err != nil {
			return dispatchReply{}, err
		}
		defer os.RemoveAll(tmp)
		finalFile = filepath.Join(tmp, "last-message")
		schema := filepath.Join(tmp, "receipt.json")
		const receiptSchema = `{"type":"object","properties":{"outcome":{"type":"string"},"summary":{"type":"string"},"items":{"type":"string"}},"required":["outcome","summary","items"],"additionalProperties":false}`
		if err := os.WriteFile(schema, []byte(receiptSchema), 0o600); err != nil {
			return dispatchReply{}, err
		}
		mode := "workspace-write"
		if readOnly {
			mode = "read-only"
		}
		args = []string{"-a", "never", "-C", dir, "-c", "sandbox_mode=\"" + mode + "\"", "exec"}
		if threadID != "" {
			args = append(args, "resume")
		} else if session.dir == "" {
			args = append(args, "--ephemeral")
		}
		args = append(args, "--json", "--output-last-message", finalFile, "--output-schema", schema)
		if threadID != "" {
			args = append(args, threadID)
		}
		args = append(args, "-")
	case "claude":
		args = []string{"--print", "--output-format", "json"}
		if session.dir == "" {
			args = append(args, "--no-session-persistence")
		} else if threadID != "" {
			args = append(args, "--resume", threadID, "--name", session.name)
		} else {
			args = append(args, "--session-id", uuid.NewV4().String(), "--name", session.name)
		}
		if readOnly {
			args = append(args, "--permission-mode", "plan")
		} else {
			args = append(args, "--permission-mode", "acceptEdits")
		}
	case "pi", "on-my-pi":
		tmp, err := os.MkdirTemp("", "slate-desktop-task-")
		if err != nil {
			return dispatchReply{}, err
		}
		defer os.RemoveAll(tmp)
		promptFile = filepath.Join(tmp, "instructions.md")
		if err := os.WriteFile(promptFile, []byte(prompt), 0o600); err != nil {
			return dispatchReply{}, err
		}
		args = []string{"--print", "--mode", "text"}
		sessionArgs, err := session.piArgs(host)
		if err != nil {
			return dispatchReply{}, err
		}
		args = append(args, sessionArgs...)
		if readOnly {
			args = append(args, "--tools", "read,grep,find,ls")
		}
		args = append(args, "--", "@"+promptFile, "Follow the attached task instructions.")
	}
	cmd := exec.CommandContext(ctx, executable, args...)
	configureDispatchProcess(cmd)
	cmd.Dir = dir
	path := strings.Join(append([]string{os.Getenv("PATH"), filepath.Dir(executable)}, desktopToolDirs()...), string(os.PathListSeparator))
	cmd.Env = append(cmd.Environ(), "PATH="+path)
	cmd.WaitDelay = 5 * time.Second
	if finalFile != "" || host == "claude" {
		cmd.Stdin = strings.NewReader(prompt)
	}
	stdout, stderr := newDispatchOutput(), newDispatchOutput()
	cmd.Stdout, cmd.Stderr = stdout, stderr
	runErr := cmd.Run()
	if host == "codex" {
		if err := session.rememberThread(codexThreadID(stdout.first.String())); err != nil {
			return dispatchReply{}, err
		}
	}
	raw := stdout.Bytes()
	if host == "claude" {
		var envelope struct {
			IsError   bool   `json:"is_error"`
			Result    string `json:"result"`
			SessionID string `json:"session_id"`
		}
		if json.Unmarshal(raw, &envelope) == nil {
			if err := session.rememberThread(envelope.SessionID); err != nil {
				return dispatchReply{}, err
			}
			if envelope.IsError {
				if providerBackoff(envelope.Result) {
					return dispatchReply{}, errProviderBackoff
				}
				return dispatchReply{}, errUnconfirmedRun
			}
			raw = []byte(envelope.Result)
		}
	}
	if ctx.Err() != nil {
		return dispatchReply{}, ctx.Err()
	}
	if runErr != nil {
		if providerBackoff(stderr.String()) {
			return dispatchReply{}, errProviderBackoff
		}
		return dispatchReply{}, errUnconfirmedRun
	}
	if finalFile != "" {
		raw, err = os.ReadFile(finalFile)
		if err != nil {
			return dispatchReply{}, errUnconfirmedRun
		}
	}
	reply, err := parseDispatchReply(raw)
	if err != nil || session.dir == "" {
		return reply, err
	}
	if !session.hasConversation(host) {
		return dispatchReply{}, errUnconfirmedRun
	}
	return reply, nil
}

func parseDispatchReply(raw []byte) (dispatchReply, error) {
	var reply dispatchReply
	raw = bytes.TrimSpace(raw)
	raw = bytes.TrimPrefix(raw, []byte("\x60\x60\x60json\n"))
	raw = bytes.TrimSuffix(raw, []byte("\n\x60\x60\x60"))
	if len(raw) > 64*1024 || json.Unmarshal(raw, &reply, json.RejectUnknownMembers(true)) != nil ||
		strings.TrimSpace(reply.Summary) == "" || len([]rune(reply.Summary)) > 14000 || len([]rune(reply.Items)) > 6000 {
		return dispatchReply{}, errUnconfirmedRun
	}
	return reply, nil
}

func providerBackoff(message string) bool {
	message = strings.ToLower(message)
	for _, indicator := range []string{"rate_limit_exceeded", "rate_limit_error", "insufficient_quota", "usage_limit_reached", "credit balance is too low", "usage limit has been reached", "you've hit your usage limit", "you’ve hit your usage limit", "429 too many requests", "rate limit reached", "rate limit exceeded"} {
		if strings.Contains(message, indicator) {
			return true
		}
	}
	return false
}

func keyForBranch(key string) (string, error) {
	for _, c := range key {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return "", fmt.Errorf("invalid card key")
		}
	}
	return key, nil
}

// Host output is used only for parsing. Limit capture and never log provider
// transcripts or error bodies that may contain credentials or task content.
type dispatchOutput struct {
	buf   bytes.Buffer
	first bytes.Buffer
}

func newDispatchOutput() *dispatchOutput { return &dispatchOutput{} }

func (o *dispatchOutput) Bytes() []byte  { return o.buf.Bytes() }
func (o *dispatchOutput) String() string { return o.buf.String() }

func (o *dispatchOutput) Write(p []byte) (int, error) {
	n := len(p)
	if room := (64 << 10) - o.first.Len(); room > 0 {
		_, _ = o.first.Write(p[:min(room, n)])
	}
	const limit = 1 << 20
	if o.buf.Len()+n > limit {
		if n >= limit {
			o.buf.Reset()
			p = p[n-limit:]
		} else {
			o.buf.Next(o.buf.Len() + n - limit)
		}
	}
	_, _ = o.buf.Write(p)
	return n, nil
}
