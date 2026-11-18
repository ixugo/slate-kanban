package api

import (
	"bufio"
	"bytes"
	"context"
	"expvar"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/ixugo/goddd/domain/version/versionapi"
	"github.com/ixugo/goddd/pkg/orm"

	"slate-kanban/internal/conf"
	"slate-kanban/internal/web/api/accessapi"
	"slate-kanban/internal/web/api/apitest"
	"slate-kanban/internal/web/api/connectapi"
	"slate-kanban/internal/web/api/kanbanapi"
)

// testAdminToken 是测试路由使用的管理令牌。
const testAdminToken = "router-test-admin-token-0123456789"

// testBuildVersion 是测试路由使用的构建版本号，MCP 服务会把它报给客户端。
const testBuildVersion = "router-test-version"

// shared 是整个测试进程共用的一套完整路由，由 TestMain 组装。
var shared struct {
	engine *gin.Engine
	uc     *Usecase
}

// TestMain 组装一套完整的路由，让本包的路由测试共用。
// web.Metrics 会注册全局的 expvar 变量，同一个进程里只能注册一次，所以不能每个测试各组装一次；
// 各个测试用不同的名称创建自己的数据，互不影响。
func TestMain(m *testing.M) {
	os.Exit(runWithSharedRouter(m))
}

// runWithSharedRouter 组装路由、跑完全部测试、关闭数据库，返回退出码。
// 单独成函数，是为了让 defer 在 os.Exit 之前执行。
func runWithSharedRouter(m *testing.M) int {
	db, err := orm.New(sqlite.Open(":memory:"), orm.Config{MaxIdleConns: 1, MaxOpenConns: 1})
	if err != nil {
		fmt.Fprintln(os.Stderr, "打开内存数据库失败:", err)
		return 1
	}
	defer func() {
		sqlDB, err := db.DB()
		if err == nil {
			err = sqlDB.Close()
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "关闭内存数据库失败:", err)
		}
	}()

	// 和 main.go 一致：每次启动都迁移表结构；/health 会读这两个 expvar 变量，正式运行时由 main.go 注册
	orm.SetEnabledAutoMigrate(true)
	expvar.NewString("git_branch").Set("test")
	expvar.NewString("git_hash").Set("test")

	bc := &conf.Bootstrap{}
	bc.Auth.AdminToken = testAdminToken
	bc.Runtime.BuildVersion = testBuildVersion
	acore, kcore := NewAccessCore(db, bc), NewKanbanCore(db)
	mcpAPI, stopMCP := NewMCP(bc, acore, kcore)
	defer stopMCP() // 在关闭数据库之前停掉通知器
	uc := &Usecase{
		Conf:    bc,
		DB:      db,
		Version: versionapi.New(versionapi.NewVersionCore(db)),
		Access:  accessapi.New(acore),
		Kanban:  kanbanapi.New(kcore),
		MCP:     mcpAPI,
		Connect: connectapi.New(),
	}
	engine, ok := NewHTTPHandler(uc).(*gin.Engine)
	if !ok {
		fmt.Fprintln(os.Stderr, "NewHTTPHandler 返回的不是 gin 引擎")
		return 1
	}
	shared.engine, shared.uc = engine, uc
	return m.Run()
}

// call 向共用的路由发请求。token 为空就不带身份。
func call(t *testing.T, method, path, token string, body any) apitest.Response {
	t.Helper()
	return apitest.Client{Engine: shared.engine}.Do(t, method, path, token, body)
}

// field 把响应体解析成对象，取出名为 key 的字段，不存在就让测试终止。
func field(t *testing.T, resp apitest.Response, key string) any {
	t.Helper()
	if resp.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d: %s", resp.Code, resp.Body)
	}
	var out map[string]any
	resp.JSON(t, &out)
	v, ok := out[key]
	if !ok {
		t.Fatalf("响应里没有字段 %s: %s", key, resp.Body)
	}
	return v
}

