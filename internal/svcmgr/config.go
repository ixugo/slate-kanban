// Package svcmgr 基于 kardianos/service 把 Slate 注册为系统服务，
// 支持 Windows 服务、Linux systemd 和 macOS launchd，对应命令行的 -d 与 -stop。
package svcmgr

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kardianos/service"

	"slate-kanban/internal/conf"
)

const (
	// serviceName 是 Windows 服务名和 Linux systemd 单元名（slate.service）。
	serviceName = "slate"
	// darwinServiceName 是 macOS launchd 的 Label，同时也是 plist 文件名。
	darwinServiceName = "dev.slate.kanban"
	// maxConfArgLen 是 -conf 路径的长度上限，防止异常长的输入写进服务文件。
	maxConfArgLen = 4096
	// defaultURL 是默认配置下的访问地址，仅用于启动成功后的提示；用户改过端口时以配置文件为准。
	defaultURL = "http://127.0.0.1:7070"
)

// Env 是注册服务所需的环境信息，测试里可以直接构造。
type Env struct {
	HomeDir    string // 运行服务的用户主目录，默认配置目录放在它下面
	WorkDir    string // 执行 -d 时所在目录，作为服务的工作目录
	Executable string // slate 可执行文件的绝对路径
	User       string // Linux 系统服务以此用户身份运行
	GOOS       string
}

// confDir 把 -conf 转成绝对路径；为空时用主目录下的默认目录，相对路径以工作目录为基准。
func (e Env) confDir(confFlag string) string {
	if confFlag == "" {
		return filepath.Join(e.HomeDir, conf.DefaultDirName)
	}
	if filepath.IsAbs(confFlag) {
		return filepath.Clean(confFlag)
	}
	return filepath.Join(e.WorkDir, confFlag)
}

// logDir 是配置目录下的日志目录；macOS 的 launchd 把标准输出写在这里，程序自己的日志也在这里。
func logDir(confDir string) string {
	return filepath.Join(confDir, "logs")
}

// buildConfig 生成 kardianos/service 的配置：
// 参数里固定写绝对的 -conf，服务方式和直接运行用同一份数据；
// macOS 用用户级 LaunchAgent（登录后启动），Linux 用系统级 systemd（开机即启，以 Env.User 身份运行），
// Windows 用系统服务（开机自动启动）。
func buildConfig(env Env, confFlag string) service.Config {
	dir := env.confDir(confFlag)
	cfg := service.Config{
		Name:             serviceName,
		DisplayName:      "Slate Kanban",
		Description:      "Slate Kanban 看板服务",
		Executable:       env.Executable,
		Arguments:        []string{"-conf", dir},
		WorkingDirectory: env.WorkDir,
		Option: service.KeyValue{
			"KeepAlive": true,
			"RunAtLoad": true,
			"Restart":   "on-failure",
		},
	}
	switch env.GOOS {
	case "darwin":
		cfg.Name = darwinServiceName
		cfg.Option["UserService"] = true
		cfg.Option["LogDirectory"] = logDir(dir)
	case "linux":
		cfg.UserName = env.User
	}
	return cfg
}

// validateConfArg 校验 -conf 的取值：不能为空、不能过长、不能含换行等控制字符，
// 否则会把服务文件写坏，甚至多出一行不该有的配置。
func validateConfArg(v string) error {
	if strings.TrimSpace(v) == "" {
		return fmt.Errorf("-conf 不能是空字符串")
	}
	if len(v) > maxConfArgLen {
		return fmt.Errorf("-conf 路径过长，最多 %d 个字节", maxConfArgLen)
	}
	for _, r := range v {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("-conf 路径里不能有换行或其他控制字符")
		}
	}
	return nil
}
