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
