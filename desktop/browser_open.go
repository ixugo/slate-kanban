package desktop

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

func openDesktopBrowser(ctx context.Context, target string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return runBrowserLauncher(ctx, runtime.GOOS, target)
}

func runBrowserLauncher(ctx context.Context, platform, target string) error {
	var cmd *exec.Cmd
	switch platform {
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", target)
	case "windows":
		cmd = exec.CommandContext(ctx, "rundll32.exe", "url.dll,FileProtocolHandler", target)
	case "linux":
		gio, err := exec.LookPath("gio")
		if err != nil {
			return openWithXDG(ctx, target)
		}
		cmd = exec.CommandContext(ctx, gio, "open", target)
	default:
		return fmt.Errorf("opening the default browser is unavailable on %s", platform)
	}
	err := cmd.Run()
	if err != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func openWithXDG(ctx context.Context, target string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// xdg-open can become the browser process. Stop waiting at the deadline
	// and let Wait reap the process when it eventually exits.
	cmd := exec.Command("xdg-open", target)
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
