package access_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"slate-kanban/internal/core/access"
	"slate-kanban/internal/core/access/accesstest"
)

const adminToken = "admin-secret-token"

// addAgent 直接往库里放一个 agent 令牌，返回它的明文。创建令牌的业务方法有自己的测试，
// 这里只需要"库里有这个令牌"这个前提。
func addAgent(t *testing.T, db interface {
	Token() access.TokenStorer
}, name, plain string,
) {
	t.Helper()
	rec := &access.Token{Name: name, Hash: access.HashForTest(plain), Prefix: "slate_abc", CreatedAt: time.Now().UTC()}
	if err := db.Token().Create(context.Background(), rec); err != nil {
		t.Fatalf("放入令牌失败: %v", err)
	}
}

// TestAuthenticateAdmin 验证管理令牌：匹配就是管理员，名称固定为 admin；不匹配返回 401。
func TestAuthenticateAdmin(t *testing.T) {
	core := accesstest.NewCore(t, adminToken)
	got, err := core.Authenticate(context.Background(), "Bearer "+adminToken)
	if err != nil || !got.Admin || got.Name != access.AdminName {
		t.Fatalf("管理令牌应该认证成功: %+v %v", got, err)
	}
	for _, header := range []string{"Bearer wrong", "Bearer " + adminToken + "x", "Bearer", "", adminToken, "Basic " + adminToken} {
		if _, err := core.Authenticate(context.Background(), header); !errors.Is(err, access.ErrUnauthorized) {
			t.Errorf("%q 应该返回 401: %v", header, err)
		}
	}
}

// TestAuthenticateNoAdminConfigured 验证没有配置管理令牌时，没有任何请求能成为管理员，
// 空令牌也不行。
func TestAuthenticateNoAdminConfigured(t *testing.T) {
	core := accesstest.NewCore(t, "")
	for _, header := range []string{"Bearer ", "Bearer", "", "Bearer anything"} {
		if got, err := core.Authenticate(context.Background(), header); !errors.Is(err, access.ErrUnauthorized) || got.Admin {
			t.Errorf("%q 不应该认证成功: %+v %v", header, got, err)
		}
	}
}

// TestAuthenticateAgent 验证 agent 令牌：明文认证成功，身份是令牌绑定的名称且不是管理员；
// 不存在的令牌返回 401；把库里存的哈希当令牌拿来用也不行。
func TestAuthenticateAgent(t *testing.T) {
	db := accesstest.NewDB(t)
	core := access.NewCore(db, adminToken)
	addAgent(t, db, "claude", "slate_plain-token")

	got, err := core.Authenticate(context.Background(), "Bearer slate_plain-token")
	if err != nil || got.Admin || got.Name != "claude" {
		t.Fatalf("agent 令牌应该认证成功: %+v %v", got, err)
	}
	for _, header := range []string{"Bearer slate_other", "Bearer " + access.HashForTest("slate_plain-token")} {
		if _, err := core.Authenticate(context.Background(), header); !errors.Is(err, access.ErrUnauthorized) {
			t.Errorf("%q 应该返回 401: %v", header, err)
		}
	}
}

// TestAuthenticateTouchesLastUsed 验证最后使用时间：第一次认证就记录；一分钟之内不再写；
// 超过一分钟再记录一次。
func TestAuthenticateTouchesLastUsed(t *testing.T) {
	ctx := context.Background()
	db := accesstest.NewDB(t)
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	core := access.NewCore(db, adminToken, access.WithClockForTest(func() time.Time { return now }))
	addAgent(t, db, "claude", "slate_plain-token")
	lastUsed := func() time.Time {
		rec, err := db.Token().GetByHash(ctx, access.HashForTest("slate_plain-token"))
		if err != nil || rec.LastUsedAt == nil {
			t.Fatalf("应该已经记录了最后使用时间: %+v %v", rec, err)
		}
		return *rec.LastUsedAt
	}
	authenticate := func() {
		t.Helper()
		if _, err := core.Authenticate(ctx, "Bearer slate_plain-token"); err != nil {
			t.Fatalf("认证失败: %v", err)
		}
	}

	authenticate()
	first := now
	if got := lastUsed(); !got.Equal(first) {
		t.Fatalf("第一次应该记录 %v，实际 %v", first, got)
	}
	now = first.Add(30 * time.Second)
	authenticate()
	if got := lastUsed(); !got.Equal(first) {
		t.Fatalf("一分钟之内不应该再写，实际变成了 %v", got)
	}
	now = first.Add(61 * time.Second)
	authenticate()
	if got := lastUsed(); !got.Equal(now) {
		t.Fatalf("超过一分钟应该更新为 %v，实际 %v", now, got)
	}
}

// failingStore 是一个可以让指定操作失败的假存储，用来验证数据库出错时的行为。
type failingStore struct {
	rec      *access.Token
	getErr   error
	touchErr error
}

func (s failingStore) Token() access.TokenStorer { return s }
func (s failingStore) Create(context.Context, *access.Token) error {
	return errors.New("不支持")
}

func (s failingStore) GetByHash(context.Context, string) (*access.Token, error) {
	return s.rec, s.getErr
}

func (s failingStore) List(context.Context) ([]*access.Token, error) {
	return nil, errors.New("不支持")
}
func (s failingStore) Delete(context.Context, int64) error { return errors.New("不支持") }
func (s failingStore) Touch(context.Context, int64, time.Time) error {
	return s.touchErr
}

// TestAuthenticateDatabaseErrors 验证数据库出错：查询失败返回数据库错误而不是 401，
// 不能让人误以为令牌有问题；只是记录最后使用时间失败时，认证仍然成功。
func TestAuthenticateDatabaseErrors(t *testing.T) {
	ctx := context.Background()
	down := access.NewCore(failingStore{getErr: errors.New("连接断了")}, adminToken)
	if _, err := down.Authenticate(ctx, "Bearer slate_x"); err == nil || errors.Is(err, access.ErrUnauthorized) {
		t.Fatalf("查询失败应该返回数据库错误: %v", err)
	}

	rec := &access.Token{ID: 1, Name: "claude"}
	flaky := access.NewCore(failingStore{rec: rec, touchErr: errors.New("写不进去")}, adminToken)
	got, err := flaky.Authenticate(ctx, "Bearer slate_x")
	if err != nil || got.Name != "claude" {
		t.Fatalf("记录时间失败不应该影响认证: %+v %v", got, err)
	}
}
