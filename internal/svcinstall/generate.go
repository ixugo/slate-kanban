package svcinstall

import (
	"strings"
)

// ServiceInput 生成服务文件时需要的三个绝对路径：可执行文件、配置目录（配置、数据库、日志都在里面）、工作目录。
type ServiceInput struct {
	Executable string
	ConfDir    string
	WorkDir    string
}

// GenerateLaunchdPlist 生成 macOS LaunchAgent 的 plist XML；ProgramArguments 逐项列出，避免 shell 转义问题。
// RunAtLoad 让加载（或下次登录）时立刻启动，KeepAlive 让进程退出后自动拉起。
func GenerateLaunchdPlist(in ServiceInput) string {
	stdout := LogDir(in.ConfDir) + "/" + stdoutLogName
	stderr := LogDir(in.ConfDir) + "/" + stderrLogName
	lines := []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">`,
		`<plist version="1.0">`,
		`<dict>`,
		`	<key>Label</key>`,
		`	<string>` + xmlEscape(launchdLabel) + `</string>`,
		`	<key>ProgramArguments</key>`,
		`	<array>`,
		`		<string>` + xmlEscape(in.Executable) + `</string>`,
		`		<string>-conf</string>`,
		`		<string>` + xmlEscape(in.ConfDir) + `</string>`,
		`	</array>`,
		`	<key>WorkingDirectory</key>`,
		`	<string>` + xmlEscape(in.WorkDir) + `</string>`,
		`	<key>RunAtLoad</key>`,
		`	<true/>`,
		`	<key>KeepAlive</key>`,
		`	<true/>`,
		`	<key>StandardOutPath</key>`,
		`	<string>` + xmlEscape(stdout) + `</string>`,
		`	<key>StandardErrorPath</key>`,
		`	<string>` + xmlEscape(stderr) + `</string>`,
		`</dict>`,
		`</plist>`,
		``,
	}
	return strings.Join(lines, "\n")
}

// GenerateSystemdUnit 生成 Linux 用户级 systemd 单元；标准输出错误走 journal，不单独配日志文件。
// WorkingDirectory 取等号后面的整段当路径，路径里有空格也不加引号（加了引号会变成路径的一部分），
// 只需要把 % 写成 %%，因为 systemd 会把 %h 这样的占位符展开。
func GenerateSystemdUnit(in ServiceInput) string {
	lines := []string{
		"[Unit]",
		"Description=Slate Kanban 看板服务",
		"After=network.target",
		"",
		"[Service]",
		"Type=simple",
		"WorkingDirectory=" + strings.ReplaceAll(in.WorkDir, "%", "%%"),
		"ExecStart=" + systemdExecArg(in.Executable) + " -conf " + systemdExecArg(in.ConfDir),
		"Restart=on-failure",
		"",
		"[Install]",
		"WantedBy=default.target",
		"",
	}
	return strings.Join(lines, "\n")
}

// systemdExecArg 把一个路径写成 ExecStart 里的单个参数：
// 全是安全字符时原样输出，否则用双引号包住，并转义反斜杠、双引号、%（占位符）和 $（变量展开）。
func systemdExecArg(s string) string {
	if strings.Trim(s, plainPathChars) == "" {
		return s
	}
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "%", "%%", "$", "$$")
	return `"` + r.Replace(s) + `"`
}

// xmlEscape 把字符串放进 plist 的 <string> 时做 XML 转义。
func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return r.Replace(s)
}
