package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	"github.com/ixugo/goddd/domain/uniqueid"
	"github.com/ixugo/goddd/domain/uniqueid/store/uniqueiddb"
	"github.com/ixugo/goddd/domain/version/versionapi"
	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/web"
	"gorm.io/gorm"

	"slate-kanban/internal/conf"
	"slate-kanban/internal/core/access"
	"slate-kanban/internal/core/access/stores/accessdb"
	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
	"slate-kanban/internal/web/api/accessapi"
	"slate-kanban/internal/web/api/kanbanapi"
	"slate-kanban/internal/web/mcpapi"
)

var (
	ProviderVersionSet = wire.NewSet(versionapi.NewVersionCore)
	ProviderSet        = wire.NewSet(
		wire.Struct(new(Usecase), "*"),
		NewHTTPHandler,
		versionapi.New,
		NewAccessCore,
		accessapi.New,
		NewKanbanCore,
		kanbanapi.New,
		NewMCP,
	)
)

// Usecase 汇总路由需要的全部依赖，由 Wire 按字段装配。
type Usecase struct {
	Conf    *conf.Bootstrap
	DB      *gorm.DB
	Version versionapi.API
	Access  accessapi.API
	Kanban  kanbanapi.API
	MCP     mcpapi.API
}

// NewHTTPHandler 生成Gin框架路由内容
func NewHTTPHandler(uc *Usecase) http.Handler {
	cfg := uc.Conf
	// 如果不处于调试模式，将 Gin 设置为发布模式
	if !uc.Conf.Runtime.Debug {
		gin.SetMode(gin.ReleaseMode)
	}
	g := gin.New()
	// 处理未找到路由的情况，返回 JSON 格式的 404 错误信息
	g.NoRoute(func(c *gin.Context) {
		c.JSON(404, "来到了无人的荒漠")
	})
	// 如果启用了 Pprof，设置 Pprof 监控
	if cfg.Server.HTTP.PProf.Enabled {
		web.SetupPProf(g, &cfg.Server.HTTP.PProf.AccessIps)
	}

	setupRouter(g, uc)
	// 在确认所有表迁移完成后，再更新版本记录
	// 防止更新中断的情况，后续启动中无法更新版本号
	uc.Version.RecordVersion()
	return g
}

// NewUniqueID 生成唯一 id
func NewUniqueID(db *gorm.DB) uniqueid.Core {
	store := uniqueiddb.NewDB(db).AutoMigrate(orm.GetEnabledAutoMigrate())
	return uniqueid.NewCore(store, 6)
}

// NewAccessCore 创建令牌领域：令牌表接到数据库，管理令牌取自配置。
// Core 构造函数带可变参数，Wire 不能直接用，所以这里包一层。表结构是否迁移由全局开关决定，和 NewUniqueID 一致。
func NewAccessCore(db *gorm.DB, bc *conf.Bootstrap) access.Core {
	store := accessdb.NewDB(db).AutoMigrate(orm.GetEnabledAutoMigrate())
	return access.NewCore(store, bc.Auth.AdminToken)
}

// NewKanbanCore 创建看板领域：看板相关的表接到数据库。
// 同样因为 Core 构造函数带可变参数而包一层；REST、SSE 和之后的 MCP 共用这一个 Core，
// 这样任何入口产生的更新，都能通过它内部的订阅通知到所有等待的连接。
func NewKanbanCore(db *gorm.DB) kanban.Core {
	store := kanbandb.NewDB(db).AutoMigrate(orm.GetEnabledAutoMigrate())
	return kanban.NewCore(store)
}

// NewMCP 创建 MCP 适配器。它和 REST、SSE 共用同一个令牌 Core 和看板 Core：
// 认证是同一批令牌，任何入口产生的更新都会通知到订阅了看板的 agent。
// mcpapi.New 带可变参数的选项，Wire 不能直接用，所以这里包一层，不传选项；
// 返回的清理函数停止通知器，由 Wire 在程序退出时调用。
func NewMCP(bc *conf.Bootstrap, acore access.Core, core kanban.Core) (mcpapi.API, func()) {
	return mcpapi.New(acore, core, bc.Runtime.BuildVersion)
}
