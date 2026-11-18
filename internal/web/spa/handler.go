package spa

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// stubMessage 在没有嵌入前端时根路径展示的说明，提示如何构建。
const stubMessage = "网页没有嵌入，请先在 web/ 下 npm run build，再用 go build -tags embed_web 构建"

// Handler 在静态文件系统上提供 gzip 缓存、ETag 和 SPA 回退。
type Handler struct {
	root     fs.FS
	hasIndex bool
	cache    sync.Map // 路径 -> *cachedFile
}

type cachedFile struct {
	raw       []byte
	gzip      []byte
	rawETag   string
	gzipETag  string
	ctype     string
	immutable bool
}

// NewHandler 用给定的 FS 创建处理器；会探测是否存在 index.html。
func NewHandler(root fs.FS) *Handler {
	h := &Handler{root: root}
	if root != nil {
		if _, err := fs.Stat(root, "index.html"); err == nil {
			h.hasIndex = true
		}
	}
	return h
}

// NoRouteHandler 供 gin 的 NoRoute 使用：API 类路径保持 JSON 404，其余走静态或 SPA 回退。
func NoRouteHandler(root fs.FS) gin.HandlerFunc {
	h := NewHandler(root)
	return func(c *gin.Context) {
		if apiLikePath(c.Request.URL.Path) {
			c.JSON(http.StatusNotFound, "来到了无人的荒漠")
			return
		}
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// ServeHTTP 只处理 GET/HEAD；有文件则返回，否则在允许时回退 index.html。
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}
	if h.root == nil || !h.hasIndex {
		if r.URL.Path == "/" || r.URL.Path == "" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			w.WriteHeader(http.StatusOK)
			if r.Method != http.MethodHead {
				_, _ = io.WriteString(w, stubMessage)
			}
			return
		}
		http.NotFound(w, r)
		return
	}
	rel, ok := safeRelPath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if rel == "" {
		rel = "index.html"
	}
	data, ok := h.loadFile(rel)
	if !ok && !apiLikePath(r.URL.Path) && rel != "index.html" {
		data, ok = h.loadFile("index.html")
		if ok {
			rel = "index.html"
		}
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	h.writeFile(w, r, rel, data)
}

// safeRelPath 把 URL 路径转成 FS 内相对路径，拒绝穿越。
func safeRelPath(urlPath string) (string, bool) {
	clean := path.Clean(urlPath)
	if strings.Contains(clean, "..") {
		return "", false
	}
	return strings.TrimPrefix(clean, "/"), true
}

// loadFile 读取并缓存可压缩文件的 gzip 副本。
func (h *Handler) loadFile(name string) (*cachedFile, bool) {
	if v, ok := h.cache.Load(name); ok {
		return v.(*cachedFile), true
	}
	raw, err := fs.ReadFile(h.root, name)
	if err != nil {
		return nil, false
	}
	cf := &cachedFile{
		raw: raw, ctype: contentType(name),
		rawETag:   `"` + hashETag(raw) + `"`,
		immutable: strings.HasPrefix(name, "assets/"),
	}
	if compressible(name) {
		if gz, err := gzipBytes(raw); err == nil {
			cf.gzip = gz
			cf.gzipETag = `"` + hashETag(gz) + "-gzip" + `"`
		}
	}
	h.cache.Store(name, cf)
	return cf, true
}

// writeFile 按 Accept-Encoding 写响应，处理 304 和缓存头。
func (h *Handler) writeFile(w http.ResponseWriter, r *http.Request, name string, cf *cachedFile) {
	useGzip := cf.gzip != nil && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
	etag := cf.rawETag
	body := cf.raw
	if useGzip {
		etag = cf.gzipETag
		body = cf.gzip
	}
	if inm := r.Header.Get("If-None-Match"); inm == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", cf.ctype)
	w.Header().Set("ETag", etag)
	w.Header().Set("Vary", "Accept-Encoding")
	if cf.immutable {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	if useGzip {
		w.Header().Set("Content-Encoding", "gzip")
	}
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(body)
}

// gzipBytes 用最高压缩级别压一次，结果会缓存在内存里。
func gzipBytes(raw []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(raw); err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// hashETag 用内容算 ETag 主体。
func hashETag(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:16])
}

// contentType 按扩展名给 MIME，未知则用 octet-stream。
func contentType(name string) string {
	switch path.Ext(name) {
	case ".html":
		return "text/html; charset=utf-8"
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".json":
		return "application/json; charset=utf-8"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".map":
		return "application/json; charset=utf-8"
	case ".webmanifest":
		return "application/manifest+json"
	default:
		return "application/octet-stream"
	}
}

// compressible 判断是否需要预压 gzip。
func compressible(name string) bool {
	switch path.Ext(name) {
	case ".html", ".js", ".mjs", ".css", ".svg", ".json", ".txt", ".map", ".webmanifest":
		return true
	default:
		return false
	}
}
