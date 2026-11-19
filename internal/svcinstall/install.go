package svcinstall

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// writeResult 描述写服务文件的结果，便于向用户输出「已安装 / 已更新 / 已经是最新」。
type writeResult int

const (
	writeCreated writeResult = iota
	writeUpdated
	writeUnchanged
)

// InstallService 生成内容并写入用户级服务文件；dryRun 时只返回内容和路径，不写盘。
func InstallService(env *Env, platform, confFlag string, dryRun bool) (targetPath, content string, wr writeResult, err error) {
	content = env.ContentForPlatform(platform, confFlag)
	targetPath = ServiceFilePath(env.HomeDir, platform)
	if dryRun {
		return targetPath, content, writeCreated, nil
	}
	if platform == PlatformLaunchd {
		// launchd 打开标准输出文件时不会替我们建目录，目录不存在会直接启动失败
		if err = os.MkdirAll(LogDir(env.ServiceInput(confFlag).ConfDir), 0o755); err != nil {
			return targetPath, content, writeCreated, fmt.Errorf("创建日志目录失败: %w", err)
		}
	}
	wr, err = writeServiceFile(targetPath, content)
	return targetPath, content, wr, err
}

// writeServiceFile 写入或跳过写入；内容相同则不覆盖。
func writeServiceFile(path, content string) (writeResult, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return writeCreated, fmt.Errorf("创建服务文件目录失败: %w", err)
	}
	existing, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return writeCreated, fmt.Errorf("读取已有服务文件失败: %w", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return writeCreated, fmt.Errorf("写入服务文件失败: %w", err)
		}
		return writeCreated, nil
	}
	if string(existing) == content {
		return writeUnchanged, nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return writeUpdated, fmt.Errorf("更新服务文件失败: %w", err)
	}
	return writeUpdated, nil
}

// UninstallService 删除服务文件；不存在视为已成功卸载。
func UninstallService(env *Env, platform string, dryRun bool) (targetPath string, existed bool, err error) {
	targetPath = ServiceFilePath(env.HomeDir, platform)
	_, statErr := os.Stat(targetPath)
	existed = statErr == nil
	if os.IsNotExist(statErr) {
		return targetPath, false, nil
	}
	if statErr != nil {
		return targetPath, false, fmt.Errorf("检查服务文件失败: %w", statErr)
	}
	if dryRun {
		return targetPath, true, nil
	}
	if err := os.Remove(targetPath); err != nil {
		return targetPath, true, fmt.Errorf("删除服务文件失败: %w", err)
	}
	return targetPath, true, nil
}

// installCommands 列出安装后要在终端手工执行的命令和说明；程序本身不调用 launchctl 或 systemctl，
// 免得未经用户同意就改动系统状态。
func installCommands(env *Env, platform, targetPath, logDir string) string {
	var b strings.Builder
	switch platform {
	case PlatformLaunchd:
		gui := fmt.Sprintf("gui/%d", env.UID)
		fmt.Fprintf(&b, "  launchctl bootstrap %s %s\n", gui, shellQuote(targetPath))
		fmt.Fprintf(&b, "  launchctl kickstart -k %s/%s\n\n", gui, launchdLabel)
		fmt.Fprintln(&b, "说明: macOS 用户级 LaunchAgent 在你登录图形会话后才会启动，不是传统意义上一通电就起的「真·开机自启」。")
		fmt.Fprintf(&b, "查看状态: launchctl print %s/%s\n", gui, launchdLabel)
		fmt.Fprintf(&b, "查看日志: tail -f %s\n", shellQuote(filepath.Join(logDir, stdoutLogName)))
	case PlatformSystemd:
		fmt.Fprintln(&b, "  systemctl --user daemon-reload")
		fmt.Fprintf(&b, "  systemctl --user enable --now %s\n\n", systemdUnitName)
		fmt.Fprintln(&b, "若希望没人登录时也能在开机后自动拉起服务，还需要（只需配置一次）：")
		fmt.Fprintf(&b, "  loginctl enable-linger %s\n\n", env.Username)
		fmt.Fprintf(&b, "查看状态: systemctl --user status %s\n", systemdUnitName)
		fmt.Fprintf(&b, "查看日志: journalctl --user -u %s -f\n", systemdUnitName)
	}
	return b.String()
}

