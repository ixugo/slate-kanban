package spa

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func testFS(t *testing.T) fstest.MapFS {
	t.Helper()
	return fstest.MapFS{
		"index.html":    {Data: []byte("<html>hi</html>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
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
