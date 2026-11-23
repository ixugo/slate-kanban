package conf

import (
	"path/filepath"
	"testing"
)

func TestWriteConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := WriteConfig(Bootstrap{Server: Server{
		HTTP: ServerHTTP{
			Port: 8080,
		},
	}}, path); err != nil {
		t.Fatal(err)
	}

	if err := WriteConfig(Bootstrap{Server: Server{
		HTTP: ServerHTTP{
			Port: 8081,
		},
	}}, path); err != nil {
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

func TestDefaultConfigLeavesDsnEmpty(t *testing.T) {
	if got := DefaultConfig(t.TempDir()).Data.Database.Dsn; got != "" {
		t.Fatalf("生成配置的 Dsn = %q，期望为空", got)
	}
}

// TestResourceDir 资源目录：配置了就用配置的；没配置就落在配置目录的 resource 下；两者都没有时报错。
func TestResourceDir(t *testing.T) {
	dir := t.TempDir()
	custom := filepath.Join(dir, "pics")
	if got, err := ResourceDir(&Bootstrap{Data: Data{ResourceDir: custom}}); err != nil || got != custom {
		t.Fatalf("配置了目录时得到 %q, %v，期望 %q", got, err, custom)
	}
	bc := &Bootstrap{Runtime: Runtime{ConfigDir: dir}}
	if got, err := ResourceDir(bc); err != nil || got != filepath.Join(dir, "resource") {
		t.Fatalf("没配置时得到 %q, %v，期望配置目录下的 resource", got, err)
	}
	if _, err := ResourceDir(&Bootstrap{}); err == nil {
		t.Fatal("两者都没有时应该报错")
	}
	if got := DefaultConfig(dir).Data.ResourceDir; got != filepath.Join(dir, "resource") {
		t.Fatalf("默认资源目录 = %q", got)
	}
}
