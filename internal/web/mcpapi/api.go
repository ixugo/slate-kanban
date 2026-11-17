// Package mcpapi 是看板领域的 MCP 适配器：agent 通过它读写看板，和 REST 一样只做
// "取身份、调 Core、转换结果"，权限判断和字段校验都留在 Core 里。
package mcpapi

import (
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"slate-kanban/internal/core/access"
	"slate-kanban/internal/core/kanban"
)

const (
	// serverName 是 MCP 服务对外的名称，host 的界面里会显示它，工具名前面通常也会带上它。
	serverName = "slate"
	// serverTitle 是给人看的服务名称。
	serverTitle = "Slate 看板"
	// sessionTimeout 是会话空闲多久后自动关闭。会话只占很少的内存，所以放宽到 6 小时，
	// agent 长时间没活干时不必频繁重新初始化；过期后客户端按协议重新初始化即可。
	sessionTimeout = 6 * time.Hour
	// maxRequestBody 是请求体的大小上限（字节），和 /api/v1 一致。看板里的内容都是文本，1 MiB 绰绰有余。
	maxRequestBody = 1 << 20
	// boardURITemplate 是看板资源的地址模板，读取它得到整板快照，订阅它得到"看板有更新"的通知。
	boardURITemplate = "slate://boards/{board_id}"
)

// view 是工具的视图：不同身份看到不同的工具，agent 只看到自己能用的，不被管理工具占满上下文。
// 视图只是减少干扰，不是安全边界，真正的权限判断在 Core 里。
type view string

const (
	// viewAgent 是 agent 令牌的视图：读取、拉取更新、领取和处理卡片。
	viewAgent view = "agent"
	// viewAdmin 是管理令牌（人）的视图：读取和全部管理操作。
	viewAdmin view = "admin"
)

// API 持有看板领域的业务入口和两个视图各自的 MCP Server。
type API struct {
	core    kanban.Core
	version string
	agent   *mcp.Server
	admin   *mcp.Server
	handler http.Handler
}

// New 创建 MCP 适配器。version 是服务的构建版本号。返回的函数用来在服务退出时释放后台资源。
func New(acore access.Core, core kanban.Core, version string) (API, func()) {
	a := API{core: core, version: version}
	a.agent = a.newServer(viewAgent)
	a.admin = a.newServer(viewAdmin)
	a.handler = a.newHandler(acore)
	return a, func() {}
}

// Handler 返回挂在 /mcp 上的 HTTP 处理器，已经包含认证。
func (a API) Handler() http.Handler {
	return a.handler
}

// newServer 创建一个视图对应的 MCP Server，并注册这个视图能用的全部工具。
func (a API) newServer(v view) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: serverName, Title: serverTitle, Version: a.version}, nil)
	a.registerReadTools(s, v)
	return s
}

// newHandler 组装 HTTP 处理器：认证在最外层，通过后交给 SDK 处理会话和协议。
//   - JSON 响应：单个请求的结果直接用 JSON 返回，不占用长连接，对反向代理更友好；
//   - 关闭"本机服务只认本机 Host"的校验：服务放在 nginx 后面时 Host 是域名，会被它误拦，
//     而 /mcp 一定要令牌，网页脚本跨站请求时带不上自定义的 Authorization 头，不会因此多出攻击面；
//   - 会话绑定创建它的身份，换一个令牌拿不到别人的会话。
func (a API) newHandler(acore access.Core) http.Handler {
	sdk := mcp.NewStreamableHTTPHandler(a.serverFor, &mcp.StreamableHTTPOptions{
		JSONResponse:               true,
		DisableLocalhostProtection: true,
		SessionTimeout:             sessionTimeout,
		MaxRequestBodyBytes:        maxRequestBody,
	})
	// 令牌没有过期时间，有效性由每个请求的认证决定，所以允许 TokenInfo 不带 Expiration。
	opts := &auth.RequireBearerTokenOptions{AllowMissingExpiration: true}
	return auth.RequireBearerToken(newVerifier(acore), opts)(sdk)
}

// serverFor 按请求的身份挑 MCP Server：人用管理视图，agent 用 agent 视图。
// 会话创建时调用一次，之后整个会话都用同一个 Server。认证中间件保证了这里一定有身份，
// 万一没有就返回 nil，SDK 会拒绝这个请求。
func (a API) serverFor(r *http.Request) *mcp.Server {
	actor, err := actorOf(auth.TokenInfoFromContext(r.Context()))
	if err != nil {
		return nil
	}
	if actor.IsHuman() {
		return a.admin
	}
	return a.agent
}
