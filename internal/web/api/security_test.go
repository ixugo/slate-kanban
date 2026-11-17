package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"slate-kanban/internal/conf"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// miniEngine 只挂安全中间件，用来单测 Host/Origin。
func miniEngine(bc *conf.Bootstrap) *gin.Engine {
	g := gin.New()
	g.Use(securityMiddleware(bc))
	g.GET("/api/v1/ping", func(c *gin.Context) { c.Status(http.StatusOK) })
	g.POST("/api/v1/ping", func(c *gin.Context) { c.Status(http.StatusOK) })
	return g
}

func TestSecurityHostWhitelist(t *testing.T) {
	bc := &conf.Bootstrap{Server: conf.Server{HTTP: conf.ServerHTTP{AllowedHosts: []string{"kanban.corp"}}}}
	g := miniEngine(bc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	req.Host = "evil.com"
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("evil Host 应 403: %d", rec.Code)
	}
	req.Host = "kanban.corp"
	rec2 := httptest.NewRecorder()
	g.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusOK {
		t.Fatalf("白名单 Host 应通过: %d", rec2.Code)
	}
	req.Host = "127.0.0.1:7070"
	rec3 := httptest.NewRecorder()
	g.ServeHTTP(rec3, req)
	if rec3.Code != http.StatusOK {
		t.Fatalf("IP Host 应通过: %d", rec3.Code)
	}
}

func TestSecurityOriginBlock(t *testing.T) {
	g := miniEngine(&conf.Bootstrap{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ping", strings.NewReader(`{}`))
	req.Host = "localhost"
	req.Header.Set("Origin", "http://evil.com")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("跨站写应 403: %d", rec.Code)
	}
	req.Header.Set("Origin", "http://localhost")
	rec2 := httptest.NewRecorder()
	g.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusOK {
		t.Fatalf("同源写应通过: %d", rec2.Code)
	}
}

func TestSecurityOriginDefaultPort(t *testing.T) {
	g := miniEngine(&conf.Bootstrap{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ping", strings.NewReader(`{}`))
	req.Host = "localhost"
	req.Header.Set("Origin", "http://localhost:80")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Origin 带 :80 应与 Host 无端口视为同源: %d", rec.Code)
	}
	req.Host = "127.0.0.1"
	req.Header.Set("Origin", "https://127.0.0.1:443")
	rec2 := httptest.NewRecorder()
	g.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusOK {
		t.Fatalf("Origin 带 :443 应与 Host 无端口视为同源: %d", rec2.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	g := miniEngine(&conf.Bootstrap{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
	if rec.Header().Get("X-Frame-Options") != "DENY" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("安全响应头缺失")
	}
}
