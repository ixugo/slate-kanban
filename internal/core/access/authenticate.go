package access

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"strings"
	"time"

	"github.com/ixugo/goddd/pkg/orm"
)

const (
	// touchInterval 记录令牌最后使用时间的最小间隔。认证每个请求都会发生，
	// 不限制的话每个请求都要多一次写库，SQLite 只有一个连接，会拖慢真正的业务请求。
	touchInterval = time.Minute
	// maxBearerLen 请求头里令牌的最大长度。合法的令牌只有几十个字符，
	// 超长的直接拒绝，免得对一大段垃圾数据做哈希。
	maxBearerLen = 256
)

// Authenticate 认证请求头 Authorization 的值，返回调用方身份。
// 先和管理令牌做恒定时间比较，再把令牌哈希后查库，所以库里没有任何明文可以比较。
// 令牌缺失、格式不对、不存在都返回 401，不区分原因，免得给试探的人提示。
func (c Core) Authenticate(ctx context.Context, authorization string) (Principal, error) {
	token, ok := parseBearer(authorization)
	if !ok {
		return Principal{}, ErrUnauthorized
	}
	if c.adminToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(c.adminToken)) == 1 {
		return Principal{Admin: true, Name: AdminName}, nil
	}
	rec, err := c.store.Token().GetByHash(ctx, hashToken(token))
	if orm.IsErrRecordNotFound(err) {
		return Principal{}, ErrUnauthorized
	}
	if err != nil {
		return Principal{}, wrapDB("查询令牌", err)
	}
	c.touch(ctx, rec)
	return Principal{Name: rec.Name}, nil
}

// touch 记录令牌的最后使用时间。距上次记录不足 touchInterval 就不写；
// 写失败只记日志，因为认证本身已经成功，不能因为记个时间让请求失败。
func (c Core) touch(ctx context.Context, rec *Token) {
	now := c.now()
	if rec.LastUsedAt != nil && now.Sub(*rec.LastUsedAt) < touchInterval {
		return
	}
	if err := c.store.Token().Touch(ctx, rec.ID, now); err != nil {
		slog.WarnContext(ctx, "记录令牌最后使用时间失败", "token_id", rec.ID, "err", err)
	}
}

// parseBearer 从请求头的值里取出令牌。只认 "Bearer 令牌"，Bearer 不区分大小写，令牌前后的空白忽略；
// 没有令牌、令牌里还夹着空白、超过长度上限都算格式不对。
func parseBearer(header string) (string, bool) {
	scheme, rest, found := strings.Cut(strings.TrimSpace(header), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token := strings.TrimSpace(rest)
	if token == "" || len(token) > maxBearerLen || strings.ContainsAny(token, " \t") {
		return "", false
	}
	return token, true
}

// hashToken 计算令牌的 SHA-256 十六进制串，库里存的就是它。
// 令牌是 40 位随机串，熵足够高，不需要加盐的慢哈希。
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
