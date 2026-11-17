// Package apitest 是 REST 接口测试的公共工具：发请求、读响应，不依赖真实的网络端口。
package apitest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
	Code int
	Body []byte
}

// Do 发一个请求。token 为空就不带 Authorization；body 不是 nil 时序列化成 JSON，并带上 JSON 的 Content-Type。
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
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return c.Serve(req)
}

// Serve 处理一个已经构造好的请求，需要自定义请求头或请求体时用它。
func (c Client) Serve(req *http.Request) Response {
	rec := httptest.NewRecorder()
	c.Engine.ServeHTTP(rec, req)
	return Response{Code: rec.Code, Body: rec.Body.Bytes()}
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
