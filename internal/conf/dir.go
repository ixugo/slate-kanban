package conf

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultDirName 是默认配置目录在用户主目录下的名字：配置文件和 SQLite 数据库都放在里面，
// 这样不管从哪个目录启动程序、服务文件怎么写，用的都是同一份配置和数据。
const DefaultDirName = ".slate-kanban"

// DefaultDir 返回默认配置目录的绝对路径（用户主目录下的 .slate-kanban）。
// 取不到用户主目录时返回错误，由调用方决定是退出还是让用户用 -conf 指定目录。
func DefaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("获取用户主目录失败，请用 -conf 指定配置目录: %w", err)
	}
	return filepath.Join(home, DefaultDirName), nil
}

// ResourceDirName 是资源目录在配置目录下的默认名字。
const ResourceDirName = "resource"

// ResourceDir 返回图片等资源文件的存放目录（绝对路径）：配置里写了就用配置的
// （相对路径以启动目录为基准），没写就用配置目录下的 resource。
func ResourceDir(bc *Bootstrap) (string, error) {
	dir := bc.Data.ResourceDir
	if dir == "" {
		dir = filepath.Join(bc.Runtime.ConfigDir, ResourceDirName)
	}
	if bc.Data.ResourceDir == "" && bc.Runtime.ConfigDir == "" {
		return "", fmt.Errorf("未配置资源目录，也没有配置目录可作为默认位置")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("解析资源目录失败: %w", err)
	}
	return abs, nil
}
