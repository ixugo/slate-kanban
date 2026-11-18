package kanban

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"slices"
	"strings"
)

const (
	// maxBoardLabels 一个看板的标签库最多几项。
	maxBoardLabels = 60
	// labelIDBytes 标签编号的随机字节数，编码成十六进制后是它的两倍长。
	labelIDBytes = 4
)

// labelHues 是标签颜色的色相，每个色相有三档深浅（1 最深，3 最浅），共 30 种，和 Trello 的配色一一对应。
var labelHues = []string{"green", "yellow", "orange", "red", "purple", "blue", "sky", "lime", "pink", "gray"}

// validLabelColor 判断颜色编号是否合法：空表示没有颜色，否则是"色相-深浅"，例如 green-2。
func validLabelColor(color string) bool {
	if color == "" {
		return true
	}
	hue, shade, ok := strings.Cut(color, "-")
	return ok && slices.Contains(labelHues, hue) && (shade == "1" || shade == "2" || shade == "3")
}

// validLabelID 判断标签编号是否是本服务生成的格式：8 位小写十六进制。
func validLabelID(id string) bool {
	if len(id) != labelIDBytes*2 {
		return false
	}
	for _, r := range id {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// normalizeLabelInputs 整理并校验标签库入参：名称按卡片标签的规则校验，颜色必须合法，
// 编号为空或格式正确且不重复，名称不区分大小写不重复，总数不超过上限。
func normalizeLabelInputs(in []LabelInput) ([]LabelInput, error) {
	if len(in) > maxBoardLabels {
		return nil, invalidf("一个看板最多 %d 个标签", maxBoardLabels)
	}
	out := make([]LabelInput, 0, len(in))
	ids := make(map[string]bool, len(in))
	names := make(map[string]bool, len(in))
	for _, l := range in {
		l.Name = strings.TrimSpace(l.Name)
		if err := checkLine("标签", l.Name, 1, maxCardTagLen); err != nil {
			return nil, err
		}
		if !validLabelColor(l.Color) {
			return nil, invalidf("标签颜色不合法：%s", l.Color)
		}
		if l.ID != "" && !validLabelID(l.ID) {
			return nil, invalidf("标签编号不合法")
		}
		if l.ID != "" && ids[l.ID] {
			return nil, invalidf("标签编号重复")
		}
		ids[l.ID] = true
		key := strings.ToLower(l.Name)
		if names[key] {
			return nil, invalidf("标签名称重复：%s", l.Name)
		}
		names[key] = true
		out = append(out, l)
	}
	return out, nil
}

// newLabelID 生成一个随机的标签编号。
func newLabelID() (string, error) {
	raw := make([]byte, labelIDBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", wrapDB("生成标签编号", err)
	}
	return hex.EncodeToString(raw), nil
}

// labelPlan 是一次标签库修改要同步到卡片上的变化，键都是小写的旧标签名。
type labelPlan struct {
	labels  []Label
	renamed map[string]string // 旧名 → 新名
	removed map[string]bool   // 被删掉的标签
}

// planLabels 对比修改前后的标签库，算出新的标签库和要同步到卡片上的改名、删除。
// 编号不在原库里的入参一律当新增并分配新编号，不信任调用方自带的编号。
func planLabels(prev []Label, in []LabelInput) (labelPlan, error) {
	plan := labelPlan{renamed: map[string]string{}, removed: map[string]bool{}}
	known := make(map[string]Label, len(prev))
	for _, l := range prev {
		known[l.ID] = l
	}
	kept := make(map[string]bool, len(in))
	names := make(map[string]bool, len(in))
	for _, l := range in {
		id := l.ID
		old, exists := known[id]
		if !exists {
			var err error
			if id, err = newLabelID(); err != nil {
				return plan, err
			}
		} else {
			kept[id] = true
			if old.Name != l.Name {
				plan.renamed[strings.ToLower(old.Name)] = l.Name
			}
		}
		names[strings.ToLower(l.Name)] = true
		plan.labels = append(plan.labels, Label{ID: id, Name: l.Name, Color: l.Color})
	}
	for _, l := range prev {
		// 删掉的标签从卡片上摘掉；但新库里已经有同名的标签时，卡片上的写法保持不动
		if key := strings.ToLower(l.Name); !kept[l.ID] && !names[key] {
			plan.removed[key] = true
		}
	}
	return plan, nil
}

// retag 按计划改写一张卡片的标签：被删的去掉、被改名的换成新名，再不区分大小写去重。
// 返回新标签和是否有变化。
func (p labelPlan) retag(tags []string) ([]string, bool) {
	out := make([]string, 0, len(tags))
	seen := make(map[string]bool, len(tags))
	for _, tag := range tags {
		key := strings.ToLower(tag)
		if p.removed[key] {
			continue
		}
		if to, ok := p.renamed[key]; ok {
			tag = to
		}
		if k := strings.ToLower(tag); !seen[k] {
			seen[k] = true
			out = append(out, tag)
		}
	}
	return out, !slices.Equal(out, tags)
}

// applyLabels 在事务里把新的标签库写到看板上，并把改名和删除同步到看板的每张卡片
// （卡片版本加一，旧页面上的修改会被乐观锁挡住）。标签库没有任何变化时返回 false。
func (u *unit) applyLabels(ctx context.Context, b *Board, in []LabelInput) (bool, error) {
	plan, err := planLabels(b.Labels, in)
	if err != nil {
		return false, err
	}
	if slices.Equal(plan.labels, b.Labels) {
		return false, nil
	}
	if len(plan.renamed) > 0 || len(plan.removed) > 0 {
		if err := u.retagCards(ctx, b.ID, plan); err != nil {
			return false, err
		}
	}
	b.Labels = plan.labels
	return true, nil
}

// retagCards 把标签计划应用到看板的全部卡片，只保存真的变了的卡片。
// 复杂度 O(卡片数 × 标签数)，一个看板通常是几百张卡片。
func (u *unit) retagCards(ctx context.Context, boardID int64, plan labelPlan) error {
	cards, err := u.card.ListByBoard(ctx, boardID)
	if err != nil {
		return wrapDB("读取卡片", err)
	}
	now := u.now()
	for _, card := range cards {
		tags, changed := plan.retag(card.Tags)
		if !changed {
			continue
		}
		card.Tags = tags
		card.Version++
		card.UpdatedAt = now
		if err := u.card.Save(ctx, card); err != nil {
			return wrapDB("保存卡片", err)
		}
	}
	return nil
}
