package svcinstall

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// commandTimeout 是单条 launchctl / systemctl 命令的超时，防止系统服务管理器卡住时 slate -d 一直挂着。
const commandTimeout = 30 * time.Second

// bootstrapRetries 是 launchctl bootstrap 的最多尝试次数。
// bootout 之后 launchd 要过一会儿才算真正卸载完，马上 bootstrap 可能报 I/O 错误，需要稍等重试。
const bootstrapRetries = 5

// retryDelay 是 bootstrap 重试之间的等待时间，做成变量是为了测试里置零。
var retryDelay = 500 * time.Millisecond

// defaultURL 是默认配置下的访问地址，仅用于启动成功后的提示；用户改过端口时以配置文件为准。
const defaultURL = "http://127.0.0.1:7070"

// Runner 执行一条外部命令并返回合并后的输出；测试里换成记录命令的假实现。
type Runner func(name string, args ...string) (string, error)

// execRunner 真正调用系统命令，带超时。
func execRunner(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// run 通过 Env 里注入的 Runner 执行命令；没注入就用真实命令。失败时把命令输出带进错误，方便定位。
func (e *Env) run(name string, args ...string) error {
	r := e.Run
	if r == nil {
		r = execRunner
	}
	out, err := r(name, args...)
	if err != nil {
		return fmt.Errorf("%s %s 失败: %w: %s", name, strings.Join(args, " "), err, out)
	}
	return nil
}

// launchdLoaded 判断 LaunchAgent 是否已经加载进当前用户的 launchd。
func (e *Env) launchdLoaded() bool {
	return e.run("launchctl", "print", e.launchdTarget()) == nil
}

// launchdDomain 是当前用户图形会话的 launchd 域。
func (e *Env) launchdDomain() string {
	return fmt.Sprintf("gui/%d", e.UID)
}

// launchdTarget 是这个服务在 launchd 里的完整标识。
func (e *Env) launchdTarget() string {
	return e.launchdDomain() + "/" + launchdLabel
}

// StartService 写好服务文件，再加载并启动；已经在运行时重启它，让新的程序或配置目录生效。
// 重复执行是安全的：每次都落到「服务在运行」这个目标状态。
func StartService(env *Env, platform, confFlag string) (path string, err error) {
	path, _, wr, err := InstallService(env, platform, confFlag, false)
	if err != nil {
		return path, err
	}
	switch platform {
	case PlatformLaunchd:
		return path, env.startLaunchd(path, wr)
	case PlatformSystemd:
		return path, env.startSystemd()
	}
	return path, fmt.Errorf("不支持的平台 %q", platform)
}

// startLaunchd 在服务文件没变且已加载时只重启进程；否则先卸载旧定义，再加载新的。
func (e *Env) startLaunchd(path string, wr writeResult) error {
	loaded := e.launchdLoaded()
	if loaded && wr == writeUnchanged {
		return e.run("launchctl", "kickstart", "-k", e.launchdTarget())
	}
	if loaded {
		if err := e.run("launchctl", "bootout", e.launchdTarget()); err != nil {
			return err
		}
	}
	var err error
	for i := 0; i < bootstrapRetries; i++ {
		if err = e.run("launchctl", "bootstrap", e.launchdDomain(), path); err == nil {
			return nil
		}
		time.Sleep(retryDelay)
	}
	return err
}

// startSystemd 重新读取单元、设为登录自启并重启，三步都成功才算启动。
func (e *Env) startSystemd() error {
	for _, args := range [][]string{
		{"--user", "daemon-reload"},
		{"--user", "enable", systemdUnitName},
		{"--user", "restart", systemdUnitName},
	} {
		if err := e.run("systemctl", args...); err != nil {
			return err
		}
	}
	return nil
}

// StopService 停掉服务进程并删除服务文件，之后不再自启。
// 服务本来就没注册时什么也不做，返回 existed=false。
func StopService(env *Env, platform string) (path string, existed bool, err error) {
	path = ServiceFilePath(env.HomeDir, platform)
	existed = fileExists(path)
	switch platform {
	case PlatformLaunchd:
		if env.launchdLoaded() {
			if err = env.run("launchctl", "bootout", env.launchdTarget()); err != nil {
				return path, existed, err
			}
			existed = true
		}
	case PlatformSystemd:
		if existed {
			if err = env.run("systemctl", "--user", "disable", "--now", systemdUnitName); err != nil {
				return path, existed, err
			}
		}
	}
	if _, _, err = UninstallService(env, platform, false); err != nil {
		return path, existed, err
	}
	if platform == PlatformSystemd && existed {
		err = env.run("systemctl", "--user", "daemon-reload")
	}
	return path, existed, err
}

// RunDaemonFromMain 是 main 在发现 -d 或 -stop 时的入口，处理完直接 os.Exit。
func RunDaemonFromMain(stop bool, confFlag string) {
	env, err := DefaultEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化环境失败: %v\n", err)
		os.Exit(1)
	}
	os.Exit(RunDaemon(os.Stdout, os.Stderr, env, stop, confFlag))
}

// RunDaemon 执行 -d（注册并启动服务）或 -stop（停止并删除服务），返回退出码。
func RunDaemon(stdout, stderr io.Writer, env *Env, stop bool, confFlag string) int {
	platform, err := DetectPlatform(env.GOOS)
	if err != nil {
		fmt.Fprintf(stderr, "当前系统 %q 不支持 -d 和 -stop（仅 macOS 和 Linux），请直接运行 slate\n", env.GOOS)
		return 1
	}
	if stop {
		return runStop(stdout, stderr, env, platform)
	}
	if confFlag != "" {
		if err := validateConfArg(confFlag); err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
	}
	path, err := StartService(env, platform, confFlag)
	if err != nil {
		fmt.Fprintf(stderr, "启动服务失败: %v\n", err)
		return 1
	}
	fmt.Fprint(stdout, formatStarted(env, platform, path, confFlag))
	return 0
}

// runStop 执行 -stop 并输出结果。
func runStop(stdout, stderr io.Writer, env *Env, platform string) int {
	path, existed, err := StopService(env, platform)
	if err != nil {
		fmt.Fprintf(stderr, "停止服务失败: %v\n", err)
		return 1
	}
	if !existed {
		fmt.Fprintf(stdout, "没有已注册的 Slate 服务，无需停止: %s\n", path)
		return 0
	}
	fmt.Fprintf(stdout, "已停止 Slate 服务，并删除服务文件: %s\n", path)
	return 0
}

// formatStarted 拼装启动成功后的提示：访问地址、日志位置、停止方法。
func formatStarted(env *Env, platform, path, confFlag string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Slate 已注册为服务并启动: %s\n", path)
	fmt.Fprintf(&b, "访问地址: %s（默认端口，以配置文件为准）\n", defaultURL)
	if platform == PlatformLaunchd {
		logDir := LogDir(env.ServiceInput(confFlag).ConfDir)
		fmt.Fprintf(&b, "查看日志: tail -f %s\n", shellQuote(filepath.Join(logDir, stdoutLogName)))
	} else {
		fmt.Fprintf(&b, "查看日志: journalctl --user -u %s -f\n", systemdUnitName)
		fmt.Fprintf(&b, "想在没人登录时也开机自启，执行一次: loginctl enable-linger %s\n", env.Username)
	}
	fmt.Fprintln(&b, "停止并删除服务: slate -stop")
	return b.String()
}
