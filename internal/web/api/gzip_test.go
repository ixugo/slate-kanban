package api

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// gzipEngine 只挂压缩中间件和几个典型响应：大 JSON、小 JSON、图片、SVG、事件流和已压缩响应。
func gzipEngine() *gin.Engine {
	g := gin.New()
	g.Use(gzipMiddleware())
	big := strings.Repeat(`{"title":"一张卡片"},`, 400)
	g.GET("/big", func(c *gin.Context) { c.Data(http.StatusOK, "application/json; charset=utf-8", []byte("["+big+"{}]")) })
	g.GET("/small", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	g.GET("/image", func(c *gin.Context) { c.Data(http.StatusOK, "image/webp", []byte(big)) })
	g.GET("/svg", func(c *gin.Context) { c.Data(http.StatusOK, "image/svg+xml", []byte(big)) })
	g.GET("/stream", func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		c.Writer.WriteString("data: 1\n\n")
		c.Writer.Flush()
	})
	g.GET("/encoded", func(c *gin.Context) {
		c.Header("Content-Encoding", "gzip")
		c.Data(http.StatusOK, "application/json", []byte(big))
	})
	g.GET("/empty", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	g.GET("/range", func(c *gin.Context) { c.Data(http.StatusPartialContent, "application/json", []byte(big)) })
	return g
}

// doGzip 发一个带 Accept-Encoding 的请求并返回响应。
func doGzip(g *gin.Engine, method, path, acceptEncoding string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if acceptEncoding != "" {
		req.Header.Set("Accept-Encoding", acceptEncoding)
	}
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
	return rec
}

// TestGzipCompressesText 验证大 JSON 和 SVG 在客户端支持时被压缩，解压后和原文一致，并带 Vary。
func TestGzipCompressesText(t *testing.T) {
	g := gzipEngine()
	plain := doGzip(g, http.MethodGet, "/big", "")
	for _, path := range []string{"/big", "/svg"} {
		rec := doGzip(g, http.MethodGet, path, "gzip, deflate, br")
		if rec.Header().Get("Content-Encoding") != "gzip" || !strings.Contains(rec.Header().Get("Vary"), "Accept-Encoding") {
			t.Fatalf("%s 应该被压缩并带 Vary: %v", path, rec.Header())
		}
		if rec.Header().Get("Content-Length") != "" {
			t.Fatalf("%s 压缩后不应保留原 Content-Length", path)
		}
		zr, err := gzip.NewReader(rec.Body)
		if err != nil {
			t.Fatalf("%s 不是合法 gzip: %v", path, err)
		}
		got, _ := io.ReadAll(zr)
		if path == "/big" && string(got) != plain.Body.String() {
			t.Fatalf("解压后内容和原文不一致")
		}
		if rec.Body.Len() >= len(got)/4 {
			t.Fatalf("%s 压缩率太差: %d -> %d", path, len(got), rec.Body.Len())
		}
	}
}

// TestGzipSkips 验证不该压缩的情况都原样返回：客户端不支持、已压缩格式、事件流、已有编码、小响应、HEAD、无正文状态和分段响应。
func TestGzipSkips(t *testing.T) {
	g := gzipEngine()
	cases := []struct{ name, method, path, accept string }{
		{"客户端不支持", http.MethodGet, "/big", ""},
		{"图片", http.MethodGet, "/image", "gzip"},
		{"事件流", http.MethodGet, "/stream", "gzip"},
		{"小响应", http.MethodGet, "/small", "gzip"},
		{"HEAD", http.MethodHead, "/big", "gzip"},
		{"无正文", http.MethodGet, "/empty", "gzip"},
		{"分段响应", http.MethodGet, "/range", "gzip"},
	}
	for _, tc := range cases {
		rec := doGzip(g, tc.method, tc.path, tc.accept)
		if rec.Header().Get("Content-Encoding") != "" {
			t.Fatalf("%s 不应压缩: %v", tc.name, rec.Header())
		}
	}
	if rec := doGzip(g, http.MethodGet, "/small", "gzip"); rec.Body.String() != `{"ok":true}` {
		t.Fatalf("小响应内容应原样返回: %q", rec.Body.String())
	}
	if rec := doGzip(g, http.MethodGet, "/empty", "gzip"); rec.Code != http.StatusNoContent {
		t.Fatalf("无正文状态码应保留: %d", rec.Code)
	}
	if rec := doGzip(g, http.MethodGet, "/range", "gzip"); rec.Code != http.StatusPartialContent {
		t.Fatalf("分段响应状态码应保留: %d", rec.Code)
	}
	if rec := doGzip(g, http.MethodGet, "/stream", "gzip"); rec.Body.String() != "data: 1\n\n" {
		t.Fatalf("事件流应原样且立即输出: %q", rec.Body.String())
	}
	// 已带 Content-Encoding 的响应不能被二次压缩
	rec := doGzip(g, http.MethodGet, "/encoded", "gzip")
	if strings.Count(rec.Header().Get("Content-Encoding"), "gzip") != 1 || rec.Body.Len() != len(strings.Repeat(`{"title":"一张卡片"},`, 400)) {
		t.Fatalf("已有编码的响应应保持原样: %v", rec.Header())
	}
}
