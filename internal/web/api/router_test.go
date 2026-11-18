package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
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
	"slate-kanban/internal/web/api/apitest"
	"slate-kanban/internal/web/api/connectapi"
	"slate-kanban/internal/web/api/kanbanapi"
)

const testBuildVersion = "router-test-version"

var shared struct {
	engine *gin.Engine
	uc     *Usecase
}

func TestMain(m *testing.M) {
	os.Exit(runWithSharedRouter(m))
}

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

	orm.SetEnabledAutoMigrate(true)
	expvar.NewString("git_branch").Set("test")
	expvar.NewString("git_hash").Set("test")

	bc := &conf.Bootstrap{}
	bc.Runtime.BuildVersion = testBuildVersion
	kcore := NewKanbanCore(db)
	mcpAPI, stopMCP := NewMCP(bc, kcore)
	defer stopMCP()
	uc := &Usecase{
		Conf:    bc,
		DB:      db,
		Version: versionapi.New(versionapi.NewVersionCore(db)),
		Kanban:  kanbanapi.New(kcore),
		MCP:     mcpAPI,
		Connect: connectapi.New(false),
	}
	engine, ok := NewHTTPHandler(uc).(*gin.Engine)
	if !ok {
		fmt.Fprintln(os.Stderr, "NewHTTPHandler 返回的不是 gin 引擎")
		return 1
	}
	shared.engine, shared.uc = engine, uc
	return m.Run()
}

func call(t *testing.T, method, path, token string, body any) apitest.Response {
	t.Helper()
	return apitest.Client{Engine: shared.engine}.Do(t, method, path, token, body)
}

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

func withDebug(t *testing.T) {
	t.Helper()
	shared.uc.Conf.Runtime.Debug = true
	t.Cleanup(func() { shared.uc.Conf.Runtime.Debug = false })
}

func TestHealthIsOpen(t *testing.T) {
	if resp := call(t, http.MethodGet, "/health", "", nil); resp.Code != http.StatusOK {
		t.Fatalf("健康检查应该免认证: %d %s", resp.Code, resp.Body)
	}
}

func TestAPINoAuthHuman(t *testing.T) {
	for _, path := range []string{"/api/v1/boards", "/api/v1/version", "/app/metrics/api", "/api/v1/connect"} {
		if resp := call(t, http.MethodGet, path, "", nil); resp.Code != http.StatusOK {
			t.Errorf("%s 应免认证: %d %s", path, resp.Code, resp.Body)
		}
	}
	board := call(t, http.MethodPost, "/api/v1/boards", "", map[string]string{"name": "免登录看板", "slug": "NOAUTH"})
	if board.Code != http.StatusOK {
		t.Fatalf("创建看板失败: %d %s", board.Code, board.Body)
	}
}

func TestGuardsMounted(t *testing.T) {
	huge := `{"name":"` + strings.Repeat("a", 1<<20) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/boards", strings.NewReader(huge))
	req.Host = "localhost"
	req.Header.Set("Content-Type", "application/json")
	if resp := (apitest.Client{Engine: shared.engine}).Serve(req); resp.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("超过 1 MiB 应该返回 413: %d", resp.Code)
	}
	plain := httptest.NewRequest(http.MethodPost, "/api/v1/boards", strings.NewReader(`{"name":"x"}`))
	plain.Host = "localhost"
	plain.Header.Set("Content-Type", "text/plain")
	if resp := (apitest.Client{Engine: shared.engine}).Serve(plain); resp.Code != http.StatusUnsupportedMediaType {
		t.Errorf("不是 JSON 应该返回 415: %d", resp.Code)
	}
}

func TestStreamThroughRouter(t *testing.T) {
	withDebug(t)
	board := call(t, http.MethodPost, "/api/v1/boards", "", map[string]string{"name": "流测试看板", "slug": "RTSTREAM"})
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
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("没能及时收到响应头: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	line, err := bufio.NewReader(resp.Body).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "event: ready" {
		t.Fatalf("第一行应该是 event: ready，实际 %q（错误: %v）", line, err)
	}
}

func TestSecurityHostAndOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/boards", http.NoBody)
	req.Host = "evil.com"
	if resp := (apitest.Client{Engine: shared.engine}).Serve(req); resp.Code != http.StatusForbidden {
		t.Errorf("非法 Host 应 403: %d", resp.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/boards", strings.NewReader(`{"name":"x","slug":"X"}`))
	req.Host = "localhost"
	req.Header.Set("Origin", "http://evil.com")
	req.Header.Set("Content-Type", "application/json")
	if resp := (apitest.Client{Engine: shared.engine}).Serve(req); resp.Code != http.StatusForbidden {
		t.Errorf("跨站写应 403: %d", resp.Code)
	}
	okReq := httptest.NewRequest(http.MethodGet, "/api/v1/boards", http.NoBody)
	okReq.Host = "localhost"
	resp := (apitest.Client{Engine: shared.engine}).Serve(okReq)
	if resp.Code != http.StatusOK {
		t.Errorf("合法 Host 应通过: %d", resp.Code)
	}
	rec := httptest.NewRecorder()
	shared.engine.ServeHTTP(rec, okReq)
	if rec.Header().Get("X-Frame-Options") != "DENY" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("安全响应头缺失")
	}
}

func TestConnectNoTokenString(t *testing.T) {
	resp := call(t, http.MethodGet, "/api/v1/connect", "", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("/connect 应 200: %d", resp.Code)
	}
	if strings.Contains(strings.ToLower(string(resp.Body)), "token") {
		t.Fatalf("connect 响应不应含 token: %s", resp.Body)
	}
	var g struct {
		Hosts []struct {
			MCP []struct {
				Kind string `json:"kind"`
			} `json:"mcp"`
		} `json:"hosts"`
	}
	if err := json.Unmarshal(resp.Body, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Hosts) < 1 || len(g.Hosts[0].MCP) < 2 {
		t.Fatalf("每个 host 应有 worker 和 manager 片段: %s", resp.Body)
	}
	if g.Hosts[0].MCP[0].Kind != "worker" || g.Hosts[0].MCP[1].Kind != "manager" {
		t.Errorf("片段 kind 不对: %+v", g.Hosts[0].MCP)
	}
}

func TestAPIUnknownPathJSON404(t *testing.T) {
	resp := call(t, http.MethodGet, "/api/v1/nope", "", nil)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("未知 API 应 404: %d", resp.Code)
	}
	if strings.Contains(string(resp.Body), "index.html") {
		t.Fatal("API 404 不应回退 SPA")
	}
}

func TestDebugBodyLogging(t *testing.T) {
	withDebug(t)
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(old) })
	call(t, http.MethodPost, "/api/v1/boards", "", map[string]string{"name": "日志看板", "slug": "LOG"})
	if !strings.Contains(logs.String(), "msg=body") {
		t.Error("调试模式应记录普通接口请求体")
	}
}
