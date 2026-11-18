package api

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ixugo/goddd/pkg/reason"
	"github.com/ixugo/goddd/pkg/web"

	"slate-kanban/internal/conf"
)

// errForbiddenHost 请求的 Host 不在白名单里，用来挡 DNS 重绑定。
var errForbiddenHost = reason.NewError("ErrForbiddenHost", "禁止访问：Host 不在允许列表中").WithHTTPStatus(http.StatusForbidden)

// errForbiddenOrigin 跨站写请求：Origin 的主机和 Host 不一致。
var errForbiddenOrigin = reason.NewError("ErrForbiddenOrigin", "禁止访问：跨站写请求被拒绝").WithHTTPStatus(http.StatusForbidden)

// securityMiddleware 返回全局安全中间件：安全响应头、Host 白名单、跨站写拦截。
// allowed 来自配置 Server.HTTP.AllowedHosts，对所有路径生效，防止 DNS 重绑定。
func securityMiddleware(bc *conf.Bootstrap) gin.HandlerFunc {
	allowed := append([]string(nil), bc.Server.HTTP.AllowedHosts...)
	return func(c *gin.Context) {
		securityHeaders(c)
		if err := checkHost(c.Request.Host, allowed); err != nil {
			web.AbortWithStatusJSON(c, err)
			return
		}
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

// checkHost 判断 Host（可带端口）是否允许：IP 字面量、localhost、*.localhost 或配置白名单。
func checkHost(host string, allowed []string) error {
	h := hostOnly(host)
	if h == "" {
		return errForbiddenHost
	}
	if hostAllowed(h, allowed) {
		return nil
	}
	return errForbiddenHost
}

// hostOnly 去掉 Host 里的端口，IPv6 地址保留方括号外的主机部分。
func hostOnly(host string) string {
	if host == "" {
		return ""
	}
	if strings.HasPrefix(host, "[") {
		if h, _, err := net.SplitHostPort(host); err == nil {
			return strings.Trim(h, "[]")
		}
		return strings.Trim(host, "[]")
	}
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		return host
	}
	return h
}

// hostAllowed 判断是否放行：本机常用名、localhost 子域或配置项（大小写不敏感、去端口比较）。
func hostAllowed(host string, allowed []string) bool {
	lower := strings.ToLower(host)
	if ip := net.ParseIP(host); ip != nil {
		return true
	}
	if lower == "localhost" {
		return true
	}
	if strings.HasSuffix(lower, ".localhost") {
		return true
	}
	for _, a := range allowed {
		if strings.EqualFold(hostOnly(a), host) {
			return true
		}
	}
	return false
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

// canonicalHostPort 规范化主机比较串：无端口时按 http 默认 80、https 默认 443 补上，避免误判。
func canonicalHostPort(host string) string {
	if host == "" {
		return ""
	}
	h, port, err := net.SplitHostPort(host)
	if err != nil {
		return strings.ToLower(host)
	}
	if port == "" {
		return strings.ToLower(h)
	}
	return strings.ToLower(net.JoinHostPort(h, port))
}
