package svcinstall

import (
	"fmt"
	"path/filepath"
)

// PlatformLaunchd 表示 macOS launchd 用户级 LaunchAgent。
const PlatformLaunchd = "launchd"

// PlatformSystemd 表示 Linux 用户级 systemd 单元。
const PlatformSystemd = "systemd"

// ServiceFilePath 根据注入的主目录和平台返回服务文件的完整路径，供安装和测试共用。
func ServiceFilePath(home, platform string) string {
	switch platform {
	case PlatformLaunchd:
		return filepath.Join(home, "Library", "LaunchAgents", launchdPlistName)
	case PlatformSystemd:
		return filepath.Join(home, ".config", "systemd", "user", systemdUnitName)
	default:
		return ""
	}
}

// DetectPlatform 按 GOOS 选择默认平台；不支持的系统返回中文错误。
func DetectPlatform(goos string) (string, error) {
	switch goos {
	case "darwin":
		return PlatformLaunchd, nil
	case "linux":
		return PlatformSystemd, nil
	default:
		return "", fmt.Errorf("当前系统 %q 不支持自动生成服务文件，请用 --platform launchd 或 --platform systemd 预览内容", goos)
	}
}

// ParsePlatformFlag 校验 --platform 白名单。
func ParsePlatformFlag(raw string) (string, error) {
	switch raw {
	case PlatformLaunchd:
		return PlatformLaunchd, nil
	case PlatformSystemd:
		return PlatformSystemd, nil
	case "":
		return "", fmt.Errorf("平台不能为空")
	default:
		return "", fmt.Errorf("不支持的 --platform 取值 %q，仅允许 launchd 或 systemd", raw)
	}
}
