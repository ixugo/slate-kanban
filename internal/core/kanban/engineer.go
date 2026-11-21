package kanban

import (
	"context"
	"math/rand/v2"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ixugo/goddd/pkg/orm"
)

// EngineerStorer 是工程师的数据持久化接口。
type EngineerStorer interface {
	// WithTx 返回绑定到事务的副本，事务内的所有读写都必须用它。
	WithTx(orm.Tx) EngineerStorer
	// Create 新增一位工程师。
	Create(context.Context, *Engineer) error
	// Save 把整个工程师写回数据库。
	Save(context.Context, *Engineer) error
	// Delete 按主键删除，幂等。
	Delete(context.Context, *Engineer) error
	// ListByBoard 返回看板的全部工程师，按 Sort、主键升序。
	ListByBoard(context.Context, int64) ([]*Engineer, error)
	// DeleteByBoard 删除看板的全部工程师，用于删除看板。
	DeleteByBoard(context.Context, int64) error
}

const (
	// maxBoardEngineers 一个看板的名册最多几位工程师，也就是最多几个并发名额。
	maxBoardEngineers = 20
	// maxEngineerNameLen 工程师名称的最大字符数，和数据库列长度一致。
	maxEngineerNameLen = 30
	// maxEngineerPromptLen 角色提示词的最大字符数。提示词会进入调度 agent 的上下文，所以不能无限长。
	maxEngineerPromptLen = 2000
	// maxEngineerAvatarLen 头像种子的最大字符数，和数据库列长度一致。
	maxEngineerAvatarLen = 32
)

// avatarSeedPattern 限定头像种子只能是字母、数字、下划线和连字符，防止塞进奇怪的内容。
var avatarSeedPattern = regexp.MustCompile(`^[A-Za-z0-9_-]*$`)

// newAvatarSeed 生成一个随机的头像种子（8 位以内的 36 进制数字串）。
func newAvatarSeed() string {
	return strconv.FormatUint(rand.Uint64N(1<<41), 36)
}

// validEngineerKind 判断类型是不是全栈、前端、后端、UI 或测试。
func validEngineerKind(kind string) bool {
	switch kind {
	case EngineerDev, EngineerFrontend, EngineerBackend, EngineerUI, EngineerReview:
		return true
	}
	return false
}

// normalizeEngineerInputs 整理并校验名册入参：名称按单行文本校验且不区分大小写不重复，
// 类型必须合法，提示词不能为空，主键不能为负也不能重复，总数不超过上限。
func normalizeEngineerInputs(in []EngineerInput) ([]EngineerInput, error) {
	if len(in) > maxBoardEngineers {
		return nil, invalidf("一个看板最多招募 %d 位工程师", maxBoardEngineers)
	}
	out := make([]EngineerInput, 0, len(in))
	ids := make(map[int64]bool, len(in))
	names := make(map[string]bool, len(in))
	for _, e := range in {
		e.Name = strings.TrimSpace(e.Name)
		e.Prompt = strings.TrimSpace(e.Prompt)
		if err := checkLine("工程师名称", e.Name, 1, maxEngineerNameLen); err != nil {
			return nil, err
		}
		if err := checkText("角色提示词", e.Prompt, 1, maxEngineerPromptLen); err != nil {
			return nil, err
		}
		e.Avatar = strings.TrimSpace(e.Avatar)
		if len(e.Avatar) > maxEngineerAvatarLen || !avatarSeedPattern.MatchString(e.Avatar) {
			return nil, invalidf("头像种子只能是不超过 %d 位的字母、数字、下划线或连字符", maxEngineerAvatarLen)
		}
		if !validEngineerKind(e.Kind) {
			return nil, invalidf("工程师类型只能是 %s、%s、%s、%s 或 %s", EngineerDev, EngineerFrontend, EngineerBackend, EngineerUI, EngineerReview)
		}
		if e.ID < 0 || (e.ID != 0 && ids[e.ID]) {
			return nil, invalidf("工程师编号不合法或重复")
		}
		ids[e.ID] = true
		key := strings.ToLower(e.Name)
		if names[key] {
			return nil, invalidf("工程师名称重复：%s", e.Name)
		}
		names[key] = true
		out = append(out, e)
	}
	return out, nil
}

