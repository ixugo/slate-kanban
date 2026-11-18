// Package connectapi 提供 agent 的接入说明：免认证的安装说明页和 skill 下载，以及需要令牌的结构化说明。
// 内容全部编译在服务里，没有外部依赖。
package connectapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ixugo/goddd/pkg/web"

	"slate-kanban/skills"
)

const (
	// markdownType 是说明页的内容类型。
	markdownType = "text/markdown; charset=utf-8"
	// installPath 和 skillPath 是免认证的两个页面的路径。
	// 免认证是因为 agent 在拿到令牌、装好 MCP 之前就要读它们；里面没有任何敏感信息。
	installPath = "/install.md"
	skillPath   = "/skills/" + skillName + "/SKILL.md"
)

// API 是接入说明接口。内容都是编译进服务的，所以没有依赖。
type API struct{}

// New 创建接入说明接口。
func New() API {
	return API{}
}

// RegisterPublic 注册免认证的两个页面：安装说明和 skill 下载。它们挂在根路径上，不在 /api/v1 下。
func RegisterPublic(r gin.IRouter, a API) {
	r.GET(installPath, a.installDoc)
	r.GET(skillPath, a.skillDoc)
}

// Register 注册需要令牌的结构化说明。r 应该是已经套好认证的 /api/v1 路由组。
func Register(r gin.IRouter, a API) {
	r.GET("/connect", web.WrapH(a.connect))
}

// installDoc 返回 Markdown 形式的安装说明。地址按请求算出来，每个访问者看到的不同，所以禁止缓存；
// 禁止浏览器猜内容类型，免得被当成网页执行。主机不合法时返回 400，不输出任何说明。
func (a API) installDoc(c *gin.Context) {
	base, err := baseURL(c.Request)
	if err != nil {
		web.AbortWithStatusJSON(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, markdownType, []byte(renderMarkdown(buildGuide(base))))
}

// skillDoc 返回 skill 全文，就是服务里编译进去的那一份。它随服务升级而变，
// 所以用 no-cache：可以缓存，但每次使用前要向服务确认是不是最新。
func (a API) skillDoc(c *gin.Context) {
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, markdownType, []byte(skills.SlateKanban()))
}

// connect 返回 JSON 形式的接入说明，网页的"连接 Agent"用它渲染带复制按钮的命令。
func (a API) connect(c *gin.Context, _ *struct{}) (*Guide, error) {
	base, err := baseURL(c.Request)
	if err != nil {
		return nil, err
	}
	g := buildGuide(base)
	return &g, nil
}
