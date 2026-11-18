package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ixugo/goddd/pkg/reason"
	"github.com/ixugo/goddd/pkg/web"

	"slate-kanban/internal/conf"
)

const (
	// sessionCookieName 是网页登录后使用的无状态会话 cookie 名。
	sessionCookieName = "slate_session"
	// sessionFormatVersion 是会话令牌格式版本，写在 cookie 值前缀。
	sessionFormatVersion = "v1"
	// sessionLifetime 是会话 cookie 的有效期；到期后须重新登录。
	sessionLifetime = 30 * 24 * time.Hour
)

var (
	// errNeedPasscode 未通过口令校验时返回给客户端的 401。
	errNeedPasscode = reason.ErrUnauthorized.WithMsg("需要访问口令")
	// errTooManyAttempts 同一 IP 短时失败过多时的 429。
	errTooManyAttempts = reason.NewError("ErrTooManyAttempts", "尝试次数过多，请稍后再试").WithHTTPStatus(http.StatusTooManyRequests)
)

// passcodeGuard 封装配置的口令与会话签发/校验逻辑，供中间件与登录接口共用。
type passcodeGuard struct {
	passcode string
	limiter  *attemptLimiter
}

// newPasscodeGuard 根据配置创建口令守卫；未配置口令时校验逻辑会直通。
func newPasscodeGuard(bc *conf.Bootstrap, limiter *attemptLimiter) *passcodeGuard {
	return &passcodeGuard{
		passcode: bc.Server.HTTP.Passcode,
		limiter:  limiter,
	}
}

// enabled 报告是否启用了访问口令。
func (g *passcodeGuard) enabled() bool {
	return g != nil && g.passcode != ""
}

// pathRequiresPasscode 判断当前路径是否要走口令检查（对照设计文档里的公开/受保护前缀表）。
func pathRequiresPasscode(path string) bool {
	if path == "/health" || path == "/install.md" {
		return false
	}
	if strings.HasPrefix(path, "/skills/") {
		return false
	}
	if strings.HasPrefix(path, "/api/v1/auth/") {
		return false
	}
	if path == "/mcp" || strings.HasPrefix(path, "/mcp/") {
		return true
	}
	if strings.HasPrefix(path, "/app/") {
		return true
	}
	if strings.HasPrefix(path, "/api/") {
		return true
	}
	return false
}

// passcodeMiddleware 在启用口令时拦截受保护路径：凭证有效才放行，
// 没带或不对返回 401，口令尝试过多返回 429（凭证检查和限流规则见 authenticate）。
func passcodeMiddleware(g *passcodeGuard) gin.HandlerFunc {
	if g == nil || !g.enabled() {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if !pathRequiresPasscode(path) {
			c.Next()
			return
		}
		switch outcome, retry := g.authenticate(c); outcome {
		case authOK:
			c.Next()
		case authBlocked:
			abortTooManyAttempts(c, retry)
		default:
			abortUnauthorized(c, path)
		}
	}
}

// authOutcome 是一次凭证检查的结果。
type authOutcome int

const (
	// authMissing 请求没带任何凭证。
	authMissing authOutcome = iota
	// authOK 凭证有效。
	authOK
	// authInvalid 带了凭证但不对。
	authInvalid
	// authBlocked 该 IP 因口令尝试过多被临时封锁。
	authBlocked
)

// authenticate 按固定顺序检查凭证并维护限流，中间件和 status 接口共用，
// 这样任何入口都不会变成不受限的猜口令入口：
//  1. 有效的会话 cookie 直接通过。cookie 靠 HMAC 签名，猜不出来，所以不受限流影响，
//     浏览器里留着过期 cookie 时也不会被误封。
//  2. 带 Bearer 才算在猜口令：先看该 IP 是否被封（封锁期内连正确口令也拒绝），
//     再比对口令，错了记一次失败。
//  3. 没带任何凭证、或只有一个无效 cookie，返回 authMissing 或 authInvalid，不计失败次数。
//
// 第二个返回值是被封锁时的 Retry-After 秒数，其余情况为 0。
func (g *passcodeGuard) authenticate(c *gin.Context) (authOutcome, int) {
	r := c.Request
	cookieVal, hasCookie := sessionCookie(r)
	if hasCookie && g.validSession(cookieVal) {
		return authOK, 0
	}
	bearer, hasBearer := bearerToken(r)
	if !hasBearer {
		if hasCookie {
			return authInvalid, 0
		}
		return authMissing, 0
	}
	ip := limiterKey(c)
	if blocked, retry := g.limiter.blocked(ip); blocked {
		return authBlocked, retry
	}
	if passcodesMatch(bearer, g.passcode) {
		return authOK, 0
	}
	if blocked, retry := g.noteFailure(ip, r.URL.Path); blocked {
		return authBlocked, retry
	}
	return authInvalid, 0
}

