package kanban

import (
	"slices"
	"strings"
	"time"
)

const (
	// maxCardTitleLen 卡片标题的最大字符数，和数据库列长度一致。
	maxCardTitleLen = 200
	// maxCardDescLen 卡片描述的最大字符数。描述会进入 agent 的上下文，所以不能无限长。
	maxCardDescLen = 20000
	// maxCardTags 一张卡片最多几个标签。
	maxCardTags = 10
	// maxCardTagLen 单个标签的最大字符数。
	maxCardTagLen = 30
	// maxCardBranchLen 关联分支名的最大字符数，和数据库列长度一致。
	maxCardBranchLen = 200
	// maxCardURLLen 关联链接的最大字符数，和数据库列长度一致。
	maxCardURLLen = 500
	// maxCardPriority 优先级的最大值：0 未设置，1 低，2 中，3 高。
	maxCardPriority = 3
)

// CreateCardInput 是创建卡片的入参。ColumnID 为 0 表示放进看板的第一列。
// 时间字段用 RFC3339 格式的字符串，空字符串表示没有。
type CreateCardInput struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	ColumnID    int64    `json:"column_id"`
	Priority    int      `json:"priority"`
	Tags        []string `json:"tags"`
	HumanOnly   bool     `json:"human_only"`
	WaitsOn     []int64  `json:"waits_on"`
	DueAt       string   `json:"due_at"`
	StartedAt   string   `json:"started_at"`
	FinishedAt  string   `json:"finished_at"`
	Branch      string   `json:"branch"`
	URL         string   `json:"url"`
}

// UpdateCardInput 是修改卡片的入参。指针字段为 nil 表示不修改，所以"清空"和"不改"是分得开的：
// 标签和依赖传空数组表示清空，时间传空字符串表示清除。
// Version 是调用方上次读到的版本号，带了而且和当前不一致就说明有人先改过，返回 409。
type UpdateCardInput struct {
	Title       *string   `json:"title"`
	Description *string   `json:"description"`
	Priority    *int      `json:"priority"`
	Tags        *[]string `json:"tags"`
	HumanOnly   *bool     `json:"human_only"`
	WaitsOn     *[]int64  `json:"waits_on"`
	DueAt       *string   `json:"due_at"`
	StartedAt   *string   `json:"started_at"`
	FinishedAt  *string   `json:"finished_at"`
	Branch      *string   `json:"branch"`
	URL         *string   `json:"url"`
	Version     *int64    `json:"version"`
}

// MoveCardInput 是移动卡片的入参。ColumnID 为 0 表示留在当前列（只调整顺序）。
// BeforeID 和 AfterID 最多填一个：放到那张卡片的前面或后面；都不填就放到目标列的末尾。
type MoveCardInput struct {
	ColumnID int64 `json:"column_id"`
	BeforeID int64 `json:"before_id"`
	AfterID  int64 `json:"after_id"`
}

// asPatch 把创建入参转成"所有字段都出现"的修改入参，创建和修改因此共用同一套校验和赋值。
// agent 创建卡片时规划类字段（优先级、HumanOnly、依赖和三个时间）不会进入修改入参，
// 调用方要先用 planningSet 拒绝 agent 填了它们的请求。
func (in CreateCardInput) asPatch(agent bool) UpdateCardInput {
	patch := UpdateCardInput{
		Title: &in.Title, Description: &in.Description, Tags: &in.Tags, Branch: &in.Branch, URL: &in.URL,
	}
	if !agent {
		patch.Priority, patch.HumanOnly, patch.WaitsOn = &in.Priority, &in.HumanOnly, &in.WaitsOn
		patch.DueAt, patch.StartedAt, patch.FinishedAt = &in.DueAt, &in.StartedAt, &in.FinishedAt
	}
	return patch
}

// planningSet 判断创建入参里有没有填规划类字段。
func (in CreateCardInput) planningSet() bool {
	return in.Priority != 0 || in.HumanOnly || len(in.WaitsOn) > 0 ||
		in.DueAt != "" || in.StartedAt != "" || in.FinishedAt != ""
}

// touchesPlanning 判断这次修改有没有出现规划类字段：优先级、HumanOnly、依赖、到期、开始、完成时间。
// 这些是人对工作的安排，agent 不能改。只看字段是否出现，不看值有没有变，规则才简单可预期。
func (in UpdateCardInput) touchesPlanning() bool {
	return in.Priority != nil || in.HumanOnly != nil || in.WaitsOn != nil ||
		in.DueAt != nil || in.StartedAt != nil || in.FinishedAt != nil
}

// assign 在 src 出现并且和 *dst 不同时写入，并记下字段名。
func assign[T comparable](dst *T, src *T, field string, changed *[]string) {
	if src != nil && *src != *dst {
		*dst = *src
		*changed = append(*changed, field)
	}
}

// apply 校验入参里出现的每个字段并写到卡片上，返回真正发生变化的字段名（和 JSON 字段名一致）。
// 它修改的是调用方读出的卡片副本，校验失败时调用方丢弃这个副本即可，数据库不受影响。
// 依赖是否存在、是否成环要用到整个看板的卡片，不在这里检查。
func (in UpdateCardInput) apply(c *Card) ([]string, error) {
	var changed []string
	for _, step := range []func(*Card) ([]string, error){in.applyTexts, in.applyFlags, in.applyLists, in.applyTimes} {
		fields, err := step(c)
		if err != nil {
			return nil, err
		}
		changed = append(changed, fields...)
	}
	return changed, nil
}

