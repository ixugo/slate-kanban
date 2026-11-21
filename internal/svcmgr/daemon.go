package svcmgr

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/kardianos/service"
)

// RunDaemonFromMain 是 main 在发现 -d 或 -stop 时的入口，处理完直接 os.Exit。
// Linux 上不是 root 时先转交给 sudo。
func RunDaemonFromMain(stop bool, confFlag string) {
	if runtime.GOOS == "linux" && os.Geteuid() != 0 {
		os.Exit(rerunWithSudo())
	}
	env, err := DefaultEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化环境失败: %v\n", err)
		os.Exit(1)
	}
	os.Exit(RunDaemon(os.Stdout, os.Stderr, env, stop, confFlag))
}

// RunDaemon 执行 -d（注册并启动服务）或 -stop（停止并删除服务），返回退出码。
func RunDaemon(stdout, stderr io.Writer, env Env, stop bool, confFlag string) int {
	if confFlag != "" {
		if err := validateConfArg(confFlag); err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
	}
	cfg := buildConfig(env, confFlag)
	svc, err := service.New(&Program{}, &cfg)
	if err != nil {
		fmt.Fprintf(stderr, "当前系统不支持注册服务: %v\n", err)
		return 1
	}
	if stop {
		return runStop(stdout, stderr, svc)
	}
	return runStart(stdout, stderr, svc, env, env.confDir(confFlag))
}

// runStart 先清掉旧的注册再重新注册并启动，所以重复执行 -d 总会落到「用当前程序和配置运行」。
func runStart(stdout, stderr io.Writer, svc service.Service, env Env, confDir string) int {
	if err := removeService(svc); err != nil {
		fmt.Fprintf(stderr, "清理旧服务失败: %v\n%s", err, adminHint(env.GOOS))
		return 1
	}
	if env.GOOS == "darwin" {
		// launchd 打开标准输出文件时不会替我们建目录，目录不存在会直接启动失败
		if err := os.MkdirAll(logDir(confDir), 0o755); err != nil {
			fmt.Fprintf(stderr, "创建日志目录失败: %v\n", err)
			return 1
		}
	}
	if err := svc.Install(); err != nil {
		fmt.Fprintf(stderr, "注册服务失败: %v\n%s", err, adminHint(env.GOOS))
		return 1
	}
	if err := svc.Start(); err != nil {
		fmt.Fprintf(stderr, "启动服务失败: %v\n", err)
		return 1
	}
	fmt.Fprint(stdout, startedMessage(env.GOOS, confDir))
	return 0
}

// runStop 停止并删除服务；本来就没注册时直接成功。
func runStop(stdout, stderr io.Writer, svc service.Service) int {
	if _, err := svc.Status(); errors.Is(err, service.ErrNotInstalled) {
		fmt.Fprintln(stdout, "没有已注册的 Slate 服务，无需停止")
		return 0
	}
	if err := removeService(svc); err != nil {
		fmt.Fprintf(stderr, "停止服务失败: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "已停止 Slate 服务")
	return 0
}

// removeService 在服务已注册时先停止再删除；没注册过什么也不做。
// systemd 的 Uninstall 只 disable 不 stop，所以运行中的服务要自己先停。
func removeService(svc service.Service) error {
	status, err := svc.Status()
	if errors.Is(err, service.ErrNotInstalled) {
		return nil
	}
	if status == service.StatusRunning {
		if err := svc.Stop(); err != nil {
			return err
		}
	}
	return svc.Uninstall()
}

// adminHint 权限不足时的补充提示，Windows 上要用管理员身份打开终端。
func adminHint(goos string) string {
	if goos == "windows" {
		return "提示：注册 Windows 服务需要管理员权限，请用「以管理员身份运行」打开命令提示符后重试\n"
	}
	return ""
}

// startedMessage 拼装启动成功后的提示：访问地址、macOS 日志位置、停止方法。
func startedMessage(goos, confDir string) string {
	msg := fmt.Sprintf("访问地址: %s（默认端口，以配置文件为准）\n", defaultURL)
	if goos == "darwin" {
		msg += fmt.Sprintf("查看日志: tail -f %s\n", logDir(confDir))
	}
	return msg + "停止并删除服务: slate -stop\n"
}
