package svcmgr

import (
	"path/filepath"
	"strings"
	"testing"
)

// testEnv 构造一份固定环境，路径都是假的，只用来检查生成的配置。
func testEnv(goos string) Env {
	return Env{
		HomeDir:    "/home/alice",
		WorkDir:    "/srv/slate",
		Executable: "/srv/slate/slate",
		User:       "alice",
		GOOS:       goos,
	}
}

// TestBuildConfigLinux Linux 是系统级服务：以指定用户运行，参数里是绝对的 -conf。
func TestBuildConfigLinux(t *testing.T) {
	cfg := buildConfig(testEnv("linux"), "")
	if cfg.Name != "slate" || cfg.UserName != "alice" {
		t.Fatalf("名称或用户不符: %+v", cfg)
	}
	want := []string{"-conf", filepath.Join("/home/alice", ".slate-kanban")}
	if strings.Join(cfg.Arguments, "|") != strings.Join(want, "|") {
		t.Fatalf("参数不符: %v", cfg.Arguments)
	}
	if cfg.Option["UserService"] != nil {
		t.Fatal("Linux 不应是用户级服务")
	}
}

// TestBuildConfigDarwin macOS 是用户级 LaunchAgent，日志写进配置目录的 logs。
func TestBuildConfigDarwin(t *testing.T) {
	cfg := buildConfig(testEnv("darwin"), "conf")
	if cfg.Name != "dev.slate.kanban" || cfg.UserName != "" {
		t.Fatalf("名称或用户不符: %+v", cfg)
	}
	if cfg.Option["UserService"] != true || cfg.Option["RunAtLoad"] != true {
		t.Fatalf("应是登录即启的用户级服务: %v", cfg.Option)
	}
	if cfg.Option["LogDirectory"] != "/srv/slate/conf/logs" {
		t.Fatalf("日志目录不符: %v", cfg.Option["LogDirectory"])
	}
}

// TestBuildConfigWindows Windows 不指定运行账户，使用服务默认账户。
func TestBuildConfigWindows(t *testing.T) {
	cfg := buildConfig(testEnv("windows"), "")
	if cfg.UserName != "" || cfg.Name != "slate" {
		t.Fatalf("Windows 配置不符: %+v", cfg)
	}
}

// TestValidateConfArg 空值、全空白、过长、含控制字符都应被拒绝。
func TestValidateConfArg(t *testing.T) {
	for _, bad := range []string{"", "   ", strings.Repeat("a", maxConfArgLen+1), "a\nb", "a\x00b"} {
		if err := validateConfArg(bad); err == nil {
			t.Fatalf("%q 应被拒绝", bad)
		}
	}
	if err := validateConfArg("/ok/dir"); err != nil {
		t.Fatal(err)
	}
}
