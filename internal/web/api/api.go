package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ixugo/goddd/domain/version/versionapi"
	"github.com/ixugo/goddd/pkg/web"

	"slate-kanban/internal/web/api/connectapi"
	"slate-kanban/internal/web/api/kanbanapi"
	"slate-kanban/internal/web/api/updateapi"
	"slate-kanban/internal/web/restactor"
)

var startRuntime = time.Now()

const (
	// apiPrefix 是全部 REST 接口的路径前缀。
	apiPrefix = "/api/v1"
	// mcpPath 是 MCP 服务的路径，认证与身份由 mcpapi 适配器处理。
	mcpPath = "/mcp"
)

// setupRouter 装配全部路由：安全兜底、日志、统计；健康检查、安装说明和 skill 免认证；REST 固定人身份。
func setupRouter(r *gin.Engine, uc *Usecase) {
	limiter := newAttemptLimiter(nil)
	guard := newPasscodeGuard(uc.Conf, limiter)

	r.Use(
		securityMiddleware(uc.Conf),
		passcodeMiddleware(guard),
		web.Recover(),
		web.Metrics(),
		web.Logger(),
		web.LoggerWithBody(web.DefaultBodyLimit,
			func(_ *gin.Context) bool {
				return !uc.Conf.Runtime.Debug
			},
			web.IgnorePath(authPrefix+"/login"),
		),
	)

	// 登录接口不要求口令，所以更要限制请求体大小和类型，免得被匿名用户用大请求体消耗内存
	registerAuthRoutes(r.Group("", limitBody(maxRequestBody), requireJSON()), guard)

	r.Any("/health", web.WrapH(uc.getHealth))
	r.GET("/app/metrics/api", web.WrapH(uc.getMetricsAPI))

	r.Any(mcpPath, gin.WrapH(uc.MCP.Handler()))
	connectapi.RegisterPublic(r, uc.Connect)

	v1 := r.Group(apiPrefix, restactor.Middleware(), limitBody(maxRequestBody), requireJSON())
	connectapi.Register(v1, uc.Connect)
	kanbanapi.Register(v1, uc.Kanban)
	updateapi.Register(v1, uc.Update)
	versionapi.Register(v1, uc.Version)
}
