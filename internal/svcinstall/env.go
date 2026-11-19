package svcinstall

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"slate-kanban/internal/conf"
)

// Env 承载安装时可注入的路径和系统信息，测试里用临时目录代替真实主目录。
type Env struct {
	HomeDir    string
	WorkDir    string
	Executable string
	UID        int
	Username   string
	GOOS       string
	Run        Runner // 执行 launchctl / systemctl 的函数；为空时调用真实命令，测试里注入假实现
}

// DefaultEnv 用当前进程信息构造环境，供真实命令行入口使用。
func DefaultEnv() (*Env, error) {
	work, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("获取工作目录失败: %w", err)
	}
	exe, err := resolveExecutable()
	if err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户主目录失败: %w", err)
	}
	if override := os.Getenv("HOME"); override != "" {
		home = override
	}
	uid := os.Getuid()
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("LOGNAME")
	}
	if user == "" {
		user = "当前用户"
	}
	return &Env{
		HomeDir:    home,
		WorkDir:    work,
		Executable: exe,
		UID:        uid,
		Username:   user,
		GOOS:       runtime.GOOS,
	}, nil
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
	exe, err = filepath.Abs(exe)
	if err != nil {
		return "", fmt.Errorf("可执行文件路径转绝对路径失败: %w", err)
	}
	return exe, nil
}

// ServiceInput 把配置目录转成绝对路径，并与工作目录、可执行路径一起返回。
// confFlag 为空时用用户主目录下的默认目录（conf.DefaultDirName），相对路径以工作目录为基准。
func (e *Env) ServiceInput(confFlag string) ServiceInput {
	if confFlag == "" {
		confFlag = filepath.Join(e.HomeDir, conf.DefaultDirName)
	}
	confAbs := confFlag
	if !filepath.IsAbs(confFlag) {
		confAbs = filepath.Join(e.WorkDir, confFlag)
	}
	confAbs, err := filepath.Abs(confAbs)
	if err != nil {
		confAbs = filepath.Join(e.WorkDir, confFlag)
	}
	work, err := filepath.Abs(e.WorkDir)
	if err != nil {
		work = e.WorkDir
	}
	return ServiceInput{
		Executable: e.Executable,
		ConfDir:    confAbs,
		WorkDir:    work,
	}
}

// ContentForPlatform 按平台生成服务文件全文。
func (e *Env) ContentForPlatform(platform, confFlag string) string {
	in := e.ServiceInput(confFlag)
	switch platform {
	case PlatformLaunchd:
		return GenerateLaunchdPlist(in)
	case PlatformSystemd:
		return GenerateSystemdUnit(in)
	default:
		return ""
	}
}

// LogDir 返回配置目录下存放日志的目录；launchd 托管进程的标准输出和错误也写在这里，
// 和程序自己的日志放在一起，换启动目录或改成服务方式都不会丢。
func LogDir(confDir string) string {
	return filepath.Join(confDir, "logs")
}