// withDebug 在当前测试里打开调试模式（会记录请求体和响应体），测试结束后还原。
func withDebug(t *testing.T) {
	t.Helper()
	shared.uc.Conf.Runtime.Debug = true
	t.Cleanup(func() { shared.uc.Conf.Runtime.Debug = false })
}

// TestHealthIsOpen 验证健康检查不需要令牌，部署时的探活不用带密钥。
func TestHealthIsOpen(t *testing.T) {
	if resp := call(t, http.MethodGet, "/health", "", nil); resp.Code != http.StatusOK {
		t.Fatalf("健康检查应该免认证: %d %s", resp.Code, resp.Body)
	}
}

// TestAPIRequiresToken 验证所有接口都要令牌：看板、身份、版本、请求统计没带令牌或者令牌不对都是 401；
// 管理令牌能通过；认证排在请求检查之前，没有令牌的人看不到请求体上限和格式的检查结果。
func TestAPIRequiresToken(t *testing.T) {
	for _, path := range []string{"/api/v1/me", "/api/v1/boards", "/api/v1/version", "/app/metrics/api"} {
		if resp := call(t, http.MethodGet, path, "", nil); resp.Code != http.StatusUnauthorized {
			t.Errorf("%s 没有令牌应该返回 401: %d %s", path, resp.Code, resp.Body)
		}
		if resp := call(t, http.MethodGet, path, "wrong-token", nil); resp.Code != http.StatusUnauthorized {
			t.Errorf("%s 令牌不对应该返回 401: %d", path, resp.Code)
		}
		if resp := call(t, http.MethodGet, path, testAdminToken, nil); resp.Code != http.StatusOK {
			t.Errorf("%s 管理令牌应该能访问: %d %s", path, resp.Code, resp.Body)
		}
	}
	bad := httptest.NewRequest(http.MethodPost, "/api/v1/boards", strings.NewReader("x"))
	bad.Header.Set("Content-Type", "text/plain")
	if resp := (apitest.Client{Engine: shared.engine}).Serve(bad); resp.Code != http.StatusUnauthorized {
		t.Errorf("没有令牌时，应该先返回 401 而不是 415: %d %s", resp.Code, resp.Body)
	}
}

// TestAgentLifecycle 验证从头到尾的流程：管理令牌创建 agent 令牌；agent 能读写看板，
// 但不能管理令牌；令牌被删除后立刻失效。这同时检查了依赖装配：表迁移、两个领域和路由都接好了。
func TestAgentLifecycle(t *testing.T) {
	created := call(t, http.MethodPost, "/api/v1/agent-tokens", testAdminToken, map[string]string{"name": "router-bot"})
	plain, _ := field(t, created, "token").(string)
	tokenID := int64(field(t, created, "id").(float64))
	if !strings.HasPrefix(plain, "slate_") {
		t.Fatalf("令牌明文应该以 slate_ 开头: %q", plain)
	}

	me := call(t, http.MethodGet, "/api/v1/me", plain, nil)
	if field(t, me, "name") != "router-bot" || field(t, me, "kind") != "agent" {
		t.Fatalf("agent 令牌应该得到 agent 身份: %s", me.Body)
	}

	board := call(t, http.MethodPost, "/api/v1/boards", testAdminToken, map[string]string{"name": "路由测试看板", "slug": "RTE2E"})
	boardID := int64(field(t, board, "id").(float64))
	got := call(t, http.MethodGet, "/api/v1/boards/"+strconv.FormatInt(boardID, 10), plain, nil)
	if field(t, got, "slug") != "RTE2E" {
		t.Fatalf("agent 应该能读到看板: %s", got.Body)
	}

	if resp := call(t, http.MethodPost, "/api/v1/agent-tokens", plain, map[string]string{"name": "other"}); resp.Code != http.StatusForbidden {
		t.Errorf("agent 不能创建令牌: %d %s", resp.Code, resp.Body)
	}

	if resp := call(t, http.MethodDelete, "/api/v1/agent-tokens/"+strconv.FormatInt(tokenID, 10), testAdminToken, nil); resp.Code != http.StatusOK {
		t.Fatalf("删除令牌失败: %d %s", resp.Code, resp.Body)
	}
	if resp := call(t, http.MethodGet, "/api/v1/me", plain, nil); resp.Code != http.StatusUnauthorized {
		t.Errorf("令牌删除后应该立刻失效: %d", resp.Code)
	}
}

