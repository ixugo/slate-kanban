package mcpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/auth"

	"slate-kanban/internal/core/kanban"
)

// TestParseQueryIdentity 验证 name 和 role 的解析、缺省名和保留名。
func TestParseQueryIdentity(t *testing.T) {
	actor, role, err := parseQueryIdentity(map[string][]string{"name": {"codex"}})
	if err != nil || role != "worker" || actor.Name != "codex" || actor.Kind != kanban.ActorAgent {
		t.Fatalf("worker+codex: %+v %q %v", actor, role, err)
	}
	actor, role, err = parseQueryIdentity(map[string][]string{"role": {"manager"}})
	if err != nil || role != "manager" || actor.Name != "manager" || actor.Kind != kanban.ActorHuman {
		t.Fatalf("manager 缺省名: %+v %q %v", actor, role, err)
	}
	if _, _, err := parseQueryIdentity(map[string][]string{"name": {"human"}}); err == nil {
		t.Fatal("保留名 human 应拒绝")
	}
	if _, _, err := parseQueryIdentity(map[string][]string{"name": {"Bad Name"}}); err == nil {
		t.Fatal("非法 name 应拒绝")
	}
	if _, _, err := parseQueryIdentity(map[string][]string{"role": {"admin"}}); err == nil {
		t.Fatal("非法 role 应拒绝")
	}
}

// TestIdentityMiddlewareHTTP 验证非法参数返回 400 JSON。
func TestIdentityMiddlewareHTTP(t *testing.T) {
	h := identityMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp?name=Bad%20Name", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "name") {
		t.Fatalf("非法 name 应 400: %d %s", rec.Code, rec.Body)
	}
}

// TestHeaderVerifier 验证 SDK 会话绑定从请求头取身份。
func TestHeaderVerifier(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/mcp?name=codex", nil)
	req.Header.Set(headerActorKind, string(kanban.ActorAgent))
	req.Header.Set(headerActorName, "codex")
	info, err := headerVerifier(req.Context(), "open", req)
	if err != nil {
		t.Fatalf("headerVerifier: %v", err)
	}
	actor, err := actorOf(info)
	if err != nil || actor.Name != "codex" || actor.Kind != kanban.ActorAgent {
		t.Fatalf("期望 agent codex，实际 %+v %v", actor, err)
	}
}

// TestActorOfMissing 验证没有身份的 TokenInfo 会被拒绝。
func TestActorOfMissing(t *testing.T) {
	for name, info := range map[string]*auth.TokenInfo{
		"没有令牌信息":  nil,
		"没有操作者":   {UserID: "agent:x"},
		"操作者类型不对": {UserID: "agent:x", Extra: map[string]any{actorKey: "claude"}},
	} {
		if _, err := actorOf(info); !errors.Is(err, kanban.ErrInvalidActor) {
			t.Errorf("%s：应该返回无效操作者: %v", name, err)
		}
	}
}
