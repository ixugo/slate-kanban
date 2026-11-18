package api

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ixugo/goddd/pkg/web"

	"slate-kanban/internal/web/api/apitest"
)

// guardClient 搭一个挂了两个防护中间件的服务，处理函数把读到的请求体长度原样返回，
// 同时统计自己被执行了几次，用来确认被拦下的请求没有进到处理函数。
func guardClient(limit int64) (apitest.Client, *int) {
	calls := new(int)
	engine := gin.New()
	engine.Any("/echo", limitBody(limit), requireJSON(), func(c *gin.Context) {
		*calls++
		raw, err := io.ReadAll(c.Request.Body)
		if err != nil {
			web.Fail(c, errBodyTooLarge)
			return
		}
		c.JSON(http.StatusOK, gin.H{"length": len(raw)})
	})
	return apitest.Client{Engine: engine}, calls
}

// request 构造一个带指定 Content-Type 和请求体的请求，contentType 为空就不设置。
func request(method, contentType, body string) *http.Request {
	req := httptest.NewRequest(method, "/echo", strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req
}

// TestLimitBody 验证请求体上限：声明的长度超过就返回 413；刚好等于上限放行；
// 没有声明长度的超大请求体读不完整，处理函数得到错误。
func TestLimitBody(t *testing.T) {
	c, calls := guardClient(16)
	over := c.Serve(request(http.MethodPost, "application/json", strings.Repeat("a", 17)))
	if over.Code != http.StatusRequestEntityTooLarge || over.Reason() != "ErrContentTooLarge" {
		t.Errorf("超过上限应该返回 413: %d %s", over.Code, over.Body)
	}
	if *calls != 0 {
		t.Errorf("声明的长度超过上限，不应该进到处理函数")
	}
	if ok := c.Serve(request(http.MethodPost, "application/json", strings.Repeat("a", 16))); ok.Code != http.StatusOK {
		t.Errorf("刚好等于上限应该放行: %d %s", ok.Code, ok.Body)
	}

	chunked := request(http.MethodPost, "application/json", strings.Repeat("a", 100))
	chunked.ContentLength = -1
	if resp := c.Serve(chunked); resp.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("没有声明长度的超大请求体应该读失败: %d %s", resp.Code, resp.Body)
	}
}

// TestRequireJSON 验证 JSON 检查：有请求体但不是 JSON 返回 415；JSON 带 charset 参数、
// 类型大小写不同都认；没有请求体的请求不受影响；DELETE 带请求体同样检查。
func TestRequireJSON(t *testing.T) {
	c, calls := guardClient(maxRequestBody)
	cases := []struct {
		name        string
		method      string
		contentType string
		body        string
		want        int
	}{
		{"纯文本", http.MethodPost, "text/plain", `{"a":1}`, http.StatusUnsupportedMediaType},
		{"表单", http.MethodPost, "application/x-www-form-urlencoded", "a=1", http.StatusUnsupportedMediaType},
		{"没写类型", http.MethodPatch, "", `{"a":1}`, http.StatusUnsupportedMediaType},
		{"删除带非 JSON", http.MethodDelete, "text/plain", "x", http.StatusUnsupportedMediaType},
		{"标准 JSON", http.MethodPost, "application/json", `{"a":1}`, http.StatusOK},
		{"带 charset", http.MethodPost, "application/json; charset=utf-8", `{"a":1}`, http.StatusOK},
		{"类型大小写不同", http.MethodPut, "Application/JSON", `{"a":1}`, http.StatusOK},
		{"没有请求体", http.MethodPost, "", "", http.StatusOK},
		{"GET 没有请求体", http.MethodGet, "", "", http.StatusOK},
	}
	for _, tc := range cases {
		before := *calls
		resp := c.Serve(request(tc.method, tc.contentType, tc.body))
		if resp.Code != tc.want {
			t.Errorf("%s：期望 %d，实际 %d %s", tc.name, tc.want, resp.Code, resp.Body)
		}
		if ran := *calls > before; ran != (tc.want == http.StatusOK) {
			t.Errorf("%s：处理函数是否被执行不对: %v", tc.name, ran)
		}
	}
	if resp := c.Serve(request(http.MethodPost, "text/plain", "x")); resp.Reason() != "ErrUnsupportedMediaType" {
		t.Errorf("415 应该带统一的错误格式: %s", resp.Body)
	}
}

// TestGuardRequestBodyReachesHandler 验证放行的请求体内容完整，防护中间件没有吃掉数据。
func TestGuardRequestBodyReachesHandler(t *testing.T) {
	c, _ := guardClient(maxRequestBody)
	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader([]byte(`{"hello":"世界"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp := c.Serve(req)
	if resp.Code != http.StatusOK || !strings.Contains(string(resp.Body), `"length":18`) {
		t.Errorf("处理函数应该读到完整的 18 个字节: %d %s", resp.Code, resp.Body)
	}
}

// TestUploadRouteGuards 验证图片上传路由：可以收 multipart 表单和超过 1 MiB 的请求体，
// 但超过图片上限的仍然 413；别的路由不受影响，仍然只收 JSON 且不超过 1 MiB。
func TestUploadRouteGuards(t *testing.T) {
	engine := gin.New()
	handler := func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{}) }
	engine.POST(uploadRoute, limitBody(maxRequestBody), requireJSON(), handler)
	engine.POST("/api/v1/boards/:board_id/cards", limitBody(maxRequestBody), requireJSON(), handler)
	c := apitest.Client{Engine: engine}
	post := func(path, contentType string, size int) apitest.Response {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(strings.Repeat("a", size)))
		req.Header.Set("Content-Type", contentType)
		return c.Serve(req)
	}

	form := "multipart/form-data; boundary=x"
	if resp := post("/api/v1/boards/1/attachments", form, 3<<20); resp.Code != http.StatusOK {
		t.Errorf("上传应该允许表单和 3 MiB 的请求体: %d %s", resp.Code, resp.Body)
	}
	if resp := post("/api/v1/boards/1/attachments", form, uploadBodyLimit+1); resp.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("超过图片上限应该 413: %d %s", resp.Code, resp.Body)
	}
	if resp := post("/api/v1/boards/1/cards", form, 10); resp.Code != http.StatusUnsupportedMediaType {
		t.Errorf("别的路由不能收表单: %d %s", resp.Code, resp.Body)
	}
	if resp := post("/api/v1/boards/1/cards", "application/json", 2<<20); resp.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("别的路由仍然限制 1 MiB: %d %s", resp.Code, resp.Body)
	}
}
