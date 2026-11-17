package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ixugo/goddd/domain/version/versionapi"
	"github.com/ixugo/goddd/pkg/web"

	"slate-kanban/internal/web/api/accessapi"
	"slate-kanban/internal/web/api/kanbanapi"
)

var startRuntime = time.Now()

const (
	// apiPrefix 是全部 REST 接口的路径前缀。
	apiPrefix = "/api/v1"
	// tokenPathPrefix 是令牌管理接口的路径前缀。创建令牌的响应里有令牌明文，
	// 无论什么环境，这些接口的请求体和响应体都不记日志。
	tokenPathPrefix = apiPrefix + "/agent-tokens"
)

// setupRouter 装配全部路由：日志、统计等全局中间件；免认证的健康检查；
// 以及需要令牌的 /api/v1 接口组。认证排在请求体检查之前，没有令牌的人看不到后面的检查结果。
func setupRouter(r *gin.Engine, uc *Usecase) {
	r.Use(
		web.Recover(),
		web.Metrics(),
		web.Logger(),
		// debug 环境中配合 debug 日志级别，记录请求体与响应体
		web.LoggerWithBody(web.DefaultBodyLimit,
			func(_ *gin.Context) bool {
				// true: 表示忽略记录日志
				// !debug 表示仅调试环境记录
				return !uc.Conf.Runtime.Debug
			},
			web.IgnorePrefix(tokenPathPrefix),
		),
	)

	r.Any("/health", web.WrapH(uc.getHealth))
	// 请求统计里有各接口的调用情况，需要令牌才能看
	r.GET("/app/metrics/api", uc.Access.Middleware(), web.WrapH(uc.getMetricsAPI))

	v1 := r.Group(apiPrefix, uc.Access.Middleware(), limitBody(maxRequestBody), requireJSON())
	accessapi.Register(v1, uc.Access)
	kanbanapi.Register(v1, uc.Kanban)
	versionapi.Register(v1, uc.Version)
}
