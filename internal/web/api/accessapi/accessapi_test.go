package accessapi_test

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"slate-kanban/internal/core/access/accesstest"
	"slate-kanban/internal/web/api/accessapi"
	"slate-kanban/internal/web/api/apitest"
)

const adminToken = "admin-secret"

// newClient 搭一个只有 /me 和令牌管理的服务，和正式路由一样挂了认证中间件。
func newClient(t *testing.T) apitest.Client {
	t.Helper()
	api := accessapi.New(accesstest.NewCore(t, adminToken))
	engine := gin.New()
	accessapi.Register(engine.Group("/api/v1", api.Middleware()), api)
	return apitest.Client{Engine: engine}
}

// created 是创建令牌接口的响应。
type created struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Prefix string `json:"prefix"`
	Token  string `json:"token"`
}

// mustCreate 用管理令牌创建一个 agent 令牌，失败就让测试终止。
func mustCreate(t *testing.T, c apitest.Client, name string) created {
	t.Helper()
	resp := c.Do(t, http.MethodPost, "/api/v1/agent-tokens", adminToken, map[string]string{"name": name})
	if resp.Code != http.StatusOK {
		t.Fatalf("创建令牌 %s 失败: %d %s", name, resp.Code, resp.Body)
	}
	var out created
	resp.JSON(t, &out)
	return out
}

// TestMiddlewareRejects 验证认证中间件：没有请求头、令牌错误、方案不对都返回 401，
// 并且带着统一的错误格式，处理函数不会被执行。
func TestMiddlewareRejects(t *testing.T) {
	c := newClient(t)
	cases := map[string]string{"没有令牌": "", "令牌错误": "wrong"}
	for name, token := range cases {
		resp := c.Do(t, http.MethodGet, "/api/v1/me", token, nil)
		if resp.Code != http.StatusUnauthorized || resp.Reason() != "ErrUnauthorized" {
			t.Errorf("%s应该返回 401: %d %s", name, resp.Code, resp.Body)
		}
	}
	req, err := http.NewRequest(http.MethodGet, "/api/v1/me", http.NoBody)
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}
	req.Header.Set("Authorization", "Basic "+adminToken)
	if resp := c.Serve(req); resp.Code != http.StatusUnauthorized {
		t.Errorf("方案不对应该返回 401: %d", resp.Code)
	}
}

// TestMe 验证当前身份：管理令牌是人，名称 admin；agent 令牌是 agent，名称是令牌绑定的名称。
func TestMe(t *testing.T) {
	c := newClient(t)
	var me struct{ Name, Kind string }
	resp := c.Do(t, http.MethodGet, "/api/v1/me", adminToken, nil)
	resp.JSON(t, &me)
	if resp.Code != http.StatusOK || me.Name != "admin" || me.Kind != "human" {
		t.Fatalf("管理令牌应该是人: %d %+v", resp.Code, me)
	}
	tok := mustCreate(t, c, "claude")
	resp = c.Do(t, http.MethodGet, "/api/v1/me", tok.Token, nil)
	resp.JSON(t, &me)
	if resp.Code != http.StatusOK || me.Name != "claude" || me.Kind != "agent" {
		t.Fatalf("agent 令牌应该是 agent: %d %+v", resp.Code, me)
	}
}

// TestTokenLifecycle 验证令牌的创建、列出、删除：明文只在创建时出现，列表里没有哈希和明文，
// 删除后令牌立刻失效，重复删除也返回成功，重名返回 409，名称不合法返回 400。
func TestTokenLifecycle(t *testing.T) {
	c := newClient(t)
	tok := mustCreate(t, c, "claude")
	if !strings.HasPrefix(tok.Token, "slate_") || tok.ID == 0 || tok.Prefix != tok.Token[:12] {
		t.Fatalf("创建的响应不对: %+v", tok)
	}

	resp := c.Do(t, http.MethodGet, "/api/v1/agent-tokens", adminToken, nil)
	var items []map[string]any
	resp.JSON(t, &items)
	if resp.Code != http.StatusOK || len(items) != 1 || items[0]["name"] != "claude" ||
		items[0]["token"] != nil || items[0]["hash"] != nil {
		t.Fatalf("列表里不应该有明文和哈希: %d %s", resp.Code, resp.Body)
	}

	if resp := c.Do(t, http.MethodPost, "/api/v1/agent-tokens", adminToken, map[string]string{"name": "claude"}); resp.Code != http.StatusConflict {
		t.Errorf("重名应该返回 409: %d %s", resp.Code, resp.Body)
	}
	if resp := c.Do(t, http.MethodPost, "/api/v1/agent-tokens", adminToken, map[string]string{"name": "Bad Name"}); resp.Code != http.StatusBadRequest {
		t.Errorf("名称不合法应该返回 400: %d %s", resp.Code, resp.Body)
	}

	path := "/api/v1/agent-tokens/" + strconv.FormatInt(tok.ID, 10)
	for range 2 {
		if resp := c.Do(t, http.MethodDelete, path, adminToken, nil); resp.Code != http.StatusOK || strings.TrimSpace(string(resp.Body)) != "{}" {
			t.Fatalf("删除应该幂等并返回 {}: %d %s", resp.Code, resp.Body)
		}
	}
	if resp := c.Do(t, http.MethodGet, "/api/v1/me", tok.Token, nil); resp.Code != http.StatusUnauthorized {
		t.Errorf("删除后令牌应该立刻失效: %d", resp.Code)
	}
}

