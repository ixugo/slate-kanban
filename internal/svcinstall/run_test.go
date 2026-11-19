package svcinstall

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// fakeRunner 记录收到的命令；fail 返回 true 的命令会报错，用来模拟「未加载」「命令失败」。
type fakeRunner struct {
	calls []string
	fail  func(cmd string) bool
}

// run 实现 Runner：记下命令，按 fail 决定成败。
func (f *fakeRunner) run(name string, args ...string) (string, error) {
	cmd := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, cmd)
	if f.fail != nil && f.fail(cmd) {
		return "boom", errors.New("exit 1")
	}
	return "", nil
}

// has 判断是否执行过包含 sub 的命令。
func (f *fakeRunner) has(sub string) bool {
	for _, c := range f.calls {
		if strings.Contains(c, sub) {
			return true
		}
	}
	return false
}

// daemonEnv 构造带假命令执行器的环境，retryDelay 置零避免测试里真的等待。
func daemonEnv(t *testing.T, goos string, f *fakeRunner) *Env {
	t.Helper()
	retryDelay = 0
	env := fixedEnv(t, tempHome(t))
	env.GOOS = goos
	env.Run = f.run
	return env
}

// TestDaemonLaunchdFirstStart 首次 -d：写 plist，未加载则直接 bootstrap，不执行 bootout。
func TestDaemonLaunchdFirstStart(t *testing.T) {
	f := &fakeRunner{fail: func(c string) bool { return strings.HasPrefix(c, "launchctl print") }}
	env := daemonEnv(t, "darwin", f)
	var out, errOut strings.Builder
	if code := RunDaemon(&out, &errOut, env, false, ""); code != 0 {
		t.Fatalf("退出码 %d: %s", code, errOut.String())
	}
	if _, err := os.Stat(ServiceFilePath(env.HomeDir, PlatformLaunchd)); err != nil {
		t.Fatal("应写入 plist")
	}
	if !f.has("launchctl bootstrap gui/501 ") || f.has("bootout") {
		t.Fatalf("命令不符: %v", f.calls)
	}
	if !strings.Contains(out.String(), "slate -stop") {
		t.Fatalf("应提示停止方法: %s", out.String())
	}
}

// TestDaemonLaunchdRestartWhenUnchanged 服务文件没变且已加载：只 kickstart 重启。
func TestDaemonLaunchdRestartWhenUnchanged(t *testing.T) {
	f := &fakeRunner{}
	env := daemonEnv(t, "darwin", f)
	if _, err := StartService(env, PlatformLaunchd, ""); err != nil {
		t.Fatal(err)
	}
	f.calls = nil
	if _, err := StartService(env, PlatformLaunchd, ""); err != nil {
		t.Fatal(err)
	}
	if !f.has("kickstart -k gui/501/"+launchdLabel) || f.has("bootstrap") || f.has("bootout") {
		t.Fatalf("命令不符: %v", f.calls)
	}
}

// TestDaemonLaunchdReloadWhenChanged 服务文件变了且已加载：先 bootout 再 bootstrap。
func TestDaemonLaunchdReloadWhenChanged(t *testing.T) {
	f := &fakeRunner{}
	env := daemonEnv(t, "darwin", f)
	if _, err := StartService(env, PlatformLaunchd, ""); err != nil {
		t.Fatal(err)
	}
	f.calls = nil
	if _, err := StartService(env, PlatformLaunchd, "./other"); err != nil {
		t.Fatal(err)
	}
	if !f.has("bootout") || !f.has("bootstrap") {
		t.Fatalf("应先卸载再加载: %v", f.calls)
	}
}

// TestDaemonLaunchdBootstrapRetry bootstrap 前几次失败后成功，应重试而不是直接报错。
func TestDaemonLaunchdBootstrapRetry(t *testing.T) {
	n := 0
	f := &fakeRunner{fail: func(c string) bool {
		if strings.HasPrefix(c, "launchctl print") {
			return true
		}
		if strings.HasPrefix(c, "launchctl bootstrap") {
			n++
			return n < 3
		}
		return false
	}}
	env := daemonEnv(t, "darwin", f)
	if _, err := StartService(env, PlatformLaunchd, ""); err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("应尝试 3 次，实际 %d", n)
	}
}

// TestDaemonBootstrapGivesUp 一直失败时返回错误，退出码非 0。
func TestDaemonBootstrapGivesUp(t *testing.T) {
	f := &fakeRunner{fail: func(c string) bool { return strings.HasPrefix(c, "launchctl") }}
	env := daemonEnv(t, "darwin", f)
	var out, errOut strings.Builder
	if code := RunDaemon(&out, &errOut, env, false, ""); code == 0 {
		t.Fatal("应失败")
	}
	if !strings.Contains(errOut.String(), "启动服务失败") {
		t.Fatalf("错误文案不符: %s", errOut.String())
	}
}

