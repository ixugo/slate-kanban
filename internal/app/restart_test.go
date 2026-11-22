package app

import (
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// phaseEnv 是测试子进程用来区分「第一阶段（负责换程序）」和「第二阶段（换完之后）」的环境变量。
const phaseEnv = "SLATE_RESTART_TEST_PHASE"

// TestRestartRequester 验证请求重启：路径会投进通道，重复请求不阻塞、只保留第一次。
func TestRestartRequester(t *testing.T) {
	request, ch := newRestartRequester()
	if !restartSupported() {
		t.Skip("当前环境不支持自动重启")
	}
	if !request("/a/slate") || !request("/b/slate") {
		t.Fatal("支持重启时请求应返回 true")
	}
	if got := <-ch; got != "/a/slate" {
		t.Fatalf("应保留第一次请求的路径，实际 %q", got)
	}
	select {
	case extra := <-ch:
		t.Fatalf("重复请求应被并入，却多出一次: %q", extra)
	default:
	}
}

// TestRestartReplacesProcess 验证 restartProcess 真的把进程换成了新程序，并带上了新的环境变量：
// 子进程第一阶段调用 restartProcess 换成自己（环境变量改成第二阶段），第二阶段打印标记后正常退出。
func TestRestartReplacesProcess(t *testing.T) {
	switch os.Getenv(phaseEnv) {
	case "1":
		// 先去掉旧值再追加：环境变量重复时 Getenv 取第一个，会让子进程永远停在第一阶段
		env := slices.DeleteFunc(os.Environ(), func(kv string) bool { return strings.HasPrefix(kv, phaseEnv+"=") })
		_ = restartProcess(os.Args[0], os.Args, append(env, phaseEnv+"=2"))
		os.Exit(3) // 走到这里说明没换成功
	case "2":
		os.Stdout.WriteString("restarted-ok")
		os.Exit(0)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestRestartReplacesProcess$")
	cmd.Env = append(os.Environ(), phaseEnv+"=1")
	out, err := cmd.Output()
	if err != nil || !strings.Contains(string(out), "restarted-ok") {
		t.Fatalf("进程应被换成第二阶段并正常退出，输出 %q，错误 %v", out, err)
	}
}
