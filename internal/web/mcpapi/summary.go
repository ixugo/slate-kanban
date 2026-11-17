package mcpapi

import (
	"time"

	"slate-kanban/internal/core/kanban"
)

const (
	// defaultListLimit 是 list_cards 不指定数量时返回的卡片数。摘要每张约几百字节，
	// 50 张足够 agent 了解一个看板的概况，又不会占掉太多上下文。
	defaultListLimit = 50
	// maxListLimit 是 list_cards 一次最多返回的卡片数。
	maxListLimit = 200
)

// cardBrief 是卡片自己的字段：没有描述和评论（描述最长 2 万字，刚写进去的内容不用再回显一遍，
// 一个看板几十张卡片全带上也会撑爆 agent 的上下文），也没有要靠整个看板才能算出来的派生状态。
// 写入类工具的结果用它。
type cardBrief struct {
	ID         int64      `json:"id"`
	Key        string     `json:"key"`
	Title      string     `json:"title"`
	ColumnID   int64      `json:"column_id"`
	Priority   int        `json:"priority"`
	Tags       []string   `json:"tags"`
	HumanOnly  bool       `json:"human_only"`
	ClaimedBy  string     `json:"claimed_by"`
	WaitsOn    []int64    `json:"waits_on"`
	DueAt      *time.Time `json:"due_at"`
	StartedAt  *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	Branch     string     `json:"branch"`
	URL        string     `json:"url"`
	Version    int64      `json:"version"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// cardSummary 是卡片的摘要：cardBrief 加上所在列的名称和三个派生状态。要看全文用 get_card。
type cardSummary struct {
	cardBrief
	Column      string `json:"column"`
	Done        bool   `json:"done"`
	Waiting     bool   `json:"waiting"`
	ClaimActive bool   `json:"claim_active"`
}

// briefOf 取出一张卡片的简要字段。标签和依赖保证是数组而不是 null，agent 不用到处判空。
func briefOf(c *kanban.Card) cardBrief {
	b := cardBrief{
		ID: c.ID, Key: c.Key, Title: c.Title, ColumnID: c.ColumnID, Priority: c.Priority,
		Tags: c.Tags, HumanOnly: c.HumanOnly, ClaimedBy: c.ClaimedBy, WaitsOn: c.WaitsOn,
		DueAt: c.DueAt, StartedAt: c.StartedAt, FinishedAt: c.FinishedAt,
		Branch: c.Branch, URL: c.URL, Version: c.Version, UpdatedAt: c.UpdatedAt,
	}
	if b.Tags == nil {
		b.Tags = []string{}
	}
	if b.WaitsOn == nil {
		b.WaitsOn = []int64{}
	}
	return b
}

// summarize 把一张卡片的视图缩成摘要。
func summarize(v *kanban.CardView) cardSummary {
	return cardSummary{cardBrief: briefOf(v.Card), Column: v.Column, Done: v.Done, Waiting: v.Waiting, ClaimActive: v.ClaimActive}
}

// summarizeAll 把一组卡片视图缩成摘要，保持顺序；没有卡片时返回空切片而不是 nil。
func summarizeAll(views []*kanban.CardView) []cardSummary {
	out := make([]cardSummary, len(views))
	for i, v := range views {
		out[i] = summarize(v)
	}
	return out
}

// clampListLimit 把 list_cards 的数量换成实际使用的数量：0 取默认值，超过上限取上限。
// 负数原样返回，由 Core 当作非法输入拒绝，这样错误说明和 REST 一致。
func clampListLimit(n int) int {
	switch {
	case n == 0:
		return defaultListLimit
	case n > maxListLimit:
		return maxListLimit
	}
	return n
}
