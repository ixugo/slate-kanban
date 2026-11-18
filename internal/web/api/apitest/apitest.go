// Package apitest 是 REST 接口测试的公共工具：发请求、读响应，不依赖真实的网络端口。
package apitest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// Client 直接把请求交给 gin 引擎处理，速度快，也不占端口。
type Client struct {
	Engine *gin.Engine
}

// Response 是一次请求的结果。
type Response struct {
	Code    int
	Body    []byte
	Headers http.Header
}

// Do 发一个请求。body 不是 nil 时序列化成 JSON 并带上 Content-Type。
// token 保留参数以兼容旧测试：非空时在 MCP 路径上加 ?name= 或 &role=manager，REST 不再使用令牌。
func (c Client) Do(t *testing.T, method, path, token string, body any) Response {
	t.Helper()
	var reader io.Reader = http.NoBody
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("序列化请求体失败: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	finalPath := path
	if token != "" {
		finalPath = withMCPIdentity(path, token)
	}
	req := httptest.NewRequest(method, finalPath, reader)
	req.Host = "localhost"
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.Serve(req)
}

// withMCPIdentity 给 /mcp 路径追加身份查询参数；管理占位串映射为 manager 视图。
func withMCPIdentity(path, token string) string {
	if !strings.HasPrefix(path, "/mcp") {
		return path
	}
	u, err := url.Parse(path)
	if err != nil {
		return path
	}
	q := u.Query()
	switch token {
	case "manager", "router-test-admin-token-0123456789", "admin-secret":
		q.Set("role", "manager")
	default:
		q.Set("name", token)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// Serve 处理一个已经构造好的请求，需要自定义请求头或请求体时用它。
func (c Client) Serve(req *http.Request) Response {
	// 测试请求默认带 localhost，避免 Host 白名单中间件误拦。
	if req.Host == "" {
		req.Host = "localhost"
	}
	rec := httptest.NewRecorder()
	c.Engine.ServeHTTP(rec, req)
	return Response{Code: rec.Code, Body: rec.Body.Bytes(), Headers: rec.Header()}
}

// JSON 把响应体解析到 v 里，解析不了就让测试终止。
func (r Response) JSON(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, v); err != nil {
		t.Fatalf("响应不是合法 JSON: %v\n%s", err, r.Body)
	}
}

// Reason 返回错误响应里的 reason 字段，响应不是错误格式时返回空串。
func (r Response) Reason() string {
	var out struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(r.Body, &out); err != nil {
		return ""
	}
	return out.Reason
}
