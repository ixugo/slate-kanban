package kanbanapi

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"slate-kanban/internal/core/kanban"
)

var priorityLabels = [...]string{"未设置", "低", "中", "高"}

// renderBoardMarkdown 把 Core 导出的数据渲染成 Markdown 文本，按列分组、列内顺序与看板一致。
func renderBoardMarkdown(data *kanban.BoardExport) []byte {
	var buf bytes.Buffer
	b := data.Board
	fmt.Fprintf(&buf, "# %s（%s）\n\n", b.Name, b.Slug)
	if strings.TrimSpace(b.Description) != "" {
		buf.WriteString(b.Description)
		buf.WriteByte('\n')
	}
	buf.WriteByte('\n')
	keys := cardKeyIndex(data.Cards)
	byColumn := cardsByColumn(data.Cards)
	for _, col := range data.Columns {
		fmt.Fprintf(&buf, "## %s\n\n", col.Name)
		if strings.TrimSpace(col.Instruction) != "" {
			writeBlockquote(&buf, col.Instruction)
			buf.WriteByte('\n')
		}
		for _, card := range byColumn[col.ID] {
			writeExportCard(&buf, card, keys)
		}
	}
	return buf.Bytes()
}

// cardKeyIndex 建立任务主键到编号的映射，渲染依赖时用编号而不是内部主键。
func cardKeyIndex(cards []*kanban.ExportCard) map[int64]string {
	out := make(map[int64]string, len(cards))
	for _, c := range cards {
		if c.Card != nil {
			out[c.ID] = c.Key
		}
	}
	return out
}

// cardsByColumn 按列主键分组，每组内顺序与导出切片一致（已是看板顺序）。
func cardsByColumn(cards []*kanban.ExportCard) map[int64][]*kanban.ExportCard {
	out := make(map[int64][]*kanban.ExportCard)
	for _, c := range cards {
		if c.Card == nil {
			continue
		}
		out[c.ColumnID] = append(out[c.ColumnID], c)
	}
	return out
}

// writeExportCard 输出一个任务：标题、元信息行、描述和评论。
func writeExportCard(buf *bytes.Buffer, card *kanban.ExportCard, keys map[int64]string) {
	fmt.Fprintf(buf, "### %s %s\n\n", card.Key, card.Title)
	var meta []string
	if card.Priority > 0 && card.Priority < len(priorityLabels) {
		meta = append(meta, fmt.Sprintf("优先级: %s", priorityLabels[card.Priority]))
	}
	if len(card.Tags) > 0 {
		meta = append(meta, "标签: "+strings.Join(card.Tags, ", "))
	}
	if t := formatOptionalTime(card.DueAt); t != "" {
		meta = append(meta, "截止时间: "+t)
	}
	if t := formatOptionalTime(card.StartedAt); t != "" {
		meta = append(meta, "开始时间: "+t)
	}
	if t := formatOptionalTime(card.FinishedAt); t != "" {
		meta = append(meta, "完成时间: "+t)
	}
	if card.ClaimedBy != "" {
		meta = append(meta, "领取者: "+card.ClaimedBy)
	}
	if card.Branch != "" {
		meta = append(meta, "branch: "+card.Branch)
	}
	if card.URL != "" {
		meta = append(meta, "url: "+card.URL)
	}
	if card.HumanOnly {
		meta = append(meta, "仅人工")
	}
	if len(card.WaitsOn) > 0 {
		var deps []string
		for _, id := range card.WaitsOn {
			if k := keys[id]; k != "" {
				deps = append(deps, k)
			}
		}
		if len(deps) > 0 {
			meta = append(meta, "依赖: "+strings.Join(deps, ", "))
		}
	}
	if len(meta) > 0 {
		buf.WriteString(strings.Join(meta, " · "))
		buf.WriteByte('\n')
		buf.WriteByte('\n')
	}
	if strings.TrimSpace(card.Description) != "" {
		buf.WriteString(card.Description)
		buf.WriteByte('\n')
		buf.WriteByte('\n')
	}
	if len(card.Comments) > 0 {
		buf.WriteString("#### 评论\n\n")
		for _, cm := range card.Comments {
			fmt.Fprintf(buf, "- **%s** (%s)\n\n", cm.Author, cm.CreatedAt.UTC().Format(time.RFC3339))
			writeBlockquote(buf, cm.Body)
			buf.WriteByte('\n')
		}
	}
	buf.WriteByte('\n')
}

// writeBlockquote 把多行正文变成 Markdown 引用块。
func writeBlockquote(buf *bytes.Buffer, text string) {
	for line := range strings.SplitSeq(strings.TrimRight(text, "\n"), "\n") {
		buf.WriteString("> ")
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
}

// formatOptionalTime 把时间格式化成 RFC3339，nil 返回空串表示不输出该项。
func formatOptionalTime(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