// applyTexts 处理标题、描述、分支和链接。标题去掉首尾空白后不能为空。
func (in UpdateCardInput) applyTexts(c *Card) ([]string, error) {
	var changed []string
	if in.Title != nil {
		title := strings.TrimSpace(*in.Title)
		if err := checkLine("卡片标题", title, 1, maxCardTitleLen); err != nil {
			return nil, err
		}
		assign(&c.Title, &title, "title", &changed)
	}
	if in.Description != nil {
		if err := checkText("卡片描述", *in.Description, 0, maxCardDescLen); err != nil {
			return nil, err
		}
		assign(&c.Description, in.Description, "description", &changed)
	}
	if in.Branch != nil {
		branch := strings.TrimSpace(*in.Branch)
		if err := checkLine("分支名", branch, 0, maxCardBranchLen); err != nil {
			return nil, err
		}
		assign(&c.Branch, &branch, "branch", &changed)
	}
	if in.URL != nil {
		url := strings.TrimSpace(*in.URL)
		if err := checkURL(url); err != nil {
			return nil, err
		}
		assign(&c.URL, &url, "url", &changed)
	}
	return changed, nil
}

// applyFlags 处理优先级和 HumanOnly。
func (in UpdateCardInput) applyFlags(c *Card) ([]string, error) {
	var changed []string
	if in.Priority != nil {
		if *in.Priority < 0 || *in.Priority > maxCardPriority {
			return nil, invalidf("优先级必须在 0 到 %d 之间", maxCardPriority)
		}
		assign(&c.Priority, in.Priority, "priority", &changed)
	}
	assign(&c.HumanOnly, in.HumanOnly, "human_only", &changed)
	return changed, nil
}

// applyLists 处理标签和依赖。两者都先整理（去重、检查数量），和现有值逐项比较后才算变化。
func (in UpdateCardInput) applyLists(c *Card) ([]string, error) {
	var changed []string
	if in.Tags != nil {
		tags, err := normalizeTags(*in.Tags)
		if err != nil {
			return nil, err
		}
		if !slices.Equal(tags, c.Tags) {
			c.Tags = tags
			changed = append(changed, "tags")
		}
	}
	if in.WaitsOn != nil {
		deps, err := normalizeWaitsOn(*in.WaitsOn)
		if err != nil {
			return nil, err
		}
		if !slices.Equal(deps, c.WaitsOn) {
			c.WaitsOn = deps
			changed = append(changed, "waits_on")
		}
	}
	return changed, nil
}

// applyTimes 处理到期、开始、完成三个时间。空字符串表示清除。
func (in UpdateCardInput) applyTimes(c *Card) ([]string, error) {
	var changed []string
	targets := []struct {
		field string
		src   *string
		dst   **time.Time
	}{
		{"due_at", in.DueAt, &c.DueAt},
		{"started_at", in.StartedAt, &c.StartedAt},
		{"finished_at", in.FinishedAt, &c.FinishedAt},
	}
	for _, t := range targets {
		if t.src == nil {
			continue
		}
		parsed, err := parseOptionalTime(t.field, *t.src)
		if err != nil {
			return nil, err
		}
		if !sameTime(parsed, *t.dst) {
			*t.dst = parsed
			changed = append(changed, t.field)
		}
	}
	return changed, nil
}

// normalizeTags 整理标签：去掉首尾空白，每个 1 到 30 个字符的单行文本，
// 不区分大小写去重并保留首次出现的写法，最多 10 个。返回的永远不是 nil。
func normalizeTags(tags []string) ([]string, error) {
	out := make([]string, 0, len(tags))
	seen := make(map[string]bool, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if err := checkLine("标签", tag, 1, maxCardTagLen); err != nil {
			return nil, err
		}
		if key := strings.ToLower(tag); !seen[key] {
			seen[key] = true
			out = append(out, tag)
		}
	}
	if len(out) > maxCardTags {
		return nil, invalidf("一张卡片最多 %d 个标签", maxCardTags)
	}
	return out, nil
}

// checkURL 校验关联链接：可以为空；非空时必须是 http 或 https 开头的单行文本。
// 只放行这两种协议，网页才能放心把它渲染成可点击的链接，不会被塞进 javascript: 之类的地址。
func checkURL(s string) error {
	if s == "" {
		return nil
	}
	if err := checkLine("链接", s, 1, maxCardURLLen); err != nil {
		return err
	}
	lower := strings.ToLower(s)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return invalidf("链接必须以 http:// 或 https:// 开头")
	}
	return nil
}

// parseOptionalTime 解析 RFC3339 时间并转成 UTC；空字符串返回 nil，表示没有。
func parseOptionalTime(field, s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, invalidf("%s 必须是 RFC3339 格式，例如 2026-11-17T09:30:00+08:00", field)
	}
	t = t.UTC()
	return &t, nil
}

// sameTime 判断两个可空时间是否表示同一时刻。
func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}
