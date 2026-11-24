package kanban

import (
	"cmp"
	"context"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/ixugo/goddd/pkg/orm"

	"slate-kanban/skills"
)

const (
	maxSkillNameLen        = 64
	maxSkillDescriptionLen = 1024
	// MaxSkillBodyBytes 留出 JSON 编码及元数据的空间，适配现有 HTTP 请求体限制。
	MaxSkillBodyBytes = 262144 // 256 KiB
)

var skillNamePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidSkillName 按 Agent Skills 名称规范校验，也用于解析单文件 URI。
func ValidSkillName(name string) bool {
	return len(name) <= maxSkillNameLen && skillNamePattern.MatchString(name)
}

func validateSkillInput(in SkillInput) error {
	if !ValidSkillName(in.Name) {
		return invalidf("Skill 名称必须为 1-64 位小写字母、数字或单个连字符分隔的词")
	}
	if _, reserved := skills.Builtin(in.Name); reserved {
		return ErrSkillNameTaken
	}
	if strings.TrimSpace(in.Description) == "" {
		return invalidf("Skill 简介不能为空")
	}
	if err := checkLine("Skill 简介", in.Description, 1, maxSkillDescriptionLen); err != nil {
		return err
	}
	if strings.TrimSpace(in.Body) == "" {
		return invalidf("Skill 正文不能为空")
	}
	if len(in.Body) > MaxSkillBodyBytes {
		return invalidf("Skill 正文不能超过 %d 字节", MaxSkillBodyBytes)
	}
	return checkText("Skill 正文", in.Body, 1, MaxSkillBodyBytes)
}

// SkillStorer 是自定义 Skill 数据读写接口。按名称使用唯一主键，修改是单条原子更新。
type SkillStorer interface {
	// ListMetadata 不读取 Markdown 正文。
	ListMetadata(context.Context) ([]*Skill, error)
	GetMetadata(context.Context, string) (*Skill, error)
	Get(context.Context, string) (*Skill, error)
	Create(context.Context, *Skill) error
	Update(context.Context, string, *Skill) error
	Delete(context.Context, string) error
}

// ListSkills 返回当前服务内置与自定义 Skill 的元数据，按名称排序。
func (c Core) ListSkills(ctx context.Context) ([]SkillSummary, error) {
	rows, err := c.store.Skill().ListMetadata(ctx)
	if err != nil {
		return nil, wrapDB("读取 Skill 列表", err)
	}
	builtin := skills.Builtins()
	out := make([]SkillSummary, 0, len(builtin)+len(rows))
	for _, doc := range builtin {
		out = append(out, skillDetail(doc, true, time.Time{}).SkillSummary)
	}
	for _, row := range rows {
		// 保留名始终由程序提供：即使升级后新增同名内置 Skill，也不发布冲突项。
		if _, reserved := skills.Builtin(row.Name); reserved {
			continue
		}
		out = append(out, row.summary())
	}
	slices.SortFunc(out, func(a, b SkillSummary) int { return cmp.Compare(a.Name, b.Name) })
	return out, nil
}

// GetSkillMetadata 读取单项 manifest，不读取数据库正文。
func (c Core) GetSkillMetadata(ctx context.Context, name string) (*SkillSummary, error) {
	if !ValidSkillName(name) {
		return nil, ErrSkillNotFound
	}
	if doc, ok := skills.Builtin(name); ok {
		return new(skillDetail(doc, true, time.Time{}).SkillSummary), nil
	}
	row, err := c.store.Skill().GetMetadata(ctx, name)
	if err != nil {
		return nil, mapNotFound(err, ErrSkillNotFound, "读取 Skill 元数据")
	}
	return new(row.summary()), nil
}

// GetSkill 根据名称读取当前 Skill 正文。内置名称优先，数据库不能覆盖它。
func (c Core) GetSkill(ctx context.Context, name string) (*SkillDetail, error) {
	if !ValidSkillName(name) {
		return nil, ErrSkillNotFound
	}
	if doc, ok := skills.Builtin(name); ok {
		return new(skillDetail(doc, true, time.Time{})), nil
	}
	row, err := c.store.Skill().Get(ctx, name)
	if err != nil {
		return nil, mapNotFound(err, ErrSkillNotFound, "读取 Skill")
	}
	return new(row.detail()), nil
}

// CreateSkill 新增一个自定义 Skill，名称与全部内置、自定义 Skill 独立且唯一。
func (c Core) CreateSkill(ctx context.Context, actor Actor, in SkillInput) (*SkillDetail, error) {
	if err := requireHuman(actor); err != nil {
		return nil, err
	}
	if err := validateSkillInput(in); err != nil {
		return nil, err
	}
	row := newSkill(in, c.now())
	if err := c.store.Skill().Create(ctx, row); err != nil {
		if orm.IsDuplicatedKey(err) {
			return nil, ErrSkillNameTaken
		}
		return nil, wrapDB("创建 Skill", err)
	}
	return new(row.detail()), nil
}

// UpdateSkill 完整替换自定义 Skill，可修改名称。新 URI 在修改成功后立即生效。
func (c Core) UpdateSkill(ctx context.Context, actor Actor, name string, in SkillInput) (*SkillDetail, error) {
	if err := requireHuman(actor); err != nil {
		return nil, err
	}
	if _, builtin := skills.Builtin(name); builtin {
		return nil, invalidf("内置 Skill 不可编辑")
	}
	if !ValidSkillName(name) {
		return nil, ErrSkillNotFound
	}
	if err := validateSkillInput(in); err != nil {
		return nil, err
	}
	row := newSkill(in, c.now())
	if err := c.store.Skill().Update(ctx, name, row); err != nil {
		if orm.IsDuplicatedKey(err) {
			return nil, ErrSkillNameTaken
		}
		return nil, mapNotFound(err, ErrSkillNotFound, "修改 Skill")
	}
	return new(row.detail()), nil
}

// DeleteSkill 删除自定义 Skill，重复删除幂等。
func (c Core) DeleteSkill(ctx context.Context, actor Actor, name string) error {
	if err := requireHuman(actor); err != nil {
		return err
	}
	if _, builtin := skills.Builtin(name); builtin {
		return invalidf("内置 Skill 不可删除")
	}
	if !ValidSkillName(name) {
		return ErrSkillNotFound
	}
	if err := c.store.Skill().Delete(ctx, name); err != nil {
		return wrapDB("删除 Skill", err)
	}
	return nil
}
