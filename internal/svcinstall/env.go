package svcinstall

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Env 承载安装时可注入的路径和系统信息，测试里用临时目录代替真实主目录。
type Env struct {
	HomeDir    string
	WorkDir    string
	Executable string
	UID        int
	Username   string
	GOOS       string
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

// ServiceInput 把相对配置目录转成绝对路径，并与工作目录、可执行路径一起返回。
func (e *Env) ServiceInput(confFlag string) ServiceInput {
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
