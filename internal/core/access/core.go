// Package access 管理访问令牌和调用方身份：人用管理令牌，每个 agent 用自己的令牌。
// 令牌决定 agent 在看板里叫什么名字，所以不同 agent 不会互相冒名，也能各自单独记读取进度。
package access

import (
	"context"
	"time"
)

// AdminName 管理令牌对应的操作者名称，和看板领域里"人"的名称一致。
const AdminName = "admin"

// Principal 是认证通过的调用方。Admin 为真表示管理令牌，拥有人的全部权限；
// 否则是某个 agent，Name 是它的令牌绑定的名称。
type Principal struct {
	Admin bool
	Name  string
}

// Storer 是访问令牌领域的数据持久化聚合接口。
type Storer interface {
	// Token 返回令牌存储。
	Token() TokenStorer
}

// TokenStorer 是令牌的数据持久化接口。
type TokenStorer interface {
	// Create 新增一个令牌。名称或哈希重复时返回可用 orm.IsDuplicatedKey 判断的错误。
	Create(context.Context, *Token) error
	// GetByHash 按令牌哈希读取，不存在时返回可用 orm.IsErrRecordNotFound 判断的错误。
	GetByHash(ctx context.Context, hash string) (*Token, error)
	// List 返回全部令牌，按创建顺序。
	List(context.Context) ([]*Token, error)
	// Delete 按主键删除，已经不存在也不报错。
	Delete(ctx context.Context, id int64) error
	// Touch 把令牌的最后使用时间设为 at。
	Touch(ctx context.Context, id int64, at time.Time) error
}

// Option 调整 Core 的行为，主要给测试用。
type Option func(*Core)

// Core 是访问令牌领域的业务入口。
type Core struct {
	store      Storer
	adminToken string
	now        func() time.Time
}

// NewCore 创建 Core。adminToken 是配置里的管理令牌，空字符串表示没有配置，
// 这时任何请求都不会被当成管理员。
func NewCore(store Storer, adminToken string, opts ...Option) Core {
	c := Core{store: store, adminToken: adminToken, now: time.Now}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}
