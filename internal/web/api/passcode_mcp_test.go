package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/ixugo/goddd/pkg/orm"

	"slate-kanban/internal/conf"
)

// TestMCPPasscodeRequired MCP initialize 无 Bearer 应 401，带上口令应能握手。
func TestMCPPasscodeRequired(t *testing.T) {
	db, err := orm.New(sqlite.Open(":memory:"), orm.Config{MaxIdleConns: 1, MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	bc := &conf.Bootstrap{Server: conf.Server{HTTP: conf.ServerHTTP{Passcode: testPasscode}}}
	bc.Runtime.BuildVersion = testBuildVersion
	kcore := NewKanbanCore(db)
	mcpAPI, stopMCP := NewMCP(bc, kcore)
	t.Cleanup(stopMCP)

	limiter := newAttemptLimiter(nil)
	guard := newPasscodeGuard(bc, limiter)
	g := gin.New()
	g.Use(passcodeMiddleware(guard))
	g.Any(mcpPath, gin.WrapH(mcpAPI.Handler()))

	srv := httptest.NewServer(g)
	t.Cleanup(srv.Close)
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`
	req, _ := http.NewRequest(http.MethodPost, srv.URL+mcpPath+"?name=codex", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("无 Bearer 应 401: %d", resp.StatusCode)
	}
	req2, _ := http.NewRequest(http.MethodPost, srv.URL+mcpPath+"?name=codex", strings.NewReader(body))
	req2.Header.Set("Authorization", "Bearer "+testPasscode)
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Accept", "application/json, text/event-stream")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusAccepted {
		t.Fatalf("带 Bearer 应能握手: %d", resp2.StatusCode)
	}
}
