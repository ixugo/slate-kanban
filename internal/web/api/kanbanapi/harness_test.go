package kanbanapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/kanbantest"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
	"slate-kanban/internal/lifecycle"
	"slate-kanban/internal/web/api/apitest"
	"slate-kanban/internal/web/api/kanbanapi"
	"slate-kanban/internal/web/restactor"
)

// stack 是一套接好内存数据库的完整接口，路由与正式环境一致（固定人身份）。
type stack struct {
	apitest.Client
	db       kanbandb.DB
	finished chan struct{}
	closing  context.Context
	closeNow context.CancelFunc
}

func newStack(t *testing.T) *stack {
	t.Helper()
	return newStackWith(t, func(a kanbanapi.API) kanbanapi.API { return a })
}

func newStackWith(t *testing.T, tune func(kanbanapi.API) kanbanapi.API) *stack {
	t.Helper()
	db := kanbantest.NewDB(t)
	s := &stack{db: db, finished: make(chan struct{}, 16)}
	s.closing, s.closeNow = context.WithCancel(t.Context())
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Next()
		select {
		case s.finished <- struct{}{}:
		default:
		}
	})
	v1 := engine.Group("/api/v1", restactor.Middleware())
	kanbanapi.Register(v1, tune(kanbanapi.New(kanban.NewCore(db, kanbantest.Resources(t)))))
	s.Client = apitest.Client{Engine: engine}
	return s
}

func (s *stack) serve(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(lifecycle.Wrap(s.Engine, s.closing))
	t.Cleanup(func() {
		srv.CloseClientConnections()
		srv.Close()
	})
	return srv
}

// human 以人的身份发请求（REST 固定 human，token 参数忽略）。
func (s *stack) human(t *testing.T, method, path string, body any) apitest.Response {
	t.Helper()
	return s.Do(t, method, "/api/v1"+path, "", body)
}

// agent REST 也当作人处理；保留签名以兼容旧测试写法。
func (s *stack) agent(t *testing.T, name, method, path string, body any) apitest.Response {
	t.Helper()
	_ = name
	return s.Do(t, method, "/api/v1"+path, "", body)
}

func (s *stack) token(t *testing.T, name string) string {
	t.Helper()
	return name
}

func decode[T any](t *testing.T, resp apitest.Response) T {
	t.Helper()
	if resp.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d: %s", resp.Code, resp.Body)
	}
	var out T
	resp.JSON(t, &out)
	return out
}

func wantFail(t *testing.T, name string, resp apitest.Response, status int, reason string) {
	t.Helper()
	if resp.Code != status || resp.Reason() != reason {
		t.Errorf("%s：期望 %d %s，实际 %d %s", name, status, reason, resp.Code, resp.Body)
	}
}

func wantEmptyObject(t *testing.T, name string, resp apitest.Response) {
	t.Helper()
	if resp.Code != http.StatusOK || strings.TrimSpace(string(resp.Body)) != "{}" {
		t.Errorf("%s：期望 200 和 {}，实际 %d %s", name, resp.Code, resp.Body)
	}
}

type (
	column struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		IsDone bool   `json:"is_done"`
	}
	board struct {
		ID          int64            `json:"id"`
		Name        string           `json:"name"`
		Slug        string           `json:"slug"`
		Description string           `json:"description"`
		ProjectPath string           `json:"project_path"`
		Tags        []map[string]any `json:"tags"`
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
		TagIDs      []int64  `json:"tag_ids"`
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

func (s *stack) newBoard(t *testing.T, slug string) board {
	t.Helper()
	return decode[board](t, s.human(t, http.MethodPost, "/boards",
		map[string]string{"name": "看板 " + slug, "slug": slug, "locale": "zh-CN"}))
}

func (b board) pickup(t *testing.T) column {
	t.Helper()
	for _, col := range b.Columns {
		if col.Name == "进行中" {
			return col
		}
	}
	t.Fatal("模板里没有开启 agent 领取的列")
	return column{}
}

func (s *stack) newCard(t *testing.T, b board, columnID int64, title string) card {
	t.Helper()
	return decode[card](t, s.human(t, http.MethodPost, "/boards/"+id(b.ID)+"/cards",
		map[string]any{"title": title, "column_id": columnID}))
}

func id(n int64) string { return strconv.FormatInt(n, 10) }
