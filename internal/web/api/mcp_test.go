package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const mcpPatience = 5 * time.Second

type mcpNameTransport struct {
	name  string
	admin bool
}

func (m mcpNameTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	u := *r.URL
	q := u.Query()
	if m.admin {
		q.Set("role", "manager")
		if m.name != "" {
			q.Set("name", m.name)
		}
	} else {
		q.Set("name", m.name)
	}
	u.RawQuery = q.Encode()
	r.URL = &u
	return http.DefaultTransport.RoundTrip(r)
}

func startServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(shared.engine)
	t.Cleanup(func() {
		srv.CloseClientConnections()
		srv.Close()
	})
	return srv
}

func mcpConnect(t *testing.T, srv *httptest.Server, name string, admin bool, opts *mcp.ClientOptions) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "router-test", Version: "0"}, opts)
	sess, err := client.Connect(t.Context(), &mcp.StreamableClientTransport{
		Endpoint:   srv.URL + mcpPath,
		HTTPClient: &http.Client{Transport: mcpNameTransport{name: name, admin: admin}},
	}, nil)
	if err != nil {
		t.Fatalf("连接 /mcp 失败: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess
}

func createRESTBoard(t *testing.T, name, slug string) (boardID, firstColumnID int64) {
	t.Helper()
	resp := call(t, http.MethodPost, "/api/v1/boards", "", map[string]string{"name": name, "slug": slug})
	var out struct {
		ID      int64 `json:"id"`
		Columns []struct {
			ID int64 `json:"id"`
		} `json:"columns"`
	}
	resp.JSON(t, &out)
	if resp.Code != http.StatusOK || out.ID == 0 || len(out.Columns) == 0 {
		t.Fatalf("创建看板失败: %d %s", resp.Code, resp.Body)
	}
	return out.ID, out.Columns[0].ID
}

func TestMCPIdentityBadName(t *testing.T) {
	srv := startServer(t)
	u, _ := url.Parse(srv.URL + mcpPath)
	u.RawQuery = "name=Bad%20Name"
	req, _ := http.NewRequest(http.MethodPost, u.String(), strings.NewReader(`{}`))
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("非法 name 应 400: %d", resp.StatusCode)
	}
}

func TestMCPInitializeWorker(t *testing.T) {
	srv := startServer(t)
	sess := mcpConnect(t, srv, "codex", false, nil)
	if got := sess.InitializeResult().ServerInfo.Version; got != testBuildVersion {
		t.Errorf("版本 %q", got)
	}
	who, err := sess.CallTool(t.Context(), &mcp.CallToolParams{Name: "whoami"})
	if err != nil || who.IsError || !strings.Contains(resultText(who), `"codex"`) {
		t.Fatalf("whoami: %v %s", err, resultText(who))
	}
	names := map[string]bool{}
	for tool, err := range sess.Tools(t.Context(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		names[tool.Name] = true
	}
	if names["create_board"] {
		t.Error("工作视图不应有 create_board")
	}
}

func TestMCPManagerTools(t *testing.T) {
	srv := startServer(t)
	sess := mcpConnect(t, srv, "mgr", true, nil)
	who, err := sess.CallTool(t.Context(), &mcp.CallToolParams{Name: "whoami"})
	if err != nil || who.IsError || !strings.Contains(resultText(who), `"human"`) {
		t.Fatalf("管理视图 whoami 应是人: %s", resultText(who))
	}
	names := map[string]bool{}
	for tool, err := range sess.Tools(t.Context(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		names[tool.Name] = true
	}
	if !names["create_board"] {
		t.Error("管理视图应有 create_board")
	}
}

func TestMCPNotifiedByREST(t *testing.T) {
	srv := startServer(t)
	boardID, columnID := createRESTBoard(t, "MCP 通知看板", "RTMCPNOTE")
	notices := make(chan string, 16)
	sess := mcpConnect(t, srv, "note-bot", false, &mcp.ClientOptions{
		ResourceUpdatedHandler: func(_ context.Context, r *mcp.ResourceUpdatedNotificationRequest) {
			notices <- r.Params.URI
		},
	})
	time.Sleep(time.Second)
	uri := "slate://boards/" + strconv.FormatInt(boardID, 10)
	if err := sess.Subscribe(t.Context(), &mcp.SubscribeParams{URI: uri}); err != nil {
		t.Fatalf("订阅看板失败: %v", err)
	}
	resp := call(t, http.MethodPost, "/api/v1/boards/"+strconv.FormatInt(boardID, 10)+"/cards", "", map[string]any{
		"title": "REST 建的卡片", "column_id": columnID,
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("REST 建卡片失败: %d %s", resp.Code, resp.Body)
	}
	select {
	case got := <-notices:
		if got != uri {
			t.Errorf("通知的地址 = %q，期望 %q", got, uri)
		}
	case <-time.After(mcpPatience):
		t.Fatal("REST 改了看板，订阅的 agent 没有收到通知")
	}
}

func resultText(res *mcp.CallToolResult) string {
	var sb strings.Builder
	for _, c := range res.Content {
		if text, ok := c.(*mcp.TextContent); ok {
			sb.WriteString(text.Text)
		}
	}
	return sb.String()
}
