package kanbanapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ixugo/goddd/pkg/reason"
	"github.com/ixugo/goddd/pkg/web"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/web/restactor"
)

// templateRoutes 注册自定义项目模板相关路由。
func (a API) templateRoutes(r gin.IRouter) {
	r.GET("/templates", route(a.listTemplates))
	r.POST("/templates", a.importTemplate)
	r.DELETE("/templates/:template_id", route(a.deleteTemplate))
	r.GET("/boards/:board_id/template-export", a.exportBoardTemplate)
}

// importTemplateQuery 是导入模板时的查询参数。
type importTemplateQuery struct {
	Replace bool `form:"replace"`
}

// listTemplates 返回全部自定义项目模板摘要。
func (a API) listTemplates(ctx context.Context, actor kanban.Actor, _ *struct{}) ([]kanban.ProjectTemplateSummary, error) {
	return a.core.ListProjectTemplates(ctx, actor)
}

// deleteTemplateInput 是删除模板的路径参数。
type deleteTemplateInput struct {
	TemplateID int64 `uri:"template_id" json:"-" form:"-"`
}

// deleteTemplate 删除一条自定义模板。
func (a API) deleteTemplate(ctx context.Context, actor kanban.Actor, in *deleteTemplateInput) (empty, error) {
	return empty{}, a.core.DeleteProjectTemplate(ctx, actor, in.TemplateID)
}

// importTemplate 接收 JSON 请求体导入模板，可用 replace=true 覆盖同名。
func (a API) importTemplate(c *gin.Context) {
	actor, err := restactor.From(c.Request.Context())
	if err != nil {
		web.Fail(c, err)
		return
	}
	var q importTemplateQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		web.Fail(c, reason.ErrBadRequest.WithCause(err))
		return
	}
	limited := io.LimitReader(c.Request.Body, kanban.MaxProjectTemplateImportBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		web.Fail(c, reason.ErrBadRequest.WithCause(err))
		return
	}
	out, err := a.core.ImportProjectTemplate(c.Request.Context(), actor, raw, q.Replace)
	if err != nil {
		web.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// exportBoardTemplate 把看板导出为项目模板 JSON 文件下载。
func (a API) exportBoardTemplate(c *gin.Context) {
	boardID, err := strconv.ParseInt(c.Param("board_id"), 10, 64)
	if err != nil {
		web.Fail(c, reason.ErrBadRequest.WithMsg("看板编号不合法"))
		return
	}
	actor, err := restactor.From(c.Request.Context())
	if err != nil {
		web.Fail(c, err)
		return
	}
	file, err := a.core.ExportProjectTemplateFromBoard(c.Request.Context(), actor, boardID)
	if err != nil {
		web.Fail(c, err)
		return
	}
	at := time.Now().UTC()
	name := templateAttachmentName(file.Name, at)
	c.Header("Content-Disposition", "attachment; filename=\""+name+"\"")
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.Status(http.StatusOK)
	enc := json.NewEncoder(c.Writer)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(file); err != nil {
		_ = c.Error(err)
	}
}

// templateAttachmentName 生成模板导出文件名。
func templateAttachmentName(templateName string, at time.Time) string {
	var b strings.Builder
	for _, r := range templateName {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '-', r == '_', r >= 0x4e00 && r <= 0x9fff:
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	if b.Len() == 0 {
		b.WriteString("template")
	}
	return b.String() + "-" + at.Format("20060102") + ".json"
}
