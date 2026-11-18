package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ixugo/goddd/pkg/reason"
	"github.com/ixugo/goddd/pkg/web"

	"slate-kanban/internal/core/kanban"
)

// maxRequestBody 是 /api/v1 下请求体的大小上限（字节）。看板里的内容都是文本，1 MiB 绰绰有余，
// 同时能防止一次请求吃光内存。
const maxRequestBody = 1 << 20

// uploadRoute 是图片上传的路由模式。它是唯一接收 multipart 表单和大请求体的接口，
// 其他接口仍然只收 JSON 且不超过 maxRequestBody；跨站写请求由 securityMiddleware 的 Origin 检查拦住。
const uploadRoute = apiPrefix + "/boards/:board_id/attachments"

// uploadBodyLimit 是图片上传请求体的上限：图片上限再多留 1 MiB 给表单边界和字段头。
const uploadBodyLimit = kanban.MaxAttachmentBytes + (1 << 20)

var (
	// errBodyTooLarge 请求体超过上限，状态码 413。
	errBodyTooLarge = reason.ErrContentTooLarge.WithMsg("请求体不能超过 1 MiB")
	// errNotJSON 请求带了请求体但不是 JSON，状态码 415。不拦下的话，框架会把 JSON 当表单解析，内容被悄悄忽略。
	errNotJSON = reason.ErrUnsupportedMediaType.WithMsg("请求体必须是 JSON，请设置 Content-Type: application/json").
			WithHTTPStatus(http.StatusUnsupportedMediaType)
)

// isUpload 判断当前请求是不是图片上传。用路由模式而不是真实路径比较，避免路径里的编号干扰。
func isUpload(c *gin.Context) bool {
	return c.Request.Method == http.MethodPost && c.FullPath() == uploadRoute
}

// limitBody 限制请求体大小。声明的长度超过上限就直接返回 413，不去读；
// 没有声明长度（分块传输）的请求用读取上限兜底，超出的部分读不到。图片上传改用 uploadBodyLimit。
func limitBody(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		bound := limit
		if isUpload(c) {
			bound = uploadBodyLimit
		}
		if c.Request.ContentLength > bound {
			web.AbortWithStatusJSON(c, errBodyTooLarge)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, bound)
		c.Next()
	}
}

// requireJSON 要求有请求体的请求必须是 JSON：Content-Type 的类型是 application/json（可以带 charset 等参数），
// 否则返回 415。没有请求体的请求不受影响，比如领取、释放这类不需要参数的 POST。图片上传是表单，不检查。
func requireJSON() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength != 0 && !isUpload(c) && !strings.EqualFold(c.ContentType(), "application/json") {
			web.AbortWithStatusJSON(c, errNotJSON)
			return
		}
		c.Next()
	}
}
