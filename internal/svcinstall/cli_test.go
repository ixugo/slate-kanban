package svcinstall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tempHome 返回测试专用「主目录」，避免写到真实的 LaunchAgents 或 systemd 目录。
func tempHome(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// fixedEnv 构造可注入的安装环境，所有路径都在临时目录下。
func fixedEnv(t *testing.T, home string) *Env {
	t.Helper()
	work := filepath.Join(home, "proj")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(work, "configs")
	if err := os.MkdirAll(conf, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(work, "slate")
	if err := os.WriteFile(exe, []byte{0}, 0o755); err != nil {
		t.Fatal(err)
	}
	return &Env{
		HomeDir:    home,
		WorkDir:    work,
		Executable: exe,
		UID:        501,
		Username:   "testuser",
		GOOS:       "darwin",
	}
}

// TestPrintWritesNoFiles print 只往标准输出写内容，不能在用户服务目录留下文件。
func TestPrintWritesNoFiles(t *testing.T) {
	home := tempHome(t)
	env := fixedEnv(t, home)
	out, err := RunCommand(env, []string{"print", "--platform", "launchd", "-conf", "./configs"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "dev.slate.kanban") {
		t.Fatalf("print 应输出 plist 内容: %s", out)
	}
	plist := filepath.Join(home, "Library", "LaunchAgents", launchdPlistName)
	if _, err := os.Stat(plist); err == nil {
		t.Fatal("print 不应创建 plist 文件")
	}
}

// TestInstallDryRunWritesNoFiles install --dry-run 只打印计划，不能写服务文件，也不能建日志目录。
func TestInstallDryRunWritesNoFiles(t *testing.T) {
	home := tempHome(t)
	env := fixedEnv(t, home)
	out, err := RunCommand(env, []string{"install", "-conf", "./configs", "--dry-run"})
	if err != nil {
		t.Fatal(err)
	}
	plist := ServiceFilePath(home, PlatformLaunchd)
	for _, want := range []string{"[dry-run] 将会写入: " + plist, "<key>RunAtLoad</key>", "launchctl bootstrap"} {
		if !strings.Contains(out, want) {
			t.Fatalf("dry-run 输出缺少 %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "已写入") {
		t.Fatalf("dry-run 没有写文件，输出里不能说已写入:\n%s", out)
	}
	if _, err := os.Stat(plist); !os.IsNotExist(err) {
		t.Fatal("dry-run 不应创建 plist")
	}
	if _, err := os.Stat(filepath.Join(env.WorkDir, "logs")); !os.IsNotExist(err) {
		t.Fatal("dry-run 不应创建日志目录")
	}
}

// TestUninstallDryRunKeepsFile uninstall --dry-run 只说明会删什么，文件要原样留着，输出里不能说已删除。
func TestUninstallDryRunKeepsFile(t *testing.T) {
	home := tempHome(t)
	env := fixedEnv(t, home)
	if _, err := RunCommand(env, []string{"install", "-conf", "./configs"}); err != nil {
		t.Fatal(err)
	}
	out, err := RunCommand(env, []string{"uninstall", "--dry-run"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "已删除") || !strings.Contains(out, "[dry-run] 将会删除") {
		t.Fatalf("dry-run 的措辞不对:\n%s", out)
	}
	if _, err := os.Stat(ServiceFilePath(home, PlatformLaunchd)); err != nil {
		t.Fatalf("dry-run 不应删除服务文件: %v", err)
	}
}

// TestInstallWritesCorrectContent install 应把生成的内容写到注入的主目录下。
func TestInstallWritesCorrectContent(t *testing.T) {
	home := tempHome(t)
	env := fixedEnv(t, home)
	out, err := RunCommand(env, []string{"install", "-conf", "./configs"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "launchctl") {
		t.Fatalf("install 应提示手工执行 launchctl: %s", out)
	}
	plist := ServiceFilePath(home, PlatformLaunchd)
	data, err := os.ReadFile(plist)
	if err != nil {
		t.Fatal(err)
	}
	want := GenerateLaunchdPlist(env.ServiceInput("./configs"))
	if string(data) != want {
		t.Fatalf("写入内容与生成不一致\nwant:\n%s\ngot:\n%s", want, data)
	}
}

// TestInstallIdempotentLatest 内容相同时再装一次应提示已经是最新且不重写。
func TestInstallIdempotentLatest(t *testing.T) {
	home := tempHome(t)
	env := fixedEnv(t, home)
	_, err := RunCommand(env, []string{"install", "-conf", "./configs"})
	if err != nil {
		t.Fatal(err)
	}
	plist := ServiceFilePath(home, PlatformLaunchd)
	before, err := os.ReadFile(plist)
	if err != nil {
		t.Fatal(err)
	}
	out, err := RunCommand(env, []string{"install", "-conf", "./configs"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "已经是最新") {
		t.Fatalf("应提示已经是最新: %s", out)
	}
	after, err := os.ReadFile(plist)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("内容相同时不应重写文件")
	}
}

// TestInstallUpdatesWhenContentChanges 内容变化时应覆盖并提示已更新。
func TestInstallUpdatesWhenContentChanges(t *testing.T) {
	home := tempHome(t)
	env := fixedEnv(t, home)
	_, err := RunCommand(env, []string{"install", "-conf", "./configs"})
	if err != nil {
		t.Fatal(err)
	}
	env.Executable = filepath.Join(env.WorkDir, "slate-v2")
	if err := os.WriteFile(env.Executable, []byte{0}, 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := RunCommand(env, []string{"install", "-conf", "./configs"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "已更新") {
		t.Fatalf("应提示已更新: %s", out)
	}
	data, err := os.ReadFile(ServiceFilePath(home, PlatformLaunchd))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "slate-v2") {
		t.Fatal("更新后 plist 应引用新可执行路径")
	}
}

// TestUninstallRemovesFile uninstall 应删除自己写的服务文件。
func TestUninstallRemovesFile(t *testing.T) {
	home := tempHome(t)
	env := fixedEnv(t, home)
	_, err := RunCommand(env, []string{"install", "-conf", "./configs"})
	if err != nil {
		t.Fatal(err)
	}
	plist := ServiceFilePath(home, PlatformLaunchd)
	if _, err := os.Stat(plist); err != nil {
		t.Fatal("安装后文件应存在")
	}
	_, err = RunCommand(env, []string{"uninstall"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(plist); !os.IsNotExist(err) {
		t.Fatal("uninstall 应删除 plist")
	}
}

// TestUninstallWhenNotExists 文件不存在时 uninstall 仍算成功并说明没有安装过。
func TestUninstallWhenNotExists(t *testing.T) {
	home := tempHome(t)
	env := fixedEnv(t, home)
	out, err := RunCommand(env, []string{"uninstall"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "没有安装过") {
		t.Fatalf("应说明没有安装过: %s", out)
	}
}

// TestUnknownPlatform 不支持的 --platform 应返回错误。
func TestUnknownPlatform(t *testing.T) {
	home := tempHome(t)
	env := fixedEnv(t, home)
	_, err := RunCommand(env, []string{"print", "--platform", "windows"})
	if err == nil {
		t.Fatal("未知平台应报错")
	}
	if !strings.Contains(err.Error(), "platform") && !strings.Contains(err.Error(), "平台") {
		t.Fatalf("错误信息应提及平台: %v", err)
	}
}

// TestIllegalArgs 未知子命令或参数应报错并带用法提示。
func TestIllegalArgs(t *testing.T) {
	home := tempHome(t)
	env := fixedEnv(t, home)
	_, err := RunCommand(env, []string{"no-such-cmd"})
	if err == nil {
		t.Fatal("未知子命令应报错")
	}
	_, err = RunCommand(env, []string{"install", "-conf"})
	if err == nil {
		t.Fatal("-conf 缺少值应报错")
	}
	_, err = RunCommand(env, []string{"install", "--foo"})
	if err == nil {
		t.Fatal("未知旗标应报错")
	}
}

// TestSystemdInstallLinux 在 Linux 环境下 install 应写入 systemd 用户单元目录。
func TestSystemdInstallLinux(t *testing.T) {
	home := tempHome(t)
	env := fixedEnv(t, home)
	env.GOOS = "linux"
	out, err := RunCommand(env, []string{"install", "-conf", "./configs"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "systemctl") {
		t.Fatalf("应提示 systemctl: %s", out)
	}
	if !strings.Contains(out, "loginctl enable-linger") {
		t.Fatalf("应提示 linger: %s", out)
	}
	unit := ServiceFilePath(home, PlatformSystemd)
	data, err := os.ReadFile(unit)
	if err != nil {
		t.Fatal(err)
	}
	want := GenerateSystemdUnit(env.ServiceInput("./configs"))
	if string(data) != want {
		t.Fatalf("systemd 单元内容不一致")
	}
}

// TestInstallCreatesLogsDirForLaunchd launchd 打开日志文件时不会建目录，所以真正安装时要先把 logs 目录建好。
func TestInstallCreatesLogsDirForLaunchd(t *testing.T) {
	home := tempHome(t)
	env := fixedEnv(t, home)
	if _, err := RunCommand(env, []string{"install", "-conf", "./configs"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(env.WorkDir, "logs"))
	if err != nil || !info.IsDir() {
		t.Fatalf("安装后应有日志目录: %v", err)
	}
}

// TestInstallFollowUpQuotesPathsWithSpaces 工作目录含空格时，打印出来的 tail 命令要能直接粘贴执行。
func TestInstallFollowUpQuotesPathsWithSpaces(t *testing.T) {
	env := fixedEnv(t, tempHome(t))
	env.WorkDir = filepath.Join(env.HomeDir, "my proj")
	if err := os.MkdirAll(env.WorkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := RunCommand(env, []string{"install", "-conf", "/etc/slate"})
	if err != nil {
		t.Fatal(err)
	}
	want := "tail -f '" + filepath.Join(env.WorkDir, "logs", stdoutLogName) + "'"
	if !strings.Contains(out, want) {
		t.Fatalf("应该用单引号包住带空格的路径，期望含 %s\n%s", want, out)
	}
}

// TestConfArgValidation -conf 的取值要校验：空、全空白、过长、带换行或控制字符都应报错，且不写文件。
func TestConfArgValidation(t *testing.T) {
	env := fixedEnv(t, tempHome(t))
	bad := map[string]string{
		"空":    "",
		"全空白":  "   ",
		"换行":   "/etc/slate\nExecStartPre=/bin/false",
		"制表符":  "/etc/\tslate",
		"过长":   "/" + strings.Repeat("a", maxConfArgLen),
		"删除字符": "/etc/slate\x7f",
	}
	for name, value := range bad {
		t.Run(name, func(t *testing.T) {
			if _, err := RunCommand(env, []string{"install", "-conf", value}); err == nil {
				t.Fatalf("-conf=%q 应该报错", value)
			}
			if _, err := os.Stat(ServiceFilePath(env.HomeDir, PlatformLaunchd)); !os.IsNotExist(err) {
				t.Fatal("参数不合法时不应写出服务文件")
			}
		})
	}
}

// TestUnsupportedSystem 不是 macOS 或 Linux 时 install 和 uninstall 要明确报错，print 仍可用 --platform 预览。
func TestUnsupportedSystem(t *testing.T) {
	env := fixedEnv(t, tempHome(t))
	env.GOOS = "windows"
	for _, args := range [][]string{{"install"}, {"uninstall"}, {"print"}} {
		if _, err := RunCommand(env, args); err == nil {
			t.Fatalf("%v 在不支持的系统上应报错", args)
		}
	}
	if _, err := RunCommand(env, []string{"print", "--platform", "systemd"}); err != nil {
		t.Fatalf("指定平台后 print 应该可用: %v", err)
	}
}

// TestShellQuote 打印给用户粘贴的命令里，普通路径原样输出，含空格或单引号的路径要能安全粘贴。
func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"/Users/a/proj/slate":    "/Users/a/proj/slate",
		"/Users/a/my proj/slate": "'/Users/a/my proj/slate'",
		"/Users/a/it's/slate":    `'/Users/a/it'\''s/slate'`,
		"/Users/a/$HOME/slate":   "'/Users/a/$HOME/slate'",
		"":                       "''",
	}
	for in, want := range cases {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %s，期望 %s", in, got, want)
		}
	}
}
