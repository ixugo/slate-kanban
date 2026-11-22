package kanbanapi

import (
	"testing"
	"time"

	"slate-kanban/internal/core/kanban"
)

// TestRenderBoardMarkdownGolden 用小数据集逐字对比 Markdown，防止导出排版悄悄变样。
func TestRenderBoardMarkdownGolden(t *testing.T) {
	due := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	data := &kanban.BoardExport{
		BoardDetail: &kanban.BoardDetail{
			Board: &kanban.Board{Name: "示例看板", Slug: "DEMO", Description: "看板说明一行"},
			Columns: []*kanban.Column{
				{ID: 1, Name: "待办", Instruction: "给 agent 的指令"},
			},
			Rules: []*kanban.Rule{},
		},
		Cards: []*kanban.ExportCard{
			{
				CardView: &kanban.CardView{
					Card: &kanban.Card{
						ID: 10, ColumnID: 1, Key: "DEMO-1", Title: "第一张卡",
						Description: "任务正文", Priority: 2, TagIDs: []int64{1},
						DueAt: &due,
					},
					Tags:   []string{"api"},
					Column: "待办",
				},
				Comments: []*kanban.Comment{
					{Author: "human", Body: "请跟进", CreatedAt: time.Date(2026, 1, 2, 8, 0, 0, 0, time.UTC)},
				},
			},
		},
		Seq: 5,
	}
	got := string(renderBoardMarkdown(data))
	const want = `# 示例看板（DEMO）

看板说明一行

## 待办

> 给 agent 的指令

### DEMO-1 第一张卡

优先级: 中 · 标签: api · 截止时间: 2026-03-01T12:00:00Z

任务正文

#### 评论

- **human** (2026-01-02T08:00:00Z)

> 请跟进


`
	if got != want {
		t.Fatalf("Markdown 与黄金文本不一致\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