// limiterKey 返回限流用的来源标识：TCP 直连对端的 IP。
// 不用 c.ClientIP()：它会采信 X-Forwarded-For，而这个头客户端可以随意伪造，每次换一个就能绕过限流。
// 放在反向代理后面时，所有请求都来自代理的地址，会被合并计数。
func limiterKey(c *gin.Context) string {
	return c.RemoteIP()
}

// noteFailure 记一次口令错误并打一条 warn 日志（只有 IP 和路径，不含口令），
// 返回这次失败之后该 IP 是否已被封锁。每个 IP 每分钟最多记 11 条，日志量有上限。
func (g *passcodeGuard) noteFailure(ip, path string) (bool, int) {
	g.limiter.recordFailure(ip)
	blocked, retry := g.limiter.blocked(ip)
	slog.Warn("访问口令校验失败", "ip", ip, "path", path, "blocked", blocked)
	return blocked, retry
}

// abortTooManyAttempts 返回 429 并告诉客户端多久后再试。
func abortTooManyAttempts(c *gin.Context, retry int) {
	c.Header("Retry-After", strconv.Itoa(retry))
	web.AbortWithStatusJSON(c, errTooManyAttempts)
}

// abortUnauthorized 返回统一的 401 JSON；MCP 路径额外带 WWW-Authenticate。
func abortUnauthorized(c *gin.Context, path string) {
	if path == "/mcp" || strings.HasPrefix(path, "/mcp/") {
		c.Header("WWW-Authenticate", `Bearer realm="slate"`)
	}
	web.AbortWithStatusJSON(c, errNeedPasscode)
}

// bearerToken 从 Authorization 头解析 Bearer 令牌；认证方案名不区分大小写（RFC 7235）。
func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) < len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	t := strings.TrimSpace(h[len(prefix):])
	if t == "" {
		return "", false
	}
	return t, true
}

// sessionCookie 读取会话 cookie 的原始值。
func sessionCookie(r *http.Request) (string, bool) {
	c, err := r.Cookie(sessionCookieName)
	if err != nil || c == nil || c.Value == "" {
		return "", false
	}
	return c.Value, true
}

// passcodesMatch 用时序安全的摘要比较判断口令是否一致。
func passcodesMatch(got, want string) bool {
	if want == "" {
		return true
	}
	if got == "" {
		return false
	}
	gh := sha256.Sum256([]byte(got))
	wh := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(gh[:], wh[:]) == 1
}

// sessionSigningKey 由口令派生会话 HMAC 密钥；改口令会使旧 cookie 全部失效。
func sessionSigningKey(passcode string) []byte {
	sum := sha256.Sum256([]byte("slate-session:" + passcode))
	return sum[:]
}

// issueSessionCookie 在登录成功时签发无状态会话 cookie。
func (g *passcodeGuard) issueSessionCookie(c *gin.Context) {
	exp := time.Now().Add(sessionLifetime).Unix()
	payload := fmt.Sprintf("%s.%d", sessionFormatVersion, exp)
	mac := hmac.New(sha256.New, sessionSigningKey(g.passcode))
	_, _ = mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	val := payload + "." + sig
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookieName,
		Value:    val,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   requestIsSecure(c.Request),
		Expires:  time.Unix(exp, 0),
		MaxAge:   int(sessionLifetime / time.Second),
	})
}

// clearSessionCookie 在登出时删除会话 cookie。
func clearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   requestIsSecure(c.Request),
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

// validSession 校验 cookie 里的无状态会话：格式、未过期、HMAC 一致。
func (g *passcodeGuard) validSession(val string) bool {
	if !g.enabled() {
		return false
	}
	exp, err := parseSessionValue(val, g.passcode)
	if err != nil {
		return false
	}
	return time.Now().Unix() <= exp
}

// parseSessionValue 解析并校验会话令牌，返回过期时间戳。
func parseSessionValue(val, passcode string) (int64, error) {
	parts := strings.Split(val, ".")
	if len(parts) != 3 || parts[0] != sessionFormatVersion {
		return 0, errors.New("invalid session format")
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, err
	}
	payload := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, sessionSigningKey(passcode))
	_, _ = mac.Write([]byte(payload))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[2]), []byte(expected)) {
		return 0, errors.New("bad session signature")
	}
	return exp, nil
}

// requestIsSecure 判断请求是否应使用 Secure cookie（直连 TLS 或反向代理声明 https）。
func requestIsSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}
