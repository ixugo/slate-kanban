package access

import (
	"context"
	"regexp"

	"github.com/ixugo/goddd/pkg/orm"
)

const (
	// tokenPrefix 令牌的固定开头，便于在日志和泄露扫描里认出它。
	tokenPrefix = "slate_"
	// tokenRandomLen 令牌里随机部分的长度，62 个字符的字母表，约 238 位熵。
	tokenRandomLen = 40
	// prefixShown 保存下来给界面展示的令牌前缀长度，不足以用来猜出令牌。
	prefixShown = 12
)

// namePattern 是令牌名称的规则：小写字母、数字、点、下划线、连字符，首字符是字母或数字，1 到 64 个字符。
// 全是 ASCII，所以字节数就是字符数；只允许小写，名称的大小写就不会造成重复或混淆；不含冒号，
// 不会和自动化规则的操作者名称（automation: 加规则名）混淆。
var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// reservedNames 是保留的名称：admin 是人的操作者名称，system 是系统操作者的类型名。
var reservedNames = map[string]bool{AdminName: true, "system": true}

// NewToken 是创建令牌的结果，Plain 是令牌明文，只在创建的这一次返回，之后无法找回。
type NewToken struct {
	*Token
	Plain string `json:"token"`
}

// checkName 校验令牌名称。
func checkName(name string) error {
	if !namePattern.MatchString(name) {
		return ErrInvalidName.WithMsg("名称只能用小写字母、数字、点、下划线和连字符，以字母或数字开头，最长 64 个字符")
	}
	if reservedNames[name] {
		return ErrInvalidName.WithMsg("这个名称是保留的，请换一个")
	}
	return nil
}

// CreateToken 为一个 agent 创建令牌。只有管理令牌能操作；名称不合法返回 400，已经被用了返回 409。
func (c Core) CreateToken(ctx context.Context, p Principal, name string) (*NewToken, error) {
	if !p.Admin {
		return nil, ErrAdminOnly
	}
	if err := checkName(name); err != nil {
		return nil, err
	}
	plain := tokenPrefix + orm.GenerateRandomString(tokenRandomLen)
	rec := &Token{Name: name, Hash: hashToken(plain), Prefix: plain[:prefixShown], CreatedAt: c.now()}
	if err := c.store.Token().Create(ctx, rec); err != nil {
		if orm.IsDuplicatedKey(err) {
			return nil, ErrTokenNameTaken
		}
		return nil, wrapDB("创建令牌", err)
	}
	return &NewToken{Token: rec, Plain: plain}, nil
}

// ListTokens 返回全部 agent 令牌，按创建顺序，不含哈希。只有管理令牌能操作。
func (c Core) ListTokens(ctx context.Context, p Principal) ([]*Token, error) {
	if !p.Admin {
		return nil, ErrAdminOnly
	}
	items, err := c.store.Token().List(ctx)
	if err != nil {
		return nil, wrapDB("读取令牌列表", err)
	}
	return items, nil
}

// DeleteToken 删除一个令牌，持有它的 agent 下一次请求就会得到 401。只有管理令牌能操作。
// 删除是幂等的：令牌已经不存在也返回成功。
func (c Core) DeleteToken(ctx context.Context, p Principal, id int64) error {
	if !p.Admin {
		return ErrAdminOnly
	}
	if err := c.store.Token().Delete(ctx, id); err != nil {
		return wrapDB("删除令牌", err)
	}
	return nil
}
