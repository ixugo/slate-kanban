//go:build slate_desktop && embed_web && (darwin || linux || windows)

package main

//go:generate go run github.com/tc-hib/go-winres@v0.3.3 simply --manifest gui --arch amd64 --icon ../../desktop/icon.png --out rsrc --product-name Slate --original-filename slate_desktop.exe

import (
	"fmt"
	"os"

	"slate-kanban/desktop"
	"slate-kanban/internal/app"
)

var (
	buildVersion = "0.0.1"
	gitBranch    = "dev"
	gitHash      = "debug"
	buildTime    string
)

func main() {
	exe, err := desktop.Run(app.BuildInfo{Version: buildVersion, Branch: gitBranch, Commit: gitHash, Time: buildTime})
	if err == nil && exe != "" {
		err = app.Restart(exe)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