// applyEngineers 在事务里把新的名册写到看板上：主键在名册里的算修改，其余算招募（不信任调用方自带的主键），
// 名册里原有、入参里没有的算解聘，并把它从所有卡片上摘掉；入参顺序就是新的显示顺序。
// 名册没有任何变化时返回 false，也不写库。
func (u *unit) applyEngineers(ctx context.Context, b *Board, in []EngineerInput) (bool, error) {
	prev, err := u.engineer.ListByBoard(ctx, b.ID)
	if err != nil {
		return false, wrapDB("读取工程师名册", err)
	}
	known := make(map[int64]*Engineer, len(prev))
	for _, e := range prev {
		known[e.ID] = e
	}
	now := u.now()
	next := make([]*Engineer, 0, len(in))
	kept := make(map[int64]bool, len(in))
	changed := false
	for i, item := range in {
		sortNo := int64(i + 1)
		e, exists := known[item.ID]
		if exists {
			kept[e.ID] = true
		}
		switch {
		case !exists:
			if item.Avatar == "" {
				item.Avatar = newAvatarSeed()
			}
			e = &Engineer{BoardID: b.ID, Name: item.Name, Kind: item.Kind, Prompt: item.Prompt, Avatar: item.Avatar, Sort: sortNo, CreatedAt: now, UpdatedAt: now}
			if err := u.engineer.Create(ctx, e); err != nil {
				return false, wrapDB("招募工程师", err)
			}
			changed = true
		case e.Name != item.Name || e.Kind != item.Kind || e.Prompt != item.Prompt || e.Sort != sortNo || (item.Avatar != "" && e.Avatar != item.Avatar):
			e.Name, e.Kind, e.Prompt, e.Sort, e.UpdatedAt = item.Name, item.Kind, item.Prompt, sortNo, now
			if item.Avatar != "" {
				e.Avatar = item.Avatar
			}
			if err := u.engineer.Save(ctx, e); err != nil {
				return false, wrapDB("保存工程师", err)
			}
			changed = true
		}
		next = append(next, e)
	}
	removed, err := u.removeEngineers(ctx, b.ID, prev, kept)
	if err != nil {
		return false, err
	}
	b.Engineers = next
	return changed || removed, nil
}

// removeEngineers 解聘 prev 里不在 kept 中的工程师，并把他们从看板的卡片上摘掉，返回有没有解聘。
func (u *unit) removeEngineers(ctx context.Context, boardID int64, prev []*Engineer, kept map[int64]bool) (bool, error) {
	gone := make(map[int64]bool)
	for _, e := range prev {
		if kept[e.ID] {
			continue
		}
		if err := u.engineer.Delete(ctx, e); err != nil {
			return false, wrapDB("解聘工程师", err)
		}
		gone[e.ID] = true
	}
	if len(gone) == 0 {
		return false, nil
	}
	return true, u.unassignCards(ctx, boardID, gone)
}

// unassignCards 把被解聘工程师的绑定和占用从看板的全部卡片上清掉，只保存真的变了的卡片
// （版本加一，旧页面上的修改会被乐观锁挡住）。复杂度 O(卡片数)，一个看板通常是几百张卡片。
func (u *unit) unassignCards(ctx context.Context, boardID int64, gone map[int64]bool) error {
	cards, err := u.card.ListByBoard(ctx, boardID)
	if err != nil {
		return wrapDB("读取任务", err)
	}
	now := u.now()
	for _, card := range cards {
		if !gone[card.EngineerID] && !gone[card.ClaimEngineerID] {
			continue
		}
		if gone[card.EngineerID] {
			card.EngineerID = 0
		}
		if gone[card.ClaimEngineerID] {
			card.ClaimEngineerID = 0
		}
		card.Version++
		card.UpdatedAt = now
		if err := u.card.Save(ctx, card); err != nil {
			return wrapDB("保存任务", err)
		}
	}
	return nil
}

// seedEngineers 按模板为新看板招募起步的工程师：研发模板 2 个全栈加 1 个测试，
// 简单模板只有 2 个全栈（没有审查列），empty 模板没有。之后都是看板自己的数据，可以随意改。
func (u *unit) seedEngineers(ctx context.Context, boardID int64, template string) ([]*Engineer, error) {
	texts := seedEngineerTexts
	now := u.now()
	var out []*Engineer
	for i, seed := range engineerSeeds[template] {
		e := &Engineer{
			BoardID: boardID, Name: texts[seed].Name, Kind: engineerSeedKinds[seed], Prompt: texts[seed].Prompt, Avatar: newAvatarSeed(),
			Sort: int64(i + 1), CreatedAt: now, UpdatedAt: now,
		}
		if err := u.engineer.Create(ctx, e); err != nil {
			return nil, wrapDB("招募起步的工程师", err)
		}
		out = append(out, e)
	}
	return out, nil
}

// loadEngineers 把看板的工程师名册读出来放到 b.Engineers 上。
func (c Core) loadEngineers(ctx context.Context, b *Board) error {
	items, err := c.store.Engineer().ListByBoard(ctx, b.ID)
	if err != nil {
		return wrapDB("读取工程师名册", err)
	}
	b.Engineers = items
	b.Normalize()
	return nil
}

// checkEngineerID 确认工程师编号是这个看板名册里的研发类工程师（全栈、前端、后端、UI），0 表示不绑定。
// 卡片上的绑定只用于研发，所以不接受测试。
func (u *unit) checkEngineerID(ctx context.Context, boardID, id int64) error {
	if id == 0 {
		return nil
	}
	items, err := u.engineer.ListByBoard(ctx, boardID)
	if err != nil {
		return wrapDB("读取工程师名册", err)
	}
	i := slices.IndexFunc(items, func(e *Engineer) bool { return e.ID == id })
	if i < 0 {
		return invalidf("工程师 %d 不存在，或者不属于这个看板", id)
	}
	if stageOf(items[i].Kind) != EngineerDev {
		return invalidf("任务只能绑定研发类工程师（全栈、前端、后端、UI），%s 是测试", items[i].Name)
	}
	return nil
}
