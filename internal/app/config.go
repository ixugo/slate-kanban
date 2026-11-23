package app

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/gofrs/flock"

	"slate-kanban/internal/conf"
)

// ErrConfigInUse 表示已有 Slate 进程使用此目录的配置和数据。
var ErrConfigInUse = errors.New("已有 Slate 服务或桌面端使用此配置目录，请退出后重试")

// OpenConfig 读取或初始化配置，并持有配置目录锁，直到服务释放数据库。
// 文件保留在原处，操作系统会在进程退出时释放锁。
func OpenConfig(dir string) (conf.Bootstrap, func(), error) {
	var bc conf.Bootstrap
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return bc, nil, err
	}
	lock := flock.New(filepath.Join(dir, "slate.lock"))
	ok, err := lock.TryLock()
	if err != nil {
		return bc, nil, fmt.Errorf("锁定 Slate 配置目录: %w", err)
	}
	if !ok {
		return bc, nil, fmt.Errorf("%w: %s", ErrConfigInUse, dir)
	}
	closeConfig := sync.OnceFunc(func() {
		if err := lock.Close(); err != nil {
			slog.Error("释放 Slate 配置目录失败", "dir", dir, "err", err)
		}
	})

	path := filepath.Join(dir, "config.toml")
	if err := conf.SetupConfig(&bc, path); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			closeConfig()
			return bc, nil, err
		}
		bc = conf.DefaultConfig(dir)
		if err := conf.WriteConfig(bc, path); err != nil {
			closeConfig()
			return bc, nil, err
		}
	}
	if err := conf.ValidatePasscode(&bc); err != nil {
		closeConfig()
		return bc, nil, err
	}
	if err := conf.UpgradeConfig(&bc, path); err != nil {
		closeConfig()
		return bc, nil, fmt.Errorf("升级 Slate 配置: %w", err)
	}

	bc.Runtime.ConfigDir = dir
	bc.Runtime.ConfigPath = path
	return bc, closeConfig, nil
}
