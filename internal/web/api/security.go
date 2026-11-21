package api

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ixugo/goddd/pkg/reason"
	"github.com/ixugo/goddd/pkg/web"
)

// errForbiddenOrigin 跨站写请求：Origin 的主机和 Host 不一致。
var errForbiddenOrigin = reason.NewError("ErrForbiddenOrigin", "禁止访问：跨站写请求被拒绝").WithHTTPStatus(http.StatusForbidden)

// securityMiddleware 返回全局安全中间件：安全响应头、跨站写拦截。
// 不限制 Host，用域名、IP 访问都放行；只有 pprof 另有自己的 IP 白名单。
func securityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		securityHeaders(c)
		if err := checkOrigin(c.Request); err != nil {
			web.AbortWithStatusJSON(c, err)
			return
		}
		c.Next()
	}
}

// securityHeaders 给所有响应加上防嵌套和防嗅探头。
func securityHeaders(c *gin.Context) {
	c.Header("X-Frame-Options", "DENY")
	c.Header("X-Content-Type-Options", "nosniff")
}

// checkOrigin 拦截跨站写请求：非安全方法且带了 Origin 时，Origin 的主机（含端口）必须和 Host 一致。
func checkOrigin(r *http.Request) error {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return nil
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return nil
	}
	ou, err := url.Parse(origin)
	if err != nil || ou.Host == "" {
		return errForbiddenOrigin
	}
	reqHost := canonicalHostPort(r.Host)
	originHost := canonicalHostPort(ou.Host)
	if !strings.EqualFold(reqHost, originHost) {
		return errForbiddenOrigin
	}
	return nil
}

// canonicalHostPort 规范化主机比较串：默认端口 80、443 当成「没写端口」，和 Host 头里省略默认端口时能对上。
func canonicalHostPort(host string) string {
	if host == "" {
		return ""
	}
	h, port, err := net.SplitHostPort(host)
	if err != nil {
		return strings.ToLower(host)
	}
	if port == "80" || port == "443" {
		return strings.ToLower(h)
	}
	return strings.ToLower(net.JoinHostPort(h, port))
}