// TestGuardsMounted 验证请求检查已经挂在接口组上：过大的请求体返回 413，不是 JSON 的返回 415。
func TestGuardsMounted(t *testing.T) {
	huge := `{"name":"` + strings.Repeat("a", 1<<20) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/boards", strings.NewReader(huge))
	req.Header.Set("Authorization", "Bearer "+testAdminToken)
	req.Header.Set("Content-Type", "application/json")
	if resp := (apitest.Client{Engine: shared.engine}).Serve(req); resp.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("超过 1 MiB 应该返回 413: %d", resp.Code)
	}

	plain := httptest.NewRequest(http.MethodPost, "/api/v1/boards", strings.NewReader(`{"name":"x"}`))
	plain.Header.Set("Authorization", "Bearer "+testAdminToken)
	plain.Header.Set("Content-Type", "text/plain")
	if resp := (apitest.Client{Engine: shared.engine}).Serve(plain); resp.Code != http.StatusUnsupportedMediaType {
		t.Errorf("不是 JSON 应该返回 415: %d", resp.Code)
	}
}

// TestTokenPathsNotLogged 验证调试模式下，令牌管理接口的请求体和响应体不会写进日志（响应里有令牌明文），
// 而普通接口照常记录，用来证明这个测试确实能看到日志。
func TestTokenPathsNotLogged(t *testing.T) {
	withDebug(t)
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(old) })
	bodyLogs := func() int { return strings.Count(logs.String(), "msg=body") }

	before := bodyLogs()
	made := call(t, http.MethodPost, "/api/v1/agent-tokens", testAdminToken, map[string]string{"name": "log-bot"})
	if plain, _ := field(t, made, "token").(string); strings.Contains(logs.String(), plain) {
		t.Fatal("令牌明文不能出现在日志里")
	}
	call(t, http.MethodGet, "/api/v1/agent-tokens", testAdminToken, nil)
	if got := bodyLogs(); got != before {
		t.Errorf("令牌管理接口不应该记录请求体和响应体，多出了 %d 条", got-before)
	}

	call(t, http.MethodPost, "/api/v1/boards", testAdminToken, map[string]string{"name": "日志对照看板", "slug": "RTLOG"})
	if got := bodyLogs(); got != before+1 {
		t.Errorf("普通接口在调试模式下应该记录请求体和响应体: 期望 %d 条，实际 %d 条", before+1, got)
	}
}

// TestStreamThroughRouter 验证实时流经过完整的中间件链也能及时送达：调试模式下日志中间件会包一层
// 响应写入器，如果它没有透传刷新，客户端要等到流结束才能收到第一个事件。
func TestStreamThroughRouter(t *testing.T) {
	withDebug(t)
	board := call(t, http.MethodPost, "/api/v1/boards", testAdminToken, map[string]string{"name": "流测试看板", "slug": "RTSTREAM"})
	boardID := int64(field(t, board, "id").(float64))

	srv := httptest.NewServer(shared.engine)
	t.Cleanup(func() {
		srv.CloseClientConnections()
		srv.Close()
	})
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1/boards/"+strconv.FormatInt(boardID, 10)+"/stream", http.NoBody)
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+testAdminToken)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("没能及时收到响应头，刷新可能被中间件挡住了: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("关闭响应体: %v", err)
		}
	}()
	line, err := bufio.NewReader(resp.Body).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "event: ready" {
		t.Fatalf("第一行应该是 event: ready，实际 %q（错误: %v）", line, err)
	}
}
