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
	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
	"slate-kanban/internal/web/api/connectapi"
	"slate-kanban/internal/web/api/kanbanapi"
	"slate-kanban/internal/web/mcpapi"
	"slate-kanban/internal/web/spa"
)

var (
	ProviderVersionSet = wire.NewSet(versionapi.NewVersionCore)
	ProviderSet        = wire.NewSet(
		wire.Struct(new(Usecase), "*"),
		NewHTTPHandler,
		versionapi.New,
		NewKanbanCore,
		kanbanapi.New,
		NewMCP,
		ProvideConnectAPI,
	)
)

// Usecase 汇总路由需要的全部依赖，由 Wire 按字段装配。
type Usecase struct {
	Conf    *conf.Bootstrap
	DB      *gorm.DB
	Version versionapi.API
	Kanban  kanbanapi.API
	MCP     mcpapi.API
	Connect connectapi.API
}

// NewHTTPHandler 生成 Gin 路由：全局安全中间件、REST/MCP/静态页，未匹配路径走 SPA 回退或 JSON 404。
func NewHTTPHandler(uc *Usecase) http.Handler {
	cfg := uc.Conf
	conf.WarnShortPasscode(uc.Conf)
	if !uc.Conf.Runtime.Debug {
		gin.SetMode(gin.ReleaseMode)
	}
	g := gin.New()
	g.NoRoute(spa.NoRouteHandler(spa.ContentFS()))
	if cfg.Server.HTTP.PProf.Enabled {
		web.SetupPProf(g, &cfg.Server.HTTP.PProf.AccessIps)
	}
	setupRouter(g, uc)
	uc.Version.RecordVersion()
	return g
}

// NewUniqueID 生成唯一 id。
func NewUniqueID(db *gorm.DB) uniqueid.Core {
	store := uniqueiddb.NewDB(db).AutoMigrate(orm.GetEnabledAutoMigrate())
	return uniqueid.NewCore(store, 6)
}

// ProvideConnectAPI 按是否配置口令生成接入说明（只传布尔，不传口令本身），供 Wire 装配。
func ProvideConnectAPI(bc *conf.Bootstrap) connectapi.API {
	return connectapi.New(conf.PasscodeRequired(bc))
}

// NewKanbanCore 创建看板领域并迁移表；REST、SSE、MCP 共用同一个 Core。
func NewKanbanCore(db *gorm.DB) kanban.Core {
	store := kanbandb.NewDB(db).AutoMigrate(orm.GetEnabledAutoMigrate())
	return kanban.NewCore(store)
}

// NewMCP 创建 MCP 适配器，与看板 Core 共用事件通知。
func NewMCP(bc *conf.Bootstrap, core kanban.Core) (mcpapi.API, func()) {
	return mcpapi.New(core, bc.Runtime.BuildVersion)
}
