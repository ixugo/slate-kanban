package api

import (
	"compress/gzip"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// gzipMinSize 是值得压缩的最小响应体，太小的响应压缩后几乎不变甚至更大，还白白耗 CPU。
const gzipMinSize = 1024

// gzipPool 复用 gzip 写入器，每个压缩响应都新建一个会分配约 1MB 内存。
var gzipPool = sync.Pool{New: func() any { return gzip.NewWriter(nil) }}

// gzipMiddleware 给文本类响应（JSON、HTML、JS、CSS、SVG 等）做 gzip 压缩。
// 看板和卡片列表是几百 KB 的 JSON，不压缩时慢网络下首屏很慢。
// PNG、JPEG、WebP 等图片本身已压缩，事件流要即时输出，它们都原样返回；
// 已经自带 Content-Encoding 的响应（例如网页静态文件）也不会被二次压缩。
func gzipMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodHead || !strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
			c.Next()
			return
		}
		gw := &gzipWriter{ResponseWriter: c.Writer}
		c.Writer = gw
		defer gw.finish()
		c.Next()
	}
}

// 响应的压缩决定：还没决定（先缓冲）、原样输出、gzip 输出。
const (
	gzipUndecided = iota
	gzipPassthrough
	gzipActive
)

// gzipWriter 包住 gin 的响应写入器。第一次写正文时按响应头决定是否压缩；
// 可压缩类型的响应先缓冲到 gzipMinSize，不够大就原样输出。
type gzipWriter struct {
	gin.ResponseWriter
	zw     *gzip.Writer
	mode   int
	status int
	buf    []byte
}

// WriteHeader 只记录状态码，等决定了是否压缩再真正写出，因为压缩要改响应头。
func (w *gzipWriter) WriteHeader(code int) {
	if w.mode == gzipUndecided {
		w.status = code
		return
	}
	w.ResponseWriter.WriteHeader(code)
}

// WriteHeaderNow 在还没决定时不能提前写出响应头，否则后面没法再加 Content-Encoding。
func (w *gzipWriter) WriteHeaderNow() {
	if w.mode != gzipUndecided {
		w.ResponseWriter.WriteHeaderNow()
	}
}

// Status 在响应头还没真正写出时返回记录的状态码，让日志等外层中间件读到正确的值。
func (w *gzipWriter) Status() int {
	if w.mode == gzipUndecided && w.status != 0 {
		return w.status
	}
	return w.ResponseWriter.Status()
}

// Written 把缓冲中的内容也算作已写。
func (w *gzipWriter) Written() bool {
	return len(w.buf) > 0 || w.ResponseWriter.Written()
}

// WriteString 和 Write 走同一条路径。
func (w *gzipWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

// Write 第一次调用时按响应头决定模式；压缩模式下写入 gzip，缓冲模式下攒到够大再决定。
func (w *gzipWriter) Write(p []byte) (int, error) {
	if w.mode == gzipUndecided && !w.canCompress() {
		w.passthrough()
	}
	switch w.mode {
	case gzipPassthrough:
		return w.ResponseWriter.Write(p)
	case gzipActive:
		return w.zw.Write(p)
	}
	w.buf = append(w.buf, p...)
	if len(w.buf) >= gzipMinSize {
		if err := w.startGzip(); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

// Flush 被要求立即输出时不能再缓冲：还没决定就按已有内容决定，压缩模式下把压缩器里的数据也冲出去。
func (w *gzipWriter) Flush() {
	if w.mode == gzipUndecided {
		if len(w.buf) == 0 && !w.canCompress() {
			w.passthrough()
		} else if len(w.buf) > 0 {
			_ = w.startGzip()
		}
	}
	if w.mode == gzipActive {
		_ = w.zw.Flush()
	}
	w.ResponseWriter.Flush()
}

// canCompress 判断响应头是否允许压缩：正常正文、没有编码、没有分段、类型是文本类。
func (w *gzipWriter) canCompress() bool {
	h := w.Header()
	status := w.status
	if status == 0 {
		status = http.StatusOK
	}
	if status != http.StatusOK || h.Get("Content-Encoding") != "" || h.Get("Content-Range") != "" {
		return false
	}
	return compressibleType(h.Get("Content-Type"))
}

// compressibleType 只压缩文本类型；事件流虽然是 text/ 开头，但必须即时输出，要排除。
func compressibleType(ct string) bool {
	ct = strings.ToLower(ct)
	if strings.HasPrefix(ct, "text/event-stream") {
		return false
	}
	for _, p := range []string{"text/", "application/json", "application/javascript", "application/manifest+json", "application/xml", "image/svg+xml"} {
		if strings.HasPrefix(ct, p) {
			return true
		}
	}
	return false
}

// passthrough 切换为原样输出，并把之前记录的状态码和缓冲内容交给底层。
func (w *gzipWriter) passthrough() {
	w.mode = gzipPassthrough
	if w.status != 0 {
		w.ResponseWriter.WriteHeader(w.status)
	}
	if len(w.buf) > 0 {
		_, _ = w.ResponseWriter.Write(w.buf)
		w.buf = nil
	}
}

// startGzip 改响应头并切换为压缩模式，把缓冲内容压缩写入。
func (w *gzipWriter) startGzip() error {
	h := w.Header()
	h.Set("Content-Encoding", "gzip")
	h.Add("Vary", "Accept-Encoding")
	h.Del("Content-Length")
	if etag := h.Get("ETag"); etag != "" && !strings.HasPrefix(etag, "W/") {
		h.Set("ETag", "W/"+etag)
	}
	if w.status != 0 {
		w.ResponseWriter.WriteHeader(w.status)
	}
	w.mode = gzipActive
	w.zw = gzipPool.Get().(*gzip.Writer)
	w.zw.Reset(w.ResponseWriter)
	_, err := w.zw.Write(w.buf)
	w.buf = nil
	return err
}

// finish 在处理结束时收尾：压缩模式关闭压缩器并归还；缓冲模式说明响应太小，原样输出；
// 还没写过正文的响应（例如只有状态码）把记录的状态码交给底层。
func (w *gzipWriter) finish() {
	switch w.mode {
	case gzipActive:
		_ = w.zw.Close()
		w.zw.Reset(nil)
		gzipPool.Put(w.zw)
	case gzipUndecided:
		w.passthrough()
	}
}
