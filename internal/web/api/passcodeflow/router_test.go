// Package passcodeflow 用真实的路由装配（api.NewHTTPHandler）验证启用访问口令后的整体行为。
// 单独放一个包，是因为路由装配会注册进程级的统计变量，同一个进程里只能装配一次；
// 而 api 包自己的测试已经装配了一套「未启用口令」的路由。
package passcodeflow

import (
	"bytes"
	"expvar"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/ixugo/goddd/domain/version/versionapi"
	"github.com/ixugo/goddd/pkg/orm"

	"slate-kanban/internal/conf"
	"slate-kanban/internal/web/api"
	"slate-kanban/internal/web/api/connectapi"
	"slate-kanban/internal/web/api/kanbanapi"
)

const (
	// passcode 是测试服务配置的访问口令。
	passcode = "flow-test-pass-123"
	// maxFailures 对应限流器每个窗口内允许的错误次数（见 api 包 maxFailedAttemptsPerWindow）。
	maxFailures = 10
	// maxBody 对应 /api/v1 请求体大小上限 1 MiB（见 api 包 maxRequestBody）。
	maxBody = 1 << 20
)

// engine 是整个测试进程共用的、已启用口令的真实路由。
var engine *gin.Engine

// TestMain 装配一套启用了口令的真实路由（内存库），所有用例共用；用例各自使用不同的来源 IP，互不影响限流。
func TestMain(m *testing.M) {
	os.Exit(run(m))
}

// run 建库、装配路由并跑测试，返回退出码（单独成函数是为了让 defer 在 os.Exit 之前执行）。
func run(m *testing.M) int {
	db, err := orm.New(sqlite.Open(":memory:"), orm.Config{MaxIdleConns: 1, MaxOpenConns: 1})
	if err != nil {
		fmt.Fprintln(os.Stderr, "打开内存数据库失败:", err)
		return 1
	}
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}()
	orm.SetEnabledAutoMigrate(true)
	expvar.NewString("git_branch").Set("test")
	expvar.NewString("git_hash").Set("test")

	bc := &conf.Bootstrap{Server: conf.Server{HTTP: conf.ServerHTTP{Passcode: passcode}}}
	bc.Runtime.BuildVersion = "passcodeflow-test"
	kcore := api.NewKanbanCore(db)
	mcpAPI, stopMCP := api.NewMCP(bc, kcore)
	defer stopMCP()
	uc := &api.Usecase{
		Conf:    bc,
		DB:      db,
		Version: versionapi.New(versionapi.NewVersionCore(db)),
		Kanban:  kanbanapi.New(kcore),
		MCP:     mcpAPI,
		Connect: connectapi.New(conf.PasscodeRequired(bc)),
	}
	h, ok := api.NewHTTPHandler(uc).(*gin.Engine)
	if !ok {
		fmt.Fprintln(os.Stderr, "NewHTTPHandler 返回的不是 gin 引擎")
		return 1
	}
	engine = h
	return m.Run()
}

// request 构造发给真实路由的请求：固定 Host，并指定 TCP 对端地址（限流按它区分来源）。
func request(method, path, remote string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.Host = "localhost"
	req.RemoteAddr = remote + ":5000"
	return req
}

// serve 把请求交给真实路由并返回响应记录。
func serve(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

// loginRequest 构造带 JSON 请求体的登录请求。
func loginRequest(body, remote string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader([]byte(body)))
	req.Host = "localhost"
	req.RemoteAddr = remote + ":5000"
	req.Header.Set("Content-Type", "application/json")
	return req
}

// TestProtectsAPIWithoutCredential 启用口令后，受保护入口无凭证都应 401，
// 连不存在的接口路径也一样（不能靠 404 与 401 的差别探测接口）。
func TestProtectsAPIWithoutCredential(t *testing.T) {
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/boards"},
		{http.MethodGet, "/api/v1/version"},
		{http.MethodGet, "/api/v1/connect"},
		{http.MethodGet, "/api/v1/no-such-endpoint"},
		{http.MethodGet, "/api/v1/boards/1/stream"},
		{http.MethodGet, "/app/metrics/api"},
		{http.MethodPost, "/mcp"},
	} {
		if rec := serve(request(tc.method, tc.path, "203.0.113.50")); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s 无凭证应 401，实际 %d", tc.method, tc.path, rec.Code)
		}
	}
}

