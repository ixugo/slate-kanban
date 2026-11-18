package kanbanapi

import (
	"encoding/json"
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

const (
	exportFormatJSON = "json"
	exportFormatMD   = "md"
	exportJSONType   = "slate-board-export"
	exportJSONVer    = 1
)

// boardExportJSON 是导出接口 JSON 格式的顶层信封，字段名与 REST 一致用 snake_case。
type boardExportJSON struct {
	Format     string               `json:"format"`
	Version    int                  `json:"version"`
	ExportedAt string               `json:"exported_at"`
	Seq        int64                `json:"seq"`
	Board      *kanban.Board        `json:"board"`
	Columns    []*kanban.Column     `json:"columns"`
	Rules      []*kanban.Rule       `json:"rules"`
	Cards      []*kanban.ExportCard `json:"cards"`
}

// export 处理 GET /boards/:board_id/export，format 默认 json，可选 md。
func (a API) export(c *gin.Context) {
	boardID, err := strconv.ParseInt(c.Param("board_id"), 10, 64)
	if err != nil {
		web.Fail(c, reason.ErrBadRequest.WithMsg("看板编号不合法"))
		return
	}
	format := strings.TrimSpace(strings.ToLower(c.Query("format")))
	if format == "" {
		format = exportFormatJSON
	}
	if format != exportFormatJSON && format != exportFormatMD {
		web.Fail(c, kanban.ErrInvalidInput.WithMsg("format 只能是 json 或 md"))
		return
	}
	ctx := c.Request.Context()
	actor, err := restactor.From(ctx)
	if err != nil {
		web.Fail(c, err)
		return
	}
	data, err := a.core.ExportBoard(ctx, actor, boardID)
	if err != nil {
		web.Fail(c, err)
		return
	}
	at := time.Now().UTC()
	name := exportAttachmentName(data.Board.Slug, at, format)
	c.Header("Content-Disposition", "attachment; filename=\""+name+"\"")
	switch format {
	case exportFormatJSON:
		c.Header("Content-Type", "application/json; charset=utf-8")
		body := boardExportJSON{
			Format: exportJSONType, Version: exportJSONVer, ExportedAt: at.Format(time.RFC3339),
			Seq: data.Seq, Board: data.Board, Columns: data.Columns, Rules: data.Rules, Cards: data.Cards,
		}
		enc := json.NewEncoder(c.Writer)
		enc.SetEscapeHTML(false)
		c.Status(http.StatusOK)
		if err := enc.Encode(body); err != nil {
			_ = c.Error(err)
		}
	default:
		c.Header("Content-Type", "text/markdown; charset=utf-8")
		c.Status(http.StatusOK)
		if _, err := c.Writer.Write(renderBoardMarkdown(data)); err != nil {
			_ = c.Error(err)
		}
	}
}

// exportAttachmentName 生成附件文件名：slug 小写、只含合法字符，加日期和后缀，防止头注入。
func exportAttachmentName(slug string, at time.Time, format string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(slug) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		b.WriteString("board")
	}
	ext := exportFormatJSON
	if format == exportFormatMD {
		ext = "md"
	}
	return b.String() + "-" + at.Format("20060102") + "." + ext
}
