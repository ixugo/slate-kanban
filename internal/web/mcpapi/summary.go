package mcpapi

import (
	"time"

	"slate-kanban/internal/core/kanban"
)

const (
	// defaultListLimit 是 list_cards 不指定数量时返回的任务数。调度 agent 同时处理的任务不会太多，
	// 每次只要一小批，用筛选条件取下一批，不占太多上下文。
	defaultListLimit = 10
	// maxListLimit 是 list_cards 一次最多返回的任务数。
	maxListLimit = 30
)

// cardBrief 是任务自己的字段：没有描述和评论（描述最长 2 万字，刚写进去的内容不用再回显一遍，
// 一个看板几十个任务全带上也会撑爆 agent 的上下文），也没有要靠整个看板才能算出来的派生状态。
// 写入类工具的结果用它。
type cardBrief struct {
	ID        int64    `json:"id"`
	Key       string   `json:"key"`
	Title     string   `json:"title"`
	ColumnID  int64    `json:"column_id"`
	Priority  int      `json:"priority"`
	Tags      []string `json:"tags"`
	TagIDs    []int64  `json:"tag_ids"`
	HumanOnly bool     `json:"human_only"`
	ClaimedBy string   `json:"claimed_by"`
	// EngineerID 是任务绑定或分配的研发工程师，ClaimEngineerID 是现在领取它的工程师，0 时不出现。
	EngineerID      int64      `json:"engineer_id,omitempty"`
	ClaimEngineerID int64      `json:"claim_engineer_id,omitempty"`
	WaitsOn         []int64    `json:"waits_on"`
	DueAt           *time.Time `json:"due_at"`
	StartedAt       *time.Time `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at"`
	Branch          string     `json:"branch"`
	MergedCommits   []string   `json:"merged_commits,omitempty"`
	URL             string     `json:"url"`
	Version         int64      `json:"version"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// cardSummary 是任务的摘要：cardBrief 加上所在列的名称和三个派生状态。要看全文用 get_card。
type cardSummary struct {
	cardBrief
	Column      string `json:"column"`
	Done        bool   `json:"done"`
	Waiting     bool   `json:"waiting"`
	ClaimActive bool   `json:"claim_active"`
}

// briefOf 取出一个任务的简要字段。tags 是任务上带文字的标签的文字（由调用方算好传入），
// tag_ids 是全部标签的主键（含没有文字的）。标签和依赖保证是数组而不是 null，agent 不用到处判空。
func briefOf(c *kanban.Card, tags []string) cardBrief {
	b := cardBrief{
		ID: c.ID, Key: c.Key, Title: c.Title, ColumnID: c.ColumnID, Priority: c.Priority,
		Tags: tags, TagIDs: c.TagIDs, HumanOnly: c.HumanOnly, ClaimedBy: c.ClaimedBy, WaitsOn: c.WaitsOn,
		EngineerID: c.EngineerID, ClaimEngineerID: c.ClaimEngineerID,
		DueAt: c.DueAt, StartedAt: c.StartedAt, FinishedAt: c.FinishedAt,
		Branch: c.Branch, MergedCommits: c.MergedCommits, URL: c.URL, Version: c.Version, UpdatedAt: c.UpdatedAt,
	}
	if b.Tags == nil {
		b.Tags = []string{}
	}
	if b.TagIDs == nil {
		b.TagIDs = []int64{}
	}
	if b.WaitsOn == nil {
		b.WaitsOn = []int64{}
	}
	return b
}

// summarize 把一个任务的视图缩成摘要。
func summarize(v *kanban.CardView) cardSummary {
	return cardSummary{cardBrief: briefOf(v.Card, v.Tags), Column: v.Column, Done: v.Done, Waiting: v.Waiting, ClaimActive: v.ClaimActive}
}

// summarizeAll 把一组任务视图缩成摘要，保持顺序；没有任务时返回空切片而不是 nil。
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
