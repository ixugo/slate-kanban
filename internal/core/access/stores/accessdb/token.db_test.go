package accessdb_test

import (
	"context"
	"testing"
	"time"

	"github.com/ixugo/goddd/pkg/orm"

	"slate-kanban/internal/core/access"
	"slate-kanban/internal/core/access/accesstest"
)

// newToken 构造一个测试用的令牌记录。
func newToken(name, hash string) *access.Token {
	return &access.Token{Name: name, Hash: hash, Prefix: "slate_abcdef", CreatedAt: time.Now().UTC()}
}

// TestTokenCreateAndGet 验证新增后主键被填上，能按哈希读回；哈希不存在返回"找不到记录"。
func TestTokenCreateAndGet(t *testing.T) {
	ctx := context.Background()
	store := accesstest.NewDB(t).Token()
	rec := newToken("claude", "h1")
	if err := store.Create(ctx, rec); err != nil || rec.ID == 0 {
		t.Fatalf("新增失败或没有主键: %+v %v", rec, err)
	}
	got, err := store.GetByHash(ctx, "h1")
	if err != nil || got.ID != rec.ID || got.Name != "claude" || got.LastUsedAt != nil {
		t.Fatalf("读回的内容不对: %+v %v", got, err)
	}
	if _, err := store.GetByHash(ctx, "missing"); !orm.IsErrRecordNotFound(err) {
		t.Fatalf("不存在的哈希应该返回找不到记录: %v", err)
	}
}

// TestTokenUniqueness 验证名称和哈希都不能重复，重复时能用 orm.IsDuplicatedKey 判断。
func TestTokenUniqueness(t *testing.T) {
	ctx := context.Background()
	store := accesstest.NewDB(t).Token()
	if err := store.Create(ctx, newToken("claude", "h1")); err != nil {
		t.Fatalf("新增失败: %v", err)
	}
	if err := store.Create(ctx, newToken("claude", "h2")); !orm.IsDuplicatedKey(err) {
		t.Errorf("名称重复应该被识别: %v", err)
	}
	if err := store.Create(ctx, newToken("codex", "h1")); !orm.IsDuplicatedKey(err) {
		t.Errorf("哈希重复应该被识别: %v", err)
	}
}

// TestTokenListAndDelete 验证列表按创建顺序，空列表是空数组而不是 null；删除是幂等的。
func TestTokenListAndDelete(t *testing.T) {
	ctx := context.Background()
	store := accesstest.NewDB(t).Token()
	empty, err := store.List(ctx)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("空列表应该是空数组: %v %v", empty, err)
	}
	a, b := newToken("a", "ha"), newToken("b", "hb")
	for _, rec := range []*access.Token{a, b} {
		if err := store.Create(ctx, rec); err != nil {
			t.Fatalf("新增失败: %v", err)
		}
	}
	items, err := store.List(ctx)
	if err != nil || len(items) != 2 || items[0].ID != a.ID || items[1].ID != b.ID {
		t.Fatalf("应该按创建顺序返回: %+v %v", items, err)
	}
	for range 2 {
		if err := store.Delete(ctx, a.ID); err != nil {
			t.Fatalf("删除应该幂等: %v", err)
		}
	}
	if _, err := store.GetByHash(ctx, "ha"); !orm.IsErrRecordNotFound(err) {
		t.Fatalf("删除之后应该找不到: %v", err)
	}
	if left, _ := store.List(ctx); len(left) != 1 || left[0].ID != b.ID {
		t.Fatalf("另一个令牌不应该受影响: %+v", left)
	}
}

// TestTokenTouch 验证只更新最后使用时间，其他字段原样。
func TestTokenTouch(t *testing.T) {
	ctx := context.Background()
	store := accesstest.NewDB(t).Token()
	rec := newToken("claude", "h1")
	if err := store.Create(ctx, rec); err != nil {
		t.Fatalf("新增失败: %v", err)
	}
	at := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	if err := store.Touch(ctx, rec.ID, at); err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	got, err := store.GetByHash(ctx, "h1")
	if err != nil || got.LastUsedAt == nil || !got.LastUsedAt.Equal(at) {
		t.Fatalf("最后使用时间应该是 %v: %+v %v", at, got, err)
	}
	if got.Name != "claude" || got.Prefix != rec.Prefix {
		t.Fatalf("其他字段不应该变: %+v", got)
	}
}
