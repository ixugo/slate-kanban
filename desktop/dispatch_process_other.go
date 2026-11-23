//go:build !darwin && !linux

package desktop

import "os/exec"

func configureDispatchProcess(_ *exec.Cmd) {}
