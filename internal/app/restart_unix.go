//go:build !windows

package app

import "syscall"

// restartSupported 表示类 Unix 总是支持自动重启：用 exec 原地换程序，不依赖任何守护进程。
func restartSupported() bool { return true }

// restartProcess 用 exec 把当前进程原地换成新程序：PID 不变，所以 launchd、systemd 看不出进程换过；
// 监听端口等文件描述符默认带 close-on-exec，会随之关闭，新程序可以重新监听同一个端口。
func restartProcess(exe string, args, env []string) error {
	return syscall.Exec(exe, args, env)
}
