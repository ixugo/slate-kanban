package app

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestOpenConfigHoldsOneOwnerPerDirectory(t *testing.T) {
	dir := t.TempDir()
	bc, closeConfig, err := OpenConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeConfig)
	if bc.Runtime.ConfigPath != filepath.Join(dir, "config.toml") ||
		bc.Data.Database.Dsn != filepath.Join(dir, "data.db") {
		t.Fatalf("未使用共享配置目录的默认路径: %+v", bc)
	}

	if _, _, err := OpenConfig(dir); !errors.Is(err, ErrConfigInUse) {
		t.Fatalf("同一目录应拒绝并发启动，实际: %v", err)
	}
	_, closeOther, err := OpenConfig(t.TempDir())
	if err != nil {
		t.Fatalf("单独指定配置目录应可启动: %v", err)
	}
	t.Cleanup(closeOther)

	closeConfig()
	_, closeAgain, err := OpenConfig(dir)
	if err != nil {
		t.Fatalf("释放后可以再次读取这份配置: %v", err)
	}
	t.Cleanup(closeAgain)
}