// TestKeepsPublicPathsOpen 公开路径在启用口令后仍可直接访问。
func TestKeepsPublicPathsOpen(t *testing.T) {
	for _, path := range []string{"/health", "/install.md", "/skills/slate-kanban/SKILL.md", "/api/v1/auth/status", "/"} {
		if rec := serve(request(http.MethodGet, path, "203.0.113.51")); rec.Code != http.StatusOK {
			t.Errorf("%s 应免口令且 200，实际 %d", path, rec.Code)
		}
	}
}

// TestInstallGuideMentionsPasscode 启用口令后，对外公开的安装说明要提醒 agent 带 Bearer 头，且不能泄露口令本身。
func TestInstallGuideMentionsPasscode(t *testing.T) {
	rec := serve(request(http.MethodGet, "/install.md", "203.0.113.55"))
	body := rec.Body.String()
	if !strings.Contains(body, "Authorization: Bearer") {
		t.Fatalf("安装说明应包含 Bearer 头的写法")
	}
	if strings.Contains(body, passcode) {
		t.Fatal("安装说明不能包含真实口令")
	}
}

// TestBearerAndCookieAccess 正确的 Bearer（方案名大小写不限）和登录后的会话 cookie 都能访问受保护接口。
func TestBearerAndCookieAccess(t *testing.T) {
	bearer := request(http.MethodGet, "/api/v1/boards", "203.0.113.52")
	bearer.Header.Set("Authorization", "bearer "+passcode)
	if rec := serve(bearer); rec.Code != http.StatusOK {
		t.Fatalf("Bearer 应通过: %d %s", rec.Code, rec.Body)
	}

	login := serve(loginRequest(`{"passcode":" `+passcode+` "}`, "203.0.113.52"))
	if login.Code != http.StatusOK {
		t.Fatalf("登录应成功（口令首尾的空白会被忽略）: %d %s", login.Code, login.Body)
	}
	withCookie := request(http.MethodGet, "/api/v1/boards", "203.0.113.52")
	for _, c := range login.Result().Cookies() {
		withCookie.AddCookie(c)
	}
	if rec := serve(withCookie); rec.Code != http.StatusOK {
		t.Fatalf("带会话 cookie 应通过: %d %s", rec.Code, rec.Body)
	}
}

// TestLoginRejectsBadBodies 登录接口不要求口令，所以必须自带请求体类型和大小限制。
func TestLoginRejectsBadBodies(t *testing.T) {
	plain := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"passcode":"x"}`))
	plain.Host = "localhost"
	plain.Header.Set("Content-Type", "text/plain")
	if rec := serve(plain); rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("非 JSON 登录请求应 415: %d", rec.Code)
	}

	huge := loginRequest(`{"passcode":"`+strings.Repeat("a", maxBody)+`"}`, "203.0.113.53")
	if rec := serve(huge); rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("超过 1 MiB 的登录请求应 413: %d", rec.Code)
	}
}

// TestThrottleIgnoresForwardedFor 攻击者每次换一个 X-Forwarded-For 也躲不过限流：
// 限流按 TCP 对端地址计数，而这个头可以随便写。
func TestThrottleIgnoresForwardedFor(t *testing.T) {
	const attacker = "198.51.100.77"
	for i := 0; i < maxFailures+1; i++ {
		req := request(http.MethodGet, "/api/v1/boards", attacker)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer wrong-%d", i))
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("192.0.2.%d", i+1))
		serve(req)
	}
	req := request(http.MethodGet, "/api/v1/boards", attacker)
	req.Header.Set("Authorization", "Bearer "+passcode)
	req.Header.Set("X-Forwarded-For", "192.0.2.200")
	if rec := serve(req); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("换 X-Forwarded-For 不应躲过封锁，期望 429，实际 %d", rec.Code)
	}

	other := request(http.MethodGet, "/api/v1/boards", "198.51.100.78")
	other.Header.Set("Authorization", "Bearer "+passcode)
	if rec := serve(other); rec.Code != http.StatusOK {
		t.Fatalf("别的来源不应被连坐: %d", rec.Code)
	}
}
