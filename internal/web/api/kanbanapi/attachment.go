package kanbanapi

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ixugo/goddd/pkg/reason"
	"github.com/ixugo/goddd/pkg/web"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/web/restactor"
)

const (
	// attachmentField 是上传表单里图片文件的字段名。
	attachmentField = "file"
	// attachmentFormSlack 是请求体比图片上限多留的余量，给 multipart 的边界和字段头用。
	attachmentFormSlack = 1 << 20
	// attachmentURLPrefix 是图片地址的前缀，网页的 Markdown 渲染只认这个前缀的图片。
	attachmentURLPrefix = "/api/v1/attachments/"
)

// attachmentRoutes 注册图片上传和读取的路由。
func (a API) attachmentRoutes(r gin.IRouter) {
	r.POST("/boards/:board_id/attachments", a.uploadAttachment)
	r.GET("/attachments/:token", a.getAttachment)
}

// attachmentResult 是上传成功的响应，url 可以直接写进 Markdown 的图片语法。
type attachmentResult struct {
	Token       string `json:"token"`
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

// uploadAttachment 处理 POST /boards/:board_id/attachments：表单字段 file 是图片。
// 请求体先限制大小再解析，超大文件不会被整个读进内存；类型由 Core 按内容判断。
func (a API) uploadAttachment(c *gin.Context) {
	boardID, err := strconv.ParseInt(c.Param("board_id"), 10, 64)
	if err != nil {
		web.Fail(c, reason.ErrBadRequest.WithMsg("看板编号不合法"))
		return
	}
	ctx := c.Request.Context()
	actor, err := restactor.From(ctx)
	if err != nil {
		web.Fail(c, err)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, kanban.MaxAttachmentBytes+attachmentFormSlack)
	data, err := readUploadedFile(c)
	if err != nil {
		web.Fail(c, err)
		return
	}
	item, err := a.core.SaveAttachment(ctx, actor, boardID, data)
	if err != nil {
		web.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, attachmentResult{
		Token: item.Token, URL: attachmentURLPrefix + item.Token, ContentType: item.ContentType, Size: item.Size,
	})
}

// readUploadedFile 读出表单里的图片字节，最多读上限加一个字节，多出来的交给 Core 判定为太大。
func readUploadedFile(c *gin.Context) ([]byte, error) {
	header, err := c.FormFile(attachmentField)
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return nil, kanban.ErrAttachmentTooLarge
		}
		return nil, kanban.ErrInvalidInput.WithMsg("请用表单字段 file 上传一张图片")
	}
	f, err := header.Open()
	if err != nil {
		return nil, kanban.ErrInvalidInput.WithMsg("读取上传文件失败")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, kanban.MaxAttachmentBytes+1))
	if err != nil {
		return nil, kanban.ErrInvalidInput.WithMsg("读取上传文件失败")
	}
	return data, nil
}

// getAttachment 处理 GET /attachments/:token，直接返回图片字节。
// 响应头把图片锁死成"只能当图片看"：禁止类型嗅探，并带上禁用一切脚本的内容安全策略；
// 编号是随机且不可改的，内容永远不变，所以可以长期缓存。
func (a API) getAttachment(c *gin.Context) {
	item, data, err := a.core.GetAttachment(c.Request.Context(), c.Param("token"))
	if err != nil {
		web.Fail(c, err)
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
	c.Header("Cache-Control", "private, max-age=31536000, immutable")
	c.Header("Content-Disposition", "inline")
	c.Data(http.StatusOK, item.ContentType, data)
}
