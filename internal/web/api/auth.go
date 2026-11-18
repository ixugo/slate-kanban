package api

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ixugo/goddd/pkg/web"

	"slate-kanban/internal/conf"
	"slate-kanban/internal/core/kanban"
)

const authPrefix = "/api/v1/auth"

// registerAuthRoutes 注册免口令检查的登录相关接口。
func registerAuthRoutes(r gin.IRouter, g *passcodeGuard) {
	r.GET(authPrefix+"/status", web.WrapH(func(c *gin.Context, _ *struct{}) (*authStatusResp, error) {
		return authStatus(g, c)
	}))
	r.POST(authPrefix+"/login", web.WrapH(func(c *gin.Context, in *loginReq) (*loginResp, error) {
		return authLogin(g, c, in)
	}))
	r.POST(authPrefix+"/logout", web.WrapH(func(c *gin.Context, _ *struct{}) (map[string]any, error) {
		clearSessionCookie(c)
		return map[string]any{}, nil
	}))
}

// authStatusResp 描述口令是否启用以及当前请求是否已认证。
type authStatusResp struct {
	Required      bool `json:"required"`
	Authenticated bool `json:"authenticated"`
}

// loginReq 是网页登录提交的单字段 JSON。
type loginReq struct {
	Passcode string `json:"passcode"`
}

// loginResp 登录成功时返回的极简状态。
type loginResp struct {
	Authenticated bool `json:"authenticated"`
}

// authStatus 按配置与当前请求的会话 cookie、Bearer 头计算 status 接口的返回值。
// 凭证检查走 authenticate，所以拿 Bearer 反复试口令同样会被计数和封锁，status 不是免限流的猜口令入口。
func authStatus(g *passcodeGuard, c *gin.Context) (*authStatusResp, error) {
	if !g.enabled() {
		return &authStatusResp{Required: false, Authenticated: true}, nil
	}
	outcome, retry := g.authenticate(c)
	if outcome == authBlocked {
		c.Header("Retry-After", strconv.Itoa(retry))
		return nil, errTooManyAttempts
	}
	return &authStatusResp{Required: true, Authenticated: outcome == authOK}, nil
}

// authLogin 校验登录口令并在成功时种会话 cookie；失败记限流并返回 401/429。
func authLogin(g *passcodeGuard, c *gin.Context, in *loginReq) (*loginResp, error) {
	if !g.enabled() {
		return &loginResp{Authenticated: true}, nil
	}
	pass := ""
	if in != nil {
		pass = strings.TrimSpace(in.Passcode)
	}
	if pass == "" || len(pass) > conf.PasscodeMaxLen {
		return nil, kanban.ErrInvalidInput.WithMsg("口令格式不合法")
	}
	ip := limiterKey(c)
	if blocked, retry := g.limiter.blocked(ip); blocked {
		c.Header("Retry-After", strconv.Itoa(retry))
		return nil, errTooManyAttempts
	}
	if !passcodesMatch(pass, g.passcode) {
		if blocked, retry := g.noteFailure(ip, c.Request.URL.Path); blocked {
			c.Header("Retry-After", strconv.Itoa(retry))
			return nil, errTooManyAttempts
		}
		return nil, errNeedPasscode
	}
	g.issueSessionCookie(c)
	return &loginResp{Authenticated: true}, nil
}