// uninstallCommands 列出卸载后要在终端手工执行的命令。
func uninstallCommands(env *Env, platform string) string {
	var b strings.Builder
	switch platform {
	case PlatformLaunchd:
		fmt.Fprintf(&b, "  launchctl bootout gui/%d/%s\n", env.UID, launchdLabel)
	case PlatformSystemd:
		fmt.Fprintf(&b, "  systemctl --user disable --now %s\n", systemdUnitName)
		fmt.Fprintln(&b, "  systemctl --user daemon-reload")
	}
	return b.String()
}

// FormatInstallFollowUp 拼装安装完成后的提示：先说文件是新写、更新还是已经是最新，再给手工命令。
func FormatInstallFollowUp(env *Env, platform, targetPath, logDir string, wr writeResult) string {
	var b strings.Builder
	switch wr {
	case writeUnchanged:
		fmt.Fprintf(&b, "服务文件已经是最新，未改写: %s\n\n", targetPath)
	case writeUpdated:
		fmt.Fprintf(&b, "服务文件已更新: %s\n\n", targetPath)
	default:
		fmt.Fprintf(&b, "服务文件已写入: %s\n\n", targetPath)
	}
	fmt.Fprintln(&b, "接下来请在本机终端手工执行下面的命令（Slate 不会替你执行 launchctl 或 systemctl，避免未经同意改动系统状态）：")
	fmt.Fprintln(&b)
	b.WriteString(installCommands(env, platform, targetPath, logDir))
	return b.String()
}

// FormatUninstallFollowUp 拼装卸载完成后的提示：先说文件删了还是本来就没有，再给手工命令。
func FormatUninstallFollowUp(env *Env, platform, targetPath string, hadFile bool) string {
	var b strings.Builder
	if hadFile {
		fmt.Fprintf(&b, "已删除服务文件: %s\n\n", targetPath)
	} else {
		fmt.Fprintf(&b, "未找到服务文件（没有安装过）: %s\n\n", targetPath)
	}
	fmt.Fprintln(&b, "若服务此前已加载到系统里，请手工执行（Slate 不会替你执行）：")
	fmt.Fprintln(&b)
	b.WriteString(uninstallCommands(env, platform))
	return b.String()
}

// FormatDryRunInstall 把 dry-run 时要写入的路径、内容和安装后的手工命令完整打印出来，并明确说没有写任何文件。
func FormatDryRunInstall(env *Env, platform, targetPath, logDir, content string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[dry-run] 将会写入: %s\n\n", targetPath)
	fmt.Fprintln(&b, "--- 文件内容 ---")
	fmt.Fprintln(&b, content)
	fmt.Fprintln(&b, "--- 结束 ---")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "[dry-run] 没有写任何文件。真正安装后，需要在本机终端手工执行下面的命令：")
	fmt.Fprintln(&b)
	b.WriteString(installCommands(env, platform, targetPath, logDir))
	return b.String()
}

// FormatDryRunUninstall 打印 dry-run 时会删除的路径，以及卸载后的手工命令，并明确说没有删任何文件。
func FormatDryRunUninstall(env *Env, platform, targetPath string, existed bool) string {
	var b strings.Builder
	if existed {
		fmt.Fprintf(&b, "[dry-run] 将会删除: %s\n\n", targetPath)
	} else {
		fmt.Fprintf(&b, "[dry-run] 目标文件不存在（没有安装过）: %s\n\n", targetPath)
	}
	fmt.Fprintln(&b, "[dry-run] 没有删除任何文件。若服务此前已加载到系统里，需要手工执行：")
	fmt.Fprintln(&b)
	b.WriteString(uninstallCommands(env, platform))
	return b.String()
}

// shellQuote 把路径写成可以直接粘贴进终端的形式：全是安全字符时原样输出，
// 否则用单引号包住；路径里自己的单引号要先结束引用，再用反斜杠转义这个单引号，然后重新开始引用。
func shellQuote(s string) string {
	if s != "" && strings.Trim(s, plainPathChars) == "" {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
