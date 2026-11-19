package api

import (
	"fmt"
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
	"slate-kanban/internal/core/kanban/stores/blobfs"
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
// 图片文件放在资源目录（见 conf.ResourceDir）；旧版存在数据库里的图片在这里搬成文件，
// 搬迁失败就不启动，避免服务起来后图片读不到。
func NewKanbanCore(db *gorm.DB, bc *conf.Bootstrap) (kanban.Core, error) {
	dir, err := conf.ResourceDir(bc)
	if err != nil {
		return kanban.Core{}, err
	}
	res := blobfs.New(dir)
	store := kanbandb.NewDB(db).AutoMigrate(orm.GetEnabledAutoMigrate())
	if orm.GetEnabledAutoMigrate() {
		if err := store.MigrateAttachmentFiles(res); err != nil {
			return kanban.Core{}, fmt.Errorf("搬迁图片到资源目录: %w", err)
		}
	}
	return kanban.NewCore(store, kanban.WithResources(res)), nil
}

// NewMCP 创建 MCP 适配器，与看板 Core 共用事件通知。
func NewMCP(bc *conf.Bootstrap, core kanban.Core) (mcpapi.API, func()) {
	return mcpapi.New(core, bc.Runtime.BuildVersion)
}
