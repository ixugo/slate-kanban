// Package mcpapi 是看板领域的 MCP 适配器：agent 通过它读写看板，和 REST 一样只做
// "取身份、调 Core、转换结果"，权限判断和字段校验都留在 Core 里。
package mcpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"slate-kanban/internal/core/access"
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
	// defaultStreamRecheck 是通知流重新校验令牌的间隔。令牌被删除后，已经连着的通知流最多再存活这么久。
	defaultStreamRecheck = time.Minute
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

// config 是 New 的可选配置。
type config struct {
	notifyDelay   time.Duration
	streamRecheck time.Duration
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

// WithStreamRecheck 设置通知流重新校验令牌的间隔，默认 1 分钟。测试里调短它，才能验证令牌被删除后流会结束。
// 不是正数的值被忽略。
func WithStreamRecheck(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.streamRecheck = d
		}
	}
}

// New 创建 MCP 适配器：两个视图的 Server、通知器和 HTTP 处理器。version 是服务的构建版本号。
// 它会在 core 上注册观察者，所以必须在开始处理请求之前调用，并且同一个 core 只能调用一次。
// 返回的函数用来在服务退出时停止通知器，可以重复调用。
func New(acore access.Core, core kanban.Core, version string, opts ...Option) (API, func()) {
	cfg := config{notifyDelay: defaultNotifyDelay, streamRecheck: defaultStreamRecheck}
	for _, opt := range opts {
		opt(&cfg)
	}
	a := API{core: core, version: version}
	a.agent = a.newServer(viewAgent)
	a.admin = a.newServer(viewAdmin)
	a.handler = a.newHandler(acore, cfg.streamRecheck)
	n := newNotifier(cfg.notifyDelay, a.agent, a.admin)
	core.Observe(observerKey, n.mark)
	return a, n.close
}

// Handler 返回挂在 /mcp 上的 HTTP 处理器，已经包含认证。
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
	if v == viewAdmin {
		a.registerAdminTools(s)
	}
	return s
}

// newHandler 组装 HTTP 处理器：认证在最外层，通过后交给 SDK 处理会话和协议。
//   - JSON 响应：单个请求的结果直接用 JSON 返回，不占用长连接，对反向代理更友好；
//   - 关闭"本机服务只认本机 Host"的校验：服务放在 nginx 后面时 Host 是域名，会被它误拦，
//     而 /mcp 一定要令牌，网页脚本跨站请求时带不上自定义的 Authorization 头，不会因此多出攻击面；
//   - 会话绑定创建它的身份，换一个令牌拿不到别人的会话；
//   - 通知流（GET）随服务关闭而结束，并且定期重新校验令牌，见 limitStream。
func (a API) newHandler(acore access.Core, streamRecheck time.Duration) http.Handler {
	sdk := mcp.NewStreamableHTTPHandler(a.serverFor, &mcp.StreamableHTTPOptions{
		JSONResponse:               true,
		DisableLocalhostProtection: true,
		SessionTimeout:             sessionTimeout,
		MaxRequestBodyBytes:        maxRequestBody,
	})
	verify := newVerifier(acore)
	// 令牌没有过期时间，有效性由每个请求的认证决定，所以允许 TokenInfo 不带 Expiration。
	opts := &auth.RequireBearerTokenOptions{AllowMissingExpiration: true}
	return auth.RequireBearerToken(verify, opts)(limitStream(sdk, verify, streamRecheck))
}

// limitStream 给通知流加两个限制。通知流是客户端用 GET 打开、一直挂着的连接；
// 其他请求（POST 的 JSON 响应、DELETE）都是一问一答，不受影响。
//   - 服务开始关闭时立刻结束，否则 http.Server.Shutdown 要一直等到超时；
//   - 每隔 recheck 重新校验一次令牌，令牌被删除就结束，否则被撤销的 agent 能一直收到"看板有更新"的提示。
//     只在令牌失效时才结束流：定时无故断开会让一些客户端把"没有进展的重连"算作失败，最终放弃整个会话。
func limitStream(next http.Handler, verify auth.TokenVerifier, recheck time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}
		bound, stopBind := lifecycle.Bind(r.Context())
		defer stopBind()
		ctx, end := context.WithCancel(bound)
		defer end()
		go endWhenRevoked(ctx, end, verify, r, recheck)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// endWhenRevoked 每隔 interval 用请求带的令牌重新认证一次，令牌已经无效就调用 end 结束通知流；
// 认证本身出错（比如查库失败）不结束，等下一轮再查。ctx 结束时退出。
func endWhenRevoked(ctx context.Context, end context.CancelFunc, verify auth.TokenVerifier, r *http.Request, interval time.Duration) {
	fields := strings.Fields(r.Header.Get("Authorization"))
	if len(fields) != 2 { // 认证中间件已经放行过这个请求，正常不会走到这里
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := verify(ctx, fields[1], r); errors.Is(err, auth.ErrInvalidToken) {
				end()
				return
			}
		}
	}
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
