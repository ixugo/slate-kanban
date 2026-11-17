package access_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"slate-kanban/internal/core/access"
	"slate-kanban/internal/core/access/accesstest"
)

var (
	admin = access.Principal{Admin: true, Name: access.AdminName}
	agent = access.Principal{Name: "claude"}
)

// TestCreateToken 验证创建：明文是 slate_ 加 40 位随机串，库里只有哈希和前缀，
// 返回的内容能序列化出明文但看不到哈希，并且拿明文立刻就能认证成 agent。
func TestCreateToken(t *testing.T) {
	ctx := context.Background()
	db := accesstest.NewDB(t)
	core := access.NewCore(db, adminToken)

	got, err := core.CreateToken(ctx, admin, "claude-code")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if !strings.HasPrefix(got.Plain, "slate_") || len(got.Plain) != 46 {
		t.Fatalf("明文的格式不对: %q", got.Plain)
	}
	if got.ID == 0 || got.Name != "claude-code" || got.Prefix != got.Plain[:12] {
		t.Fatalf("返回的记录不对: %+v", got.Token)
	}
	stored, err := db.Token().GetByHash(ctx, access.HashForTest(got.Plain))
	if err != nil || stored.ID != got.ID || stored.Hash == got.Plain {
		t.Fatalf("库里应该按哈希存，并且不是明文: %+v %v", stored, err)
	}

	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	if fields["token"] != got.Plain || fields["hash"] != nil || fields["name"] != "claude-code" {
		t.Fatalf("返回的 JSON 应该有明文、没有哈希: %s", raw)
	}

	who, err := core.Authenticate(ctx, "Bearer "+got.Plain)
	if err != nil || who.Admin || who.Name != "claude-code" {
		t.Fatalf("新令牌应该能认证成对应的 agent: %+v %v", who, err)
	}
	other, err := core.CreateToken(ctx, admin, "codex")
	if err != nil || other.Plain == got.Plain {
		t.Fatalf("两个令牌的明文必须不同: %v", err)
	}
}

// TestCreateTokenNames 验证名称规则：合法的通过；大写、符号开头、空格、冒号、中文、超长、保留名都返回 400。
func TestCreateTokenNames(t *testing.T) {
	core := accesstest.NewCore(t, adminToken)
	valid := []string{"a", "7", "claude-code", "codex_1", "gpt.5", strings.Repeat("a", 64)}
	for _, name := range valid {
		if _, err := core.CreateToken(context.Background(), admin, name); err != nil {
			t.Errorf("%q 应该合法: %v", name, err)
		}
	}
	invalid := []string{"", "Claude", "-a", ".a", "_a", "a b", "a:b", "automation:x", "名字", strings.Repeat("a", 65), "admin", "system", " claude"}
	for _, name := range invalid {
		if _, err := core.CreateToken(context.Background(), admin, name); !errors.Is(err, access.ErrInvalidName) {
			t.Errorf("%q 应该返回 ErrInvalidName: %v", name, err)
		}
	}
}

// TestCreateTokenRejects 验证拒绝的情况：agent 不能创建令牌；名称重复返回 409，并且不会多出一条记录。
func TestCreateTokenRejects(t *testing.T) {
	ctx := context.Background()
	core := accesstest.NewCore(t, adminToken)
	if _, err := core.CreateToken(ctx, agent, "codex"); !errors.Is(err, access.ErrAdminOnly) {
		t.Errorf("agent 创建令牌应该返回 403: %v", err)
	}
	if _, err := core.CreateToken(ctx, admin, "claude"); err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := core.CreateToken(ctx, admin, "claude"); !errors.Is(err, access.ErrTokenNameTaken) {
		t.Errorf("名称重复应该返回 409: %v", err)
	}
	items, err := core.ListTokens(ctx, admin)
	if err != nil || len(items) != 1 {
		t.Errorf("只应该有一个令牌: %+v %v", items, err)
	}
}

// TestListTokens 验证列表：只有管理令牌能看，按创建顺序，序列化后没有哈希；空列表是空数组。
func TestListTokens(t *testing.T) {
	ctx := context.Background()
	core := accesstest.NewCore(t, adminToken)
	empty, err := core.ListTokens(ctx, admin)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("空列表应该是空数组: %v %v", empty, err)
	}
	for _, name := range []string{"b", "a"} {
		if _, err := core.CreateToken(ctx, admin, name); err != nil {
			t.Fatalf("创建失败: %v", err)
		}
	}
	items, err := core.ListTokens(ctx, admin)
	if err != nil || len(items) != 2 || items[0].Name != "b" || items[1].Name != "a" {
		t.Fatalf("应该按创建顺序返回: %+v %v", items, err)
	}
	raw, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var fields []map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	for _, item := range fields {
		if item["hash"] != nil || item["token"] != nil || item["name"] == nil || item["prefix"] == nil {
			t.Fatalf("列表里不应该有哈希和明文，但要有名称和前缀: %v", item)
		}
	}
	if _, err := core.ListTokens(ctx, agent); !errors.Is(err, access.ErrAdminOnly) {
		t.Errorf("agent 列出令牌应该返回 403: %v", err)
	}
}

// TestDeleteToken 验证删除：agent 不能删；管理令牌删除后该令牌立刻认证失败，别的令牌不受影响；
// 重复删除和删除不存在的令牌都返回成功。
func TestDeleteToken(t *testing.T) {
	ctx := context.Background()
	core := accesstest.NewCore(t, adminToken)
	a, err := core.CreateToken(ctx, admin, "a")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	b, err := core.CreateToken(ctx, admin, "b")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}

	if err := core.DeleteToken(ctx, agent, a.ID); !errors.Is(err, access.ErrAdminOnly) {
		t.Fatalf("agent 删除令牌应该返回 403: %v", err)
	}
	if _, err := core.Authenticate(ctx, "Bearer "+a.Plain); err != nil {
		t.Fatalf("被拒绝的删除不应该让令牌失效: %v", err)
	}
	for range 2 {
		if err := core.DeleteToken(ctx, admin, a.ID); err != nil {
			t.Fatalf("删除应该幂等: %v", err)
		}
	}
	if err := core.DeleteToken(ctx, admin, 9999); err != nil {
		t.Fatalf("删除不存在的令牌应该返回成功: %v", err)
	}
	if _, err := core.Authenticate(ctx, "Bearer "+a.Plain); !errors.Is(err, access.ErrUnauthorized) {
		t.Fatalf("删除后应该返回 401: %v", err)
	}
	if _, err := core.Authenticate(ctx, "Bearer "+b.Plain); err != nil {
		t.Fatalf("别的令牌不应该受影响: %v", err)
	}
}
