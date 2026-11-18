package spa

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
)

func testFS(t *testing.T) fstest.MapFS {
	t.Helper()
	return fstest.MapFS{
		"index.html":           {Data: []byte("<html>hi</html>")},
		"assets/app.js":        {Data: []byte("console.log(1)")},
		"manifest.webmanifest": {Data: []byte(`{"name":"Slate"}`)},
		"sw.js":                {Data: []byte("// sw")},
		"icons/icon.png":       {Data: []byte{0x89, 0x50, 0x4e, 0x47}},
	}
}

func TestSPAFileAndFallback(t *testing.T) {
	h := NewHandler(testFS(t))
	req := httptest.NewRequest(http.MethodGet, "/b/3", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<html>") {
		t.Fatalf("应回退 index.html: %d %s", rec.Code, rec.Body)
	}
}

func TestAPILikePath(t *testing.T) {
	if !apiLikePath("/api/v1/x") || !apiLikePath("/mcp") || apiLikePath("/b/1") {
		t.Fatal("apiLikePath 分类不对")
	}
}

func TestGzipAndETag(t *testing.T) {
	h := NewHandler(testFS(t))
	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("应 gzip: %v", rec.Header())
	}
	etag := rec.Header().Get("ETag")
	req2 := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	req2.Header.Set("Accept-Encoding", "gzip")
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNotModified {
		t.Fatalf("应 304: %d", rec2.Code)
	}
}

func TestAssetsCacheControl(t *testing.T) {
	h := NewHandler(testFS(t))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	if !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Fatal("assets 应长期缓存")
	}
}

func TestStubRoot(t *testing.T) {
	h := NewHandler(nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	body, _ := io.ReadAll(rec.Body)
	if rec.Code != http.StatusOK || !strings.Contains(string(body), "embed_web") {
		t.Fatalf("无 dist 时根路径提示: %d %s", rec.Code, body)
	}
}

// TestManifestContentType manifest 要以 application/manifest+json 返回，浏览器才认它。
func TestManifestContentType(t *testing.T) {
	h := NewHandler(testFS(t))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/manifest.webmanifest", nil))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/manifest+json" {
		t.Fatalf("manifest 类型不对: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
}

// TestPngContentType 图标 png 要带 image/png 类型。
func TestPngContentType(t *testing.T) {
	h := NewHandler(testFS(t))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/icons/icon.png", nil))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("png 类型不对: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
}

// TestSwAndManifestNoCache sw.js 和 manifest 不能被长期缓存，否则应用升级后浏览器拿不到新版本。
func TestSwAndManifestNoCache(t *testing.T) {
	h := NewHandler(testFS(t))
	for _, path := range []string{"/sw.js", "/manifest.webmanifest"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-cache" {
			t.Fatalf("%s 应 no-cache: %v", path, rec.Header())
		}
	}
}

// TestMissingSwAndIconsNotSPA sw.js 和图标缺失时必须返回 404，不能回退成 index.html，
// 否则浏览器会把 HTML 当成 Service Worker 脚本去解析而报错。
func TestMissingSwAndIconsNotSPA(t *testing.T) {
	h := NewHandler(fstest.MapFS{
		"index.html": {Data: []byte("<html>hi</html>")},
	})
	for _, path := range []string{"/sw.js", "/icons/missing.png"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "<html>") {
			t.Fatalf("%s 应 404 且非 HTML: %d %s", path, rec.Code, rec.Body.String())
		}
	}
}

// TestWellKnownNotSPA 授权发现地址（/.well-known/）必须返回 404，不能回退成 index.html。
// MCP 客户端收到 401 后会去这些地址找授权服务器的元数据，拿到 HTML 会按 JSON 解析失败，
// 用户看到的是「SyntaxError」，而不是「需要口令」。
func TestWellKnownNotSPA(t *testing.T) {
	h := NewHandler(fstest.MapFS{
		"index.html": {Data: []byte("<html>hi</html>")},
	})
	paths := []string{
		"/.well-known/oauth-protected-resource",
		"/.well-known/oauth-protected-resource/mcp",
		"/.well-known/oauth-authorization-server",
		"/.well-known/openid-configuration/mcp",
	}
	for _, path := range paths {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "<html>") {
			t.Fatalf("%s 应 404 且非 HTML: %d %s", path, rec.Code, rec.Body.String())
		}
	}
}

// TestMCPSubpathsNotSPA /mcp/ 下面的路径属于后端，没匹配上时要返回 JSON 404；
// 客户端也会在 /mcp/.well-known/ 下找授权元数据。
func TestMCPSubpathsNotSPA(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.NoRoute(NoRouteHandler(fstest.MapFS{"index.html": {Data: []byte("<html>hi</html>")}}))
	for _, path := range []string{"/mcp/.well-known/oauth-protected-resource", "/mcp/.well-known/openid-configuration", "/mcp/other"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "<html>") {
			t.Fatalf("%s 应 404 且非 HTML: %d %s", path, rec.Code, rec.Body.String())
		}
	}
	// 普通的前端路由不受影响，仍回退到 index.html
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/b/1", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<html>") {
		t.Fatalf("前端路由应回退 index.html: %d %s", rec.Code, rec.Body.String())
	}
}
