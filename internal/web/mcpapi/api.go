// Package mcpapi 是看板领域的 MCP 适配器：agent 通过它读写看板，和 REST 一样只做
// "取身份、调 Core、转换结果"，权限判断和字段校验都留在 Core 里。
package mcpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/lifecycle"
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
	// viewAgent 是工作视图的代号：读取、拉取更新、领取和处理卡片。
	viewAgent view = "agent"
	// viewAdmin 是管理视图的代号：读取和全部管理操作。
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

// config 是 New 的可选配置。
type config struct {
	notifyDelay time.Duration
}

// Option 调整 API 的可选行为。生产环境不需要传，主要给测试用。
type Option func(*config)

// WithNotifyDelay 设置通知的合并窗口，默认 200 毫秒。测试里调大它，才能稳定地验证"窗口内的多次更新只发一条"，
// 调小它，测试才不用干等。不是正数的值被忽略。
func WithNotifyDelay(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.notifyDelay = d
		}
	}
}

// New 创建 MCP 适配器：两个视图的 Server、通知器和 HTTP 处理器。version 是服务的构建版本号。
// 它会在 core 上注册观察者，所以必须在开始处理请求之前调用，并且同一个 core 只能调用一次。
// 返回的函数用来在服务退出时停止通知器，可以重复调用。
func New(core kanban.Core, version string, opts ...Option) (API, func()) {
	cfg := config{notifyDelay: defaultNotifyDelay}
	for _, opt := range opts {
		opt(&cfg)
	}
	a := API{core: core, version: version}
	a.agent = a.newServer(viewAgent)
	a.admin = a.newServer(viewAdmin)
	a.handler = a.newHandler()
	n := newNotifier(cfg.notifyDelay, a.agent, a.admin)
	core.Observe(observerKey, n.mark)
	return a, n.close
}

// Handler 返回挂在 /mcp 上的 HTTP 处理器。
func (a API) Handler() http.Handler {
	return a.handler
}

// newServer 创建一个视图对应的 MCP Server，并注册这个视图能用的全部工具。
func (a API) newServer(v view) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: serverName, Title: serverTitle, Version: a.version}, &mcp.ServerOptions{
		Instructions:       instructionsFor(v),
		SubscribeHandler:   a.subscribe,
		UnsubscribeHandler: a.unsubscribe,
	})
	a.registerResources(s)
	a.registerReadTools(s, v)
	if v == viewAgent {
		a.registerSyncTools(s)
	}
	a.registerWorkTools(s, v)
	a.registerCreateBoard(s)
	if v == viewAdmin {
		a.registerAdminTools(s)
	}
	return s
}

// newHandler 组装 HTTP 处理器：先校验网址上的 name/role，再交给 SDK 处理会话和协议。
// 关闭 SDK 自带的 localhost Host 校验，改由全局 securityMiddleware 统一做 Host 白名单。
// 通知流（GET）随服务关闭而结束，见 limitStream。
func (a API) newHandler() http.Handler {
	sdk := mcp.NewStreamableHTTPHandler(a.serverFor, &mcp.StreamableHTTPOptions{
		JSONResponse:               true,
		DisableLocalhostProtection: true,
		SessionTimeout:             sessionTimeout,
		MaxRequestBodyBytes:        maxRequestBody,
	})
	return identityMiddleware(bearerGate(limitStream(sdk)))
}

// limitStream 给通知流加生命周期限制。通知流是客户端用 GET 打开、一直挂着的连接；
// 其他请求（POST 的 JSON 响应、DELETE）都是一问一答，不受影响。服务开始关闭时立刻结束。
func limitStream(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}
		bound, stopBind := lifecycle.Bind(r.Context())
		defer stopBind()
		ctx, end := context.WithCancel(bound)
		defer end()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// serverFor 按 role 参数挑 MCP Server：管理视图用 admin，工作视图用 agent。
// 会话创建时调用一次，之后整个会话都用同一个 Server。
func (a API) serverFor(r *http.Request) *mcp.Server {
	if r.Header.Get(headerViewRole) == "manager" {
		return a.admin
	}
	return a.agent
}
