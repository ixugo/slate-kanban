package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// miniEngine 只挂安全中间件，用来单测 Origin 和响应头。
func miniEngine() *gin.Engine {
	g := gin.New()
	g.Use(securityMiddleware())
	g.GET("/api/v1/ping", func(c *gin.Context) { c.Status(http.StatusOK) })
	g.POST("/api/v1/ping", func(c *gin.Context) { c.Status(http.StatusOK) })
	return g
}

func TestSecurityAnyHostAllowed(t *testing.T) {
	g := miniEngine()
	for _, host := range []string{"gowvp.golang.space:7070", "kanban.corp", "127.0.0.1:7070", "localhost"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		g.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("Host %s 应通过: %d", host, rec.Code)
		}
	}
}

func TestSecurityOriginBlock(t *testing.T) {
	g := miniEngine()
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
	g := miniEngine()
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
	g := miniEngine()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
	if rec.Header().Get("X-Frame-Options") != "DENY" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("安全响应头缺失")
	}
}
