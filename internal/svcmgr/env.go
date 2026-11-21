package svcmgr

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	osuser "os/user"
	"path/filepath"
	"runtime"
)

// DefaultEnv 用当前进程信息构造环境。
// Linux 上用 sudo 提权运行时，主目录和运行用户取 sudo 发起者，这样配置、数据库不会落到 /root，服务也不以 root 运行。
func DefaultEnv() (Env, error) {
	work, err := os.Getwd()
	if err != nil {
		return Env{}, fmt.Errorf("获取工作目录失败: %w", err)
	}
	exe, err := resolveExecutable()
	if err != nil {
		return Env{}, err
	}
	u, err := targetUser()
	if err != nil {
		return Env{}, err
	}
	return Env{HomeDir: u.HomeDir, WorkDir: work, Executable: exe, User: u.Username, GOOS: runtime.GOOS}, nil
}

// targetUser 返回服务归属的用户：sudo 提权时是发起者，否则是当前用户。
func targetUser() (*osuser.User, error) {
	if name := os.Getenv("SUDO_USER"); runtime.GOOS == "linux" && os.Geteuid() == 0 && name != "" && name != "root" {
		u, err := osuser.Lookup(name)
		if err != nil {
			return nil, fmt.Errorf("查询 sudo 发起用户 %q 失败: %w", name, err)
		}
		return u, nil
	}
	u, err := osuser.Current()
	if err != nil {
		return nil, fmt.Errorf("获取当前用户失败: %w", err)
	}
	return u, nil
}

// resolveExecutable 取当前二进制绝对路径并解析符号链接，服务文件里要写真实路径。
func resolveExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("获取可执行文件路径失败: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("解析可执行文件符号链接失败: %w", err)
	}
	return filepath.Abs(exe)
}

// rerunWithSudo 用 sudo 重新执行当前命令（Linux 系统服务要写 /etc 并调用 systemctl，必须 root），返回退出码。
// 标准输入输出直接接给终端，这样 sudo 能提示输入密码。
func rerunWithSudo() int {
	sudo, err := exec.LookPath("sudo")
	if err != nil {
		fmt.Fprintln(os.Stderr, "安装系统级服务需要 root 权限，但没找到 sudo，请切换到 root 后再执行")
		return 1
	}
	exe, err := resolveExecutable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	fmt.Fprintln(os.Stderr, "系统级服务需要 root 权限，将通过 sudo 执行")
	cmd := exec.Command(sudo, append([]string{exe}, os.Args[1:]...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "执行 sudo 失败: %v\n", err)
		return 1
	}
	return 0
}
