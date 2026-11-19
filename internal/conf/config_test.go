package conf

import (
	"path/filepath"
	"testing"
)

func TestWriteConfig(t *testing.T) {
	if err := WriteConfig(Bootstrap{Server: Server{
		HTTP: ServerHTTP{
			Port: 8080,
		},
	}}, "test.toml"); err != nil {
		t.Fatal(err)
	}

	if err := WriteConfig(Bootstrap{Server: Server{
		HTTP: ServerHTTP{
			Port: 8081,
		},
	}}, "test.toml"); err != nil {
		t.Fatal(err)
	}
}

// TestDefaultDirUnderHome 默认配置目录在用户主目录的 .slate-kanban 下。
func TestDefaultDirUnderHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	got, err := DefaultDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".slate-kanban"); got != want {
		t.Fatalf("默认目录 = %q，期望 %q", got, want)
	}
}

// TestDefaultConfigDsnInConfDir 默认数据库放在配置目录下，并且是传入目录的绝对路径，不依赖启动目录。
func TestDefaultConfigDsnInConfDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".slate-kanban")
	got := DefaultConfig(dir).Data.Database.Dsn
	if want := filepath.Join(dir, "data.db"); got != want {
		t.Fatalf("Dsn = %q，期望 %q", got, want)
	}
}