// TestTokenAdminOnly 验证 agent 用自己的令牌管理令牌会被拒绝，并且没有产生任何副作用。
func TestTokenAdminOnly(t *testing.T) {
	c := newClient(t)
	agent := mustCreate(t, c, "claude")
	victim := mustCreate(t, c, "codex")
	path := "/api/v1/agent-tokens"
	for name, resp := range map[string]apitest.Response{
		"创建": c.Do(t, http.MethodPost, path, agent.Token, map[string]string{"name": "mine"}),
		"列出": c.Do(t, http.MethodGet, path, agent.Token, nil),
		"删除": c.Do(t, http.MethodDelete, path+"/"+strconv.FormatInt(victim.ID, 10), agent.Token, nil),
	} {
		if resp.Code != http.StatusForbidden || resp.Reason() != "ErrAdminOnly" {
			t.Errorf("agent %s令牌应该返回 403: %d %s", name, resp.Code, resp.Body)
		}
	}
	if resp := c.Do(t, http.MethodGet, "/api/v1/me", victim.Token, nil); resp.Code != http.StatusOK {
		t.Errorf("被拒绝的删除不应该让令牌失效: %d", resp.Code)
	}
}

// TestTokenIDOnlyFromPath 验证路径里的编号不会被请求体或查询参数里的同名字段覆盖：
// 删除令牌 a 的请求里夹带令牌 b 的编号，结果 a 被删，b 完好。
func TestTokenIDOnlyFromPath(t *testing.T) {
	cases := map[string]func(a, b created) (path string, body any){
		"请求体": func(a, b created) (string, any) {
			return "/api/v1/agent-tokens/" + strconv.FormatInt(a.ID, 10),
				map[string]any{"token_id": b.ID, "TokenID": b.ID, "id": b.ID}
		},
		"查询参数": func(a, b created) (string, any) {
			id := strconv.FormatInt(b.ID, 10)
			return "/api/v1/agent-tokens/" + strconv.FormatInt(a.ID, 10) + "?TokenID=" + id + "&token_id=" + id, nil
		},
	}
	for name, build := range cases {
		c := newClient(t)
		a, b := mustCreate(t, c, "a"), mustCreate(t, c, "b")
		path, body := build(a, b)
		if resp := c.Do(t, http.MethodDelete, path, adminToken, body); resp.Code != http.StatusOK {
			t.Fatalf("%s：删除失败: %d %s", name, resp.Code, resp.Body)
		}
		if resp := c.Do(t, http.MethodGet, "/api/v1/me", a.Token, nil); resp.Code != http.StatusUnauthorized {
			t.Errorf("%s：路径里的令牌 a 应该被删除: %d", name, resp.Code)
		}
		if resp := c.Do(t, http.MethodGet, "/api/v1/me", b.Token, nil); resp.Code != http.StatusOK {
			t.Errorf("%s：夹带的编号不应该生效，令牌 b 被删了: %d", name, resp.Code)
		}
	}
}

// TestBadPathID 验证路径里的编号不是数字时返回 400。
func TestBadPathID(t *testing.T) {
	c := newClient(t)
	if resp := c.Do(t, http.MethodDelete, "/api/v1/agent-tokens/abc", adminToken, nil); resp.Code != http.StatusBadRequest {
		t.Errorf("编号不是数字应该返回 400: %d %s", resp.Code, resp.Body)
	}
}

// TestNoMiddlewareMeansUnauthorized 验证路由忘了挂认证中间件时，处理函数按未认证处理，而不是放行或崩溃。
func TestNoMiddlewareMeansUnauthorized(t *testing.T) {
	api := accessapi.New(accesstest.NewCore(t, adminToken))
	engine := gin.New()
	accessapi.Register(engine.Group("/api/v1"), api)
	c := apitest.Client{Engine: engine}
	for _, path := range []string{"/api/v1/me", "/api/v1/agent-tokens"} {
		if resp := c.Do(t, http.MethodGet, path, adminToken, nil); resp.Code != http.StatusUnauthorized {
			t.Errorf("%s 没有中间件时应该返回 401: %d %s", path, resp.Code, resp.Body)
		}
	}
}