// TestDaemonSystemdStart Linux 上依次 daemon-reload、enable、restart。
func TestDaemonSystemdStart(t *testing.T) {
	f := &fakeRunner{}
	env := daemonEnv(t, "linux", f)
	var out, errOut strings.Builder
	if code := RunDaemon(&out, &errOut, env, false, ""); code != 0 {
		t.Fatalf("退出码 %d: %s", code, errOut.String())
	}
	want := []string{
		"systemctl --user daemon-reload",
		"systemctl --user enable slate.service",
		"systemctl --user restart slate.service",
	}
	if strings.Join(f.calls, "|") != strings.Join(want, "|") {
		t.Fatalf("命令不符: %v", f.calls)
	}
	if !strings.Contains(out.String(), "enable-linger") {
		t.Fatalf("应提示 linger: %s", out.String())
	}
}

// TestStopLaunchd -stop：已加载则 bootout，并删除 plist。
func TestStopLaunchd(t *testing.T) {
	f := &fakeRunner{}
	env := daemonEnv(t, "darwin", f)
	if _, err := StartService(env, PlatformLaunchd, ""); err != nil {
		t.Fatal(err)
	}
	f.calls = nil
	var out, errOut strings.Builder
	if code := RunDaemon(&out, &errOut, env, true, ""); code != 0 {
		t.Fatalf("退出码 %d: %s", code, errOut.String())
	}
	if !f.has("bootout gui/501/" + launchdLabel) {
		t.Fatalf("应执行 bootout: %v", f.calls)
	}
	if _, err := os.Stat(ServiceFilePath(env.HomeDir, PlatformLaunchd)); err == nil {
		t.Fatal("plist 应被删除")
	}
}

// TestStopSystemd -stop：先 disable --now，再删单元文件，最后 daemon-reload。
func TestStopSystemd(t *testing.T) {
	f := &fakeRunner{}
	env := daemonEnv(t, "linux", f)
	if _, err := StartService(env, PlatformSystemd, ""); err != nil {
		t.Fatal(err)
	}
	f.calls = nil
	if _, _, err := StopService(env, PlatformSystemd); err != nil {
		t.Fatal(err)
	}
	want := "systemctl --user disable --now slate.service|systemctl --user daemon-reload"
	if strings.Join(f.calls, "|") != want {
		t.Fatalf("命令不符: %v", f.calls)
	}
	if _, err := os.Stat(ServiceFilePath(env.HomeDir, PlatformSystemd)); err == nil {
		t.Fatal("单元文件应被删除")
	}
}

// TestStopWhenNotRegistered 没注册过时 -stop 直接成功，不执行任何停止命令。
func TestStopWhenNotRegistered(t *testing.T) {
	for _, goos := range []string{"darwin", "linux"} {
		f := &fakeRunner{fail: func(c string) bool { return strings.HasPrefix(c, "launchctl print") }}
		env := daemonEnv(t, goos, f)
		var out, errOut strings.Builder
		if code := RunDaemon(&out, &errOut, env, true, ""); code != 0 {
			t.Fatalf("%s 退出码 %d: %s", goos, code, errOut.String())
		}
		if f.has("bootout") || f.has("disable") {
			t.Fatalf("%s 不应执行停止命令: %v", goos, f.calls)
		}
		if !strings.Contains(out.String(), "没有已注册") {
			t.Fatalf("%s 文案不符: %s", goos, out.String())
		}
	}
}

// TestStopKeepsFileWhenCommandFails 停止命令失败时保留服务文件并报错，避免进程还在跑却没了注册信息。
func TestStopKeepsFileWhenCommandFails(t *testing.T) {
	f := &fakeRunner{}
	env := daemonEnv(t, "linux", f)
	if _, err := StartService(env, PlatformSystemd, ""); err != nil {
		t.Fatal(err)
	}
	f.fail = func(c string) bool { return strings.Contains(c, "disable") }
	var out, errOut strings.Builder
	if code := RunDaemon(&out, &errOut, env, true, ""); code == 0 {
		t.Fatal("应失败")
	}
	if _, err := os.Stat(ServiceFilePath(env.HomeDir, PlatformSystemd)); err != nil {
		t.Fatal("停止失败时应保留单元文件")
	}
}

// TestDaemonRejectsBadInput 不支持的系统、非法 -conf 都应报错且不执行任何命令、不写文件。
func TestDaemonRejectsBadInput(t *testing.T) {
	f := &fakeRunner{}
	env := daemonEnv(t, "windows", f)
	var out, errOut strings.Builder
	if code := RunDaemon(&out, &errOut, env, false, ""); code == 0 || !strings.Contains(errOut.String(), "不支持") {
		t.Fatalf("windows 应报不支持: %s", errOut.String())
	}
	env.GOOS = "darwin"
	errOut.Reset()
	if code := RunDaemon(&out, &errOut, env, false, "bad\npath"); code == 0 {
		t.Fatal("含换行的 -conf 应被拒绝")
	}
	if len(f.calls) != 0 {
		t.Fatalf("不应执行命令: %v", f.calls)
	}
	if _, err := os.Stat(ServiceFilePath(env.HomeDir, PlatformLaunchd)); err == nil {
		t.Fatal("不应写文件")
	}
}
