package main

import (
	"path/filepath"
	"testing"
)

// TestResolveConfigDir 不指定目录时用 ~/.slate-kanban，指定相对路径时转成启动目录下的绝对路径。
func TestResolveConfigDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	got, err := resolveConfigDir("")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".slate-kanban"); got != want {
		t.Fatalf("默认目录 = %q，期望 %q", got, want)
	}

	got, err = resolveConfigDir("./custom")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(got) || filepath.Base(got) != "custom" {
		t.Fatalf("相对路径应转成绝对路径，得到 %q", got)
	}
}
