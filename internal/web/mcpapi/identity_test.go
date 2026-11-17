package mcpapi

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/auth"

	"slate-kanban/internal/core/access"
	"slate-kanban/internal/core/access/accesstest"
	"slate-kanban/internal/core/kanban"
)

const testAdminToken = "admin-secret"

// admin 是调用令牌管理方法时用的管理员身份。
var admin = access.Principal{Admin: true, Name: access.AdminName}

// TestVerifier 验证令牌认证：管理令牌换成人，agent 令牌换成 agent，
// 会话绑定用的 UserID 带上类型，令牌没有过期时间，无效和已删除的令牌都按无效令牌处理。
func TestVerifier(t *testing.T) {
	core := accesstest.NewCore(t, testAdminToken)
	made, err := core.CreateToken(t.Context(), admin, "claude")
	if err != nil {
		t.Fatalf("创建令牌失败: %v", err)
	}
	verify := newVerifier(core)

	cases := []struct {
		name   string
		token  string
		userID string
		actor  kanban.Actor
	}{
		{"管理令牌", testAdminToken, "human:admin", kanban.Actor{Kind: kanban.ActorHuman, Name: "admin"}},
		{"agent 令牌", made.Plain, "agent:claude", kanban.Actor{Kind: kanban.ActorAgent, Name: "claude"}},
	}
	for _, tc := range cases {
		info, err := verify(t.Context(), tc.token, nil)
		if err != nil {
			t.Fatalf("%s：认证失败: %v", tc.name, err)
		}
		got, err := actorOf(info)
		if err != nil || got != tc.actor || info.UserID != tc.userID {
			t.Errorf("%s：期望 %q %+v，实际 %q %+v (%v)", tc.name, tc.userID, tc.actor, info.UserID, got, err)
		}
		if !info.Expiration.IsZero() {
			t.Errorf("%s：令牌没有过期时间，不应该设置 Expiration: %v", tc.name, info.Expiration)
		}
	}

	if _, err := verify(t.Context(), "slate_not-a-real-token", nil); !errors.Is(err, auth.ErrInvalidToken) {
		t.Errorf("不存在的令牌应该是无效令牌: %v", err)
	}
	if err := core.DeleteToken(t.Context(), admin, made.ID); err != nil {
		t.Fatalf("删除令牌失败: %v", err)
	}
	if _, err := verify(t.Context(), made.Plain, nil); !errors.Is(err, auth.ErrInvalidToken) {
		t.Errorf("已删除的令牌应该立刻失效: %v", err)
	}
}

// brokenTokens 是一个查询必定失败的令牌存储，用来模拟数据库出错。
type brokenTokens struct{ access.TokenStorer }

// GetByHash 总是返回带内部细节的错误。
func (brokenTokens) GetByHash(context.Context, string) (*access.Token, error) {
	return nil, errors.New("dial tcp 10.0.0.1:5432: secret-host refused")
}

// Token 让存储把自己当作令牌存储返回。
func (b brokenTokens) Token() access.TokenStorer { return b }

// TestVerifierStoreFailure 验证查库出错时：不当成无效令牌（否则用户会以为令牌错了），
// 不把内部细节返回给没有通过认证的调用方，但完整的错误要记进日志。
func TestVerifierStoreFailure(t *testing.T) {
	buf := captureLog(t)
	verify := newVerifier(access.NewCore(brokenStoreOf(), testAdminToken))
	_, err := verify(t.Context(), "slate_whatever", nil)
	if err == nil || errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("查库出错不应该是无效令牌: %v", err)
	}
	if strings.Contains(err.Error(), "secret-host") {
		t.Errorf("返回给调用方的错误不应该包含内部细节: %v", err)
	}
	if !strings.Contains(buf.String(), "secret-host") {
		t.Errorf("日志里应该有完整的错误: %s", buf)
	}
}

// brokenStoreOf 返回一个令牌查询必定失败的存储。
func brokenStoreOf() access.Storer { return brokenTokens{} }

// TestActorOfMissing 验证没有身份的请求被拒绝：没有令牌信息、或令牌信息里没有操作者，都按未认证处理。
func TestActorOfMissing(t *testing.T) {
	for name, info := range map[string]*auth.TokenInfo{
		"没有令牌信息":  nil,
		"没有操作者":   {UserID: "agent:x"},
		"操作者类型不对": {UserID: "agent:x", Extra: map[string]any{actorKey: "claude"}},
	} {
		if _, err := actorOf(info); !errors.Is(err, access.ErrUnauthorized) {
			t.Errorf("%s：应该返回未认证: %v", name, err)
		}
	}
}
