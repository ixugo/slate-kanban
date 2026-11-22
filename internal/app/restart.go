package app

import (
	"log/slog"
	"os"
)

// newRestartRequester 返回「请求重启」函数和接收重启请求的通道。
// 函数只负责把新程序路径投进通道（不阻塞），真正的关闭和重启由 Run 完成：
// 要等正在处理的请求（包括发出这个请求的更新接口自己）返回后，才关闭服务、再换成新程序。
func newRestartRequester() (func(exe string) bool, <-chan string) {
	ch := make(chan string, 1)
	return func(exe string) bool {
		if !restartSupported() {
			return false
		}
		select {
		case ch <- exe:
		default: // 已经有一次重启排队，重复请求直接并入
		}
		return true
	}, ch
}

// Restart 用 exe 指向的程序替换（或接替）当前进程，沿用当前的启动参数和环境变量。
// 必须在 Run 返回之后调用：此时端口已释放、数据库和日志已关闭。
// 成功时类 Unix 不会返回（进程被原地换成新程序，PID 不变，守护进程无感）；失败时返回错误。
func Restart(exe string) error {
	slog.Info("<-restart", "exe", exe)
	return restartProcess(exe, os.Args, os.Environ())
}
