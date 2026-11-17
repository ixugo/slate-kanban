package kanbanapi_test

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"slate-kanban/internal/core/access"
	"slate-kanban/internal/core/access/accesstest"
	"slate-kanban/internal/core/kanban/kanbantest"
	"slate-kanban/internal/web/api/accessapi"
	"slate-kanban/internal/web/api/apitest"
	"slate-kanban/internal/web/api/kanbanapi"
)

const adminToken = "admin-secret"

// stack 是一套接好内存数据库的完整接口：令牌、看板，路由和正式环境一样挂了认证中间件。
type stack struct {
	apitest.Client
	tokens map[string]string // agent 名称 -> 令牌明文
	access access.Core
}

// newStack 搭一套全新的接口。
func newStack(t *testing.T) *stack {
	t.Helper()
	acore := accesstest.NewCore(t, adminToken)
	aapi := accessapi.New(acore)
	engine := gin.New()
	v1 := engine.Group("/api/v1", aapi.Middleware())
	accessapi.Register(v1, aapi)
	kanbanapi.Register(v1, kanbanapi.New(kanbantest.NewCore(t)))
	return &stack{Client: apitest.Client{Engine: engine}, tokens: map[string]string{}, access: acore}
}

// token 返回名为 name 的 agent 令牌，第一次用到时才创建。
func (s *stack) token(t *testing.T, name string) string {
	t.Helper()
	if tok, ok := s.tokens[name]; ok {
		return tok
	}
	made, err := s.access.CreateToken(t.Context(), access.Principal{Admin: true, Name: access.AdminName}, name)
	if err != nil {
		t.Fatalf("创建令牌 %s 失败: %v", name, err)
	}
	s.tokens[name] = made.Plain
	return made.Plain
}

// human 以人（管理令牌）的身份发请求。
func (s *stack) human(t *testing.T, method, path string, body any) apitest.Response {
	t.Helper()
	return s.Do(t, method, "/api/v1"+path, adminToken, body)
}

// agent 以名为 name 的 agent 的身份发请求。
func (s *stack) agent(t *testing.T, name, method, path string, body any) apitest.Response {
	t.Helper()
	return s.Do(t, method, "/api/v1"+path, s.token(t, name), body)
}

// decode 要求响应是 200，并解析成 T，否则让测试终止。
func decode[T any](t *testing.T, resp apitest.Response) T {
	t.Helper()
	if resp.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d: %s", resp.Code, resp.Body)
	}
	var out T
	resp.JSON(t, &out)
	return out
}

// wantFail 要求响应是指定状态码和 reason 的错误。
func wantFail(t *testing.T, name string, resp apitest.Response, status int, reason string) {
	t.Helper()
	if resp.Code != status || resp.Reason() != reason {
		t.Errorf("%s：期望 %d %s，实际 %d %s", name, status, reason, resp.Code, resp.Body)
	}
}

// wantEmptyObject 要求响应是 200 和 {}。
func wantEmptyObject(t *testing.T, name string, resp apitest.Response) {
	t.Helper()
	if resp.Code != http.StatusOK || strings.TrimSpace(string(resp.Body)) != "{}" {
		t.Errorf("%s：期望 200 和 {}，实际 %d %s", name, resp.Code, resp.Body)
	}
}

// 下面是响应里用到的字段，只列测试关心的。
type (
	column struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		AgentPickup bool   `json:"agent_pickup"`
		IsDone      bool   `json:"is_done"`
	}
	board struct {
		ID          int64            `json:"id"`
		Name        string           `json:"name"`
		Slug        string           `json:"slug"`
		Description string           `json:"description"`
		ProjectPath string           `json:"project_path"`
		Columns     []column         `json:"columns"`
		Rules       []map[string]any `json:"rules"`
	}
	card struct {
		ID          int64    `json:"id"`
		ColumnID    int64    `json:"column_id"`
		Key         string   `json:"key"`
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Priority    int      `json:"priority"`
		Tags        []string `json:"tags"`
		ClaimedBy   string   `json:"claimed_by"`
		Version     int64    `json:"version"`
	}
	comment struct {
		ID     int64  `json:"id"`
		Author string `json:"author"`
		Body   string `json:"body"`
	}
	cardDetail struct {
		card
		ClaimActive       bool      `json:"claim_active"`
		ColumnInstruction string    `json:"column_instruction"`
		Comments          []comment `json:"comments"`
	}
)

// newBoard 以人的身份创建一个 dev 模板的看板。
func (s *stack) newBoard(t *testing.T, slug string) board {
	t.Helper()
	return decode[board](t, s.human(t, http.MethodPost, "/boards",
		map[string]string{"name": "看板 " + slug, "slug": slug, "locale": "zh-CN"}))
}

// pickup 返回开启了 agent 领取的那一列。
func (b board) pickup(t *testing.T) column {
	t.Helper()
	for _, col := range b.Columns {
		if col.AgentPickup {
			return col
		}
	}
	t.Fatal("模板里没有开启 agent 领取的列")
	return column{}
}

// newCard 以人的身份在指定列创建一张卡片。
func (s *stack) newCard(t *testing.T, b board, columnID int64, title string) card {
	t.Helper()
	return decode[card](t, s.human(t, http.MethodPost, "/boards/"+id(b.ID)+"/cards",
		map[string]any{"title": title, "column_id": columnID}))
}

// id 把编号换成路径里用的字符串。
func id(n int64) string { return strconv.FormatInt(n, 10) }
