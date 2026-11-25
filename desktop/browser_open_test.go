//go:build darwin || linux

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
	"testing"
	"time"
)

func TestDesktopBrowserReportsLauncherExitAndAllowsRetry(t *testing.T) {
	dir := t.TempDir()
	capture := filepath.Join(dir, "opened-url")
	t.Setenv("PATH", dir)
	t.Setenv("SLATE_TEST_BROWSER_CAPTURE", capture)
	command := "open"
	if runtime.GOOS == "linux" {
		command = "gio"
	}
	writeBrowserLauncher(t, dir, command, `
if [ "$#" -eq 2 ] && [ "$1" = open ]; then shift; fi
if [ ! -f "$SLATE_TEST_BROWSER_CAPTURE" ]; then
  printf '%s\n' "$1" > "$SLATE_TEST_BROWSER_CAPTURE"
  exit 23
fi
printf '%s\n' "$1" >> "$SLATE_TEST_BROWSER_CAPTURE"
`)
	const origin = "http://127.0.0.1:51234"
	connection := desktopConnection{openPath: "/browser"}
	handler := connection.wrap(origin, openDesktopBrowser, http.NotFoundHandler())
	for _, want := range []int{http.StatusInternalServerError, http.StatusNoContent} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, origin+connection.openPath, nil)
		req.RemoteAddr = "127.0.0.1:51235"
		req.Header.Set("Origin", origin)
		req.Header.Set("X-Slate-Desktop", "browser")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		if recorder.Code != want {
			t.Fatalf("open status = %d, want %d", recorder.Code, want)
		}
	}
	actual, err := os.ReadFile(capture)
	if err != nil || string(actual) != origin+"\n"+origin+"\n" {
		t.Fatalf("launch targets = %q (%v)", actual, err)
	}
}

func TestDesktopBrowserRequestCancelsLauncherWait(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started := make(chan struct{})
	finished := make(chan struct{})
	stopTest := make(chan struct{})
	defer close(stopTest)
	const origin = "http://127.0.0.1:51234"
	connection := desktopConnection{openPath: "/browser"}
	handler := connection.wrap(origin, func(launcherCtx context.Context, _ string) error {
		close(started)
		select {
		case <-launcherCtx.Done():
			return launcherCtx.Err()
		case <-stopTest:
			return errors.New("launcher context was never canceled")
		}
	}, http.NotFoundHandler())
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, origin+connection.openPath, nil)
	req.RemoteAddr = "127.0.0.1:51235"
	req.Header.Set("Origin", origin)
	req.Header.Set("X-Slate-Desktop", "browser")
	go func() {
		defer close(finished)
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("browser handler did not start")
	}
	cancel()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("browser handler still waiting after request cancellation")
	}
}

func TestBrowserLauncherKeepsURLAsOneArgument(t *testing.T) {
	const target = "http://[::1]:51234/?first=one%20two&second=3"
	for _, test := range []struct {
		name, platform, command string
		wantArgs                []string
	}{
		{name: "macOS", platform: "darwin", command: "open", wantArgs: []string{target}},
		{name: "Windows", platform: "windows", command: "rundll32.exe", wantArgs: []string{"url.dll,FileProtocolHandler", target}},
		{name: "Linux GIO", platform: "linux", command: "gio", wantArgs: []string{"open", target}},
		{name: "Linux fallback", platform: "linux", command: "xdg-open", wantArgs: []string{target}},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			capture := filepath.Join(dir, "arguments")
			t.Setenv("PATH", dir)
			t.Setenv("SLATE_TEST_BROWSER_CAPTURE", capture)
			writeBrowserLauncher(t, dir, test.command, `printf '%s\n' "$@" > "$SLATE_TEST_BROWSER_CAPTURE"`)
			if err := runBrowserLauncher(t.Context(), test.platform, target); err != nil {
				t.Fatal(err)
			}
			actual, err := os.ReadFile(capture)
			want := strings.Join(test.wantArgs, "\n") + "\n"
			if err != nil || string(actual) != want {
				t.Fatalf("launch arguments = %q, want %q (%v)", actual, want, err)
			}
		})
	}
}

func TestBrowserLauncherEndsWaitOnDeadline(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	writeBrowserLauncher(t, dir, "open", "exec /bin/sleep 10")

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	err := runBrowserLauncher(ctx, "darwin", "http://127.0.0.1:51234")
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > 2*time.Second {
		t.Fatalf("open completed in %s with %v, want prompt timeout", time.Since(started), err)
	}
}

func TestBrowserLauncherLeavesXDGFallbackRunningAfterCancellation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("SLATE_TEST_BROWSER_DIR", dir)
	writeBrowserLauncher(t, dir, "xdg-open", `
printf 'started' > "$SLATE_TEST_BROWSER_DIR/started"
i=0
while [ ! -f "$SLATE_TEST_BROWSER_DIR/continue" ]; do
  [ "$i" -lt 150 ] || exit 1
  i=$((i + 1))
  /bin/sleep 0.02
done
printf 'finished' > "$SLATE_TEST_BROWSER_DIR/finished"
`)
	release := filepath.Join(dir, "continue")
	t.Cleanup(func() { _ = os.WriteFile(release, nil, 0o600) })
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- runBrowserLauncher(ctx, "linux", "http://127.0.0.1:51234") }()
	waitForLauncherFile(t, filepath.Join(dir, "started"))

	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("opening canceled with %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("canceled request kept waiting for xdg-open")
	}
	if err := os.WriteFile(release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	waitForLauncherFile(t, filepath.Join(dir, "finished"))
}

func writeBrowserLauncher(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
}

func waitForLauncherFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		} else if !errors.Is(err, os.ErrNotExist) || time.Now().After(deadline) {
			t.Fatalf("waiting for %s: %v", path, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
