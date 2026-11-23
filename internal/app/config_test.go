package app

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"slate-kanban/internal/conf"
)

func TestOpenConfigHoldsOneOwnerPerDirectory(t *testing.T) {
	dir := t.TempDir()
	bc, closeConfig, err := OpenConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeConfig)
	if bc.Runtime.ConfigPath != filepath.Join(dir, "config.toml") ||
		bc.Data.Database.Dsn != "" || bc.ConfigVersion != conf.CurrentConfigVersion {
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

func TestOpenConfigUpgradesLegacyDSNOnce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	defaultDatabase := filepath.Join(home, conf.DefaultDirName, "data.db")
	cases := []struct {
		name string
		dsn  string
		want string
	}{
		{name: "missing", want: defaultDatabase},
		{name: "empty", dsn: "dsn = ''\n", want: defaultDatabase},
		{name: "relative", dsn: "dsn = 'old.db'\n", want: defaultDatabase},
		{name: "absolute", dsn: "dsn = '/old/slate.db'\n", want: defaultDatabase},
		{name: "postgres", dsn: "dsn = 'postgres://db.example/slate'\n", want: "postgres://db.example/slate"},
		{name: "postgres-uppercase", dsn: "dsn = 'POSTGRES://db.example/slate'\n", want: "POSTGRES://db.example/slate"},
		{name: "postgresql", dsn: "dsn = 'postgresql://db.example/slate'\n", want: defaultDatabase},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.toml")
			raw := "[Server.HTTP]\nPort = 7071\n[data.database]\n" + tc.dsn + "\n[Extension]\nEnabled = true\n"
			if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
				t.Fatal(err)
			}
			bc, closeConfig, err := OpenConfig(dir)
			if err != nil {
				t.Fatal(err)
			}
			closeConfig()
			if bc.Data.Database.Dsn != tc.want || bc.ConfigVersion != conf.CurrentConfigVersion || bc.Server.HTTP.Port != 7071 {
				t.Fatalf("运行配置未正确升级: %+v", bc)
			}

			var saved struct {
				conf.Bootstrap
				Extension struct{ Enabled bool }
			}
			if err := conf.SetupConfig(&saved, path); err != nil {
				t.Fatal(err)
			}
			if saved.Data.Database.Dsn != tc.want || saved.ConfigVersion != conf.CurrentConfigVersion || !saved.Extension.Enabled {
				t.Fatalf("持久化配置未正确升级: %+v", saved)
			}
			if runtime.GOOS != "windows" {
				info, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				if got := info.Mode().Perm(); got != 0o600 {
					t.Fatalf("权限 = %o，期望 600", got)
				}
			}

			saved.Data.Database.Dsn = "later.db"
			if err := conf.WriteConfig(saved.Bootstrap, path); err != nil {
				t.Fatal(err)
			}
			bc, closeAgain, err := OpenConfig(dir)
			if err != nil {
				t.Fatal(err)
			}
			closeAgain()
			if bc.Data.Database.Dsn != "later.db" {
				t.Fatalf("重启覆盖了迁移后填写的相对路径: %+v", bc)
			}
		})
	}
	if _, err := os.Stat(defaultDatabase); !os.IsNotExist(err) {
		t.Fatalf("迁移配置不应创建数据库: %v", err)
	}
}

func TestOpenConfigReleasesLockWhenLegacyUpgradeFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	const legacy = "[Data.Database]\nDsn = 'old.db'\n"
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	if _, cleanup, err := OpenConfig(dir); err == nil {
		cleanup()
		t.Fatal("未找到用户主目录时应停止升级")
	}
	saved, err := os.ReadFile(path)
	if err != nil || string(saved) != legacy {
		t.Fatalf("升级失败后配置应保持原样: %v", err)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if _, cleanup, err := OpenConfig(dir); err != nil {
		t.Fatalf("修复环境后应能重新获得配置锁: %v", err)
	} else {
		cleanup()
	}
}
