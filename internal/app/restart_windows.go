//go:build windows

package app

import (
	"os"
	"os/exec"

	"slate-kanban/internal/svcmgr"
)

// restartSupported 表示 Windows 上只有命令行直接运行时支持自动重启：
// 以 Windows 服务运行时，新进程会脱离服务管理器，服务状态会乱，所以交给用户手动重启服务。
func restartSupported() bool { return !svcmgr.UnderServiceManager() }

// restartProcess 在 Windows 上没有 exec，只能先拉起新进程再让当前进程退出；
// 调用时旧进程已关闭监听，新进程可以直接接着监听同一个端口。
func restartProcess(exe string, args, env []string) error {
	cmd := exec.Command(exe, args[1:]...)
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	os.Exit(0)
	return nil
}
