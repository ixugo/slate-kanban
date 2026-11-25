package kanban

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unicode"
)

const (
	// maxBoardNameLen 看板名称的最大字符数，和数据库列长度一致。
	maxBoardNameLen = 100
	// maxBoardDescLen 看板说明的最大字符数。
	maxBoardDescLen = 2000
	// maxProjectPathLen 项目路径的最大字符数，和数据库列长度一致。
	maxProjectPathLen = 512
	// minSlugLen、maxSlugLen 是看板标识的长度范围。标识会拼进每个任务的编号，所以要短。
	minSlugLen = 2
	maxSlugLen = 10
	// slugBaseLen 自动生成标识时，最多取名称里的几个字母或数字。
	slugBaseLen = 6
	// fallbackSlug 名称里取不出合格标识（比如纯中文名）时使用的标识。
	fallbackSlug = "BOARD"
	// maxSlugAttempts 自动生成的标识被占用时，最多换几次数字后缀。
	maxSlugAttempts = 20
)

// 起步模板的名称。
const (
	// TemplateDev 为开发工作准备的五列流程：收件箱、待办、进行中、审查、完成。
	TemplateDev = "dev"
	// TemplateSimple 三列流程：待办、进行中、完成。
	TemplateSimple = "simple"
	// TemplateEmpty 不创建任何列，由用户自己搭。
	TemplateEmpty = "empty"
)

// CreateBoardInput 是创建看板的入参。
type CreateBoardInput struct {
	Name              string `json:"name"`
	Slug              string `json:"slug"` // 可选，不填就按名称生成
	Description       string `json:"description"`
	ProjectPath       string `json:"project_path"`
	Template          string `json:"template"`            // dev、simple 或 empty，不填按 dev；有 project_template_id 时忽略
	Locale            string `json:"locale"`              // 起步列文案的语言，不认识的按 en
	ProjectTemplateID int64  `json:"project_template_id"` // 非 0 时使用对应的项目模板
}

// UpdateBoardInput 是修改看板的入参。指针字段为 nil 表示不修改，只更新请求里出现的字段。
// slug 不在其中：它已经拼进了所有任务的编号，创建后不允许修改。
type UpdateBoardInput struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	ProjectPath *string `json:"project_path"`
	// Tags 整体替换标签库：库里原有、入参里没有的算删除（同时从活跃任务上摘掉），
	// 主键相同的算修改文字或颜色，入参顺序就是显示顺序。只允许人修改。
	Tags *[]TagInput `json:"tags"`
	// Engineers 整体替换工程师名册：名册里原有、入参里没有的算解聘（同时从活跃任务上摘掉），
	// 主键相同的算修改，入参顺序就是显示顺序。只允许人修改。
	Engineers *[]EngineerInput `json:"engineers"`
}

// normalize 整理并校验创建入参：去掉首尾空白，slug 转大写，模板缺省为 dev。
// 它修改的是调用方传进来的副本，不会影响原始请求。
func (in *CreateBoardInput) normalize() error {
	in.Name = strings.TrimSpace(in.Name)
	in.Slug = strings.ToUpper(strings.TrimSpace(in.Slug))
	in.ProjectPath = strings.TrimSpace(in.ProjectPath)
	in.Template = strings.TrimSpace(in.Template)
	if in.Template == "" {
		in.Template = TemplateDev
	}
	if err := checkBoardFields(in.Name, in.Description, in.ProjectPath); err != nil {
		return err
	}
	if in.Slug != "" {
		if err := validateSlug(in.Slug); err != nil {
			return err
		}
	}
	if in.ProjectTemplateID < 0 {
		return invalidf("project_template_id 不合法")
	}
	if in.ProjectTemplateID > 0 {
		return nil
	}
	switch in.Template {
	case TemplateDev, TemplateSimple, TemplateEmpty:
		return nil
	}
	return invalidf("起步模板只能是 %s、%s 或 %s", TemplateDev, TemplateSimple, TemplateEmpty)
}

// normalize 整理并校验修改入参，只检查出现的字段。
func (in *UpdateBoardInput) normalize() error {
	if in.Name != nil {
		trimmed := strings.TrimSpace(*in.Name)
		in.Name = &trimmed
	}
	if in.ProjectPath != nil {
		trimmed := strings.TrimSpace(*in.ProjectPath)
		in.ProjectPath = &trimmed
	}
	if in.Name != nil {
		if err := checkLine("看板名称", *in.Name, 1, maxBoardNameLen); err != nil {
			return err
		}
	}
	if in.Description != nil {
		if err := checkText("看板说明", *in.Description, 0, maxBoardDescLen); err != nil {
			return err
		}
	}
	if in.Tags != nil {
		tags, err := normalizeTagInputs(*in.Tags)
		if err != nil {
			return err
		}
		in.Tags = &tags
	}
	if in.Engineers != nil {
		engineers, err := normalizeEngineerInputs(*in.Engineers)
		if err != nil {
			return err
		}
		in.Engineers = &engineers
	}
	if in.ProjectPath != nil {
		return validateProjectPath(*in.ProjectPath)
	}
	return nil
}

// apply 把入参里出现的字段写到看板上，返回真正发生变化的字段名（和 JSON 字段名一致）。
// 值和原来相同的字段不算变化，所以重复提交同样的修改不会产生事件。
func (in UpdateBoardInput) apply(b *Board) []string {
	var changed []string
	if in.Name != nil && *in.Name != b.Name {
		b.Name = *in.Name
		changed = append(changed, "name")
	}
	if in.Description != nil && *in.Description != b.Description {
		b.Description = *in.Description
		changed = append(changed, "description")
	}
	if in.ProjectPath != nil && *in.ProjectPath != b.ProjectPath {
		b.ProjectPath = *in.ProjectPath
		changed = append(changed, "project_path")
	}
	return changed
}

// checkBoardFields 校验创建时一定会出现的三个字段。
func checkBoardFields(name, description, projectPath string) error {
	if err := checkLine("看板名称", name, 1, maxBoardNameLen); err != nil {
		return err
	}
	if err := checkText("看板说明", description, 0, maxBoardDescLen); err != nil {
		return err
	}
	return validateProjectPath(projectPath)
}

// validateSlug 校验已经转成大写的看板标识：2 到 10 位，首位是大写字母，其余是大写字母或数字。
func validateSlug(s string) error {
	if len(s) < minSlugLen || len(s) > maxSlugLen {
		return invalidf("看板标识长度必须在 %d 到 %d 位之间", minSlugLen, maxSlugLen)
	}
	for i := range len(s) {
		upper := s[i] >= 'A' && s[i] <= 'Z'
		digit := s[i] >= '0' && s[i] <= '9'
		if i == 0 && !upper {
			return invalidf("看板标识必须以大写字母开头")
		}
		if !upper && !digit {
			return invalidf("看板标识只能包含大写字母和数字")
		}
	}
	return nil
}

// generateSlug 由看板名称生成标识。attempt 为 0 时返回基础标识；
// 大于 0 说明前一个已被占用，在末尾追加数字 attempt+1（2、3、4……），
// 必要时截短前面的部分，保证总长度不超过上限。
func generateSlug(name string, attempt int) string {
	base := slugBase(name)
	if attempt == 0 {
		return base
	}
	suffix := strconv.Itoa(attempt + 1)
	return base[:min(len(base), maxSlugLen-len(suffix))] + suffix
}

// slugBase 取名称里前 slugBaseLen 个 ASCII 字母或数字，转成大写。
// 结果不足两位或不是以字母开头时（比如纯中文名、数字开头），改用 fallbackSlug。
func slugBase(name string) string {
	var b strings.Builder
	for _, r := range name {
		if b.Len() >= slugBaseLen {
			break
		}
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(unicode.ToUpper(r))
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}
	s := b.String()
	if len(s) < minSlugLen || s[0] < 'A' || s[0] > 'Z' {
		return fallbackSlug
	}
	return s
}

// validateProjectPath 校验项目路径。允许为空；非空时必须是绝对路径，
// 并且不含空字符和控制字符。同时接受 Unix 路径、Windows 盘符路径和网络共享路径，
// 以兼容外部 agent 通过 MCP 写入其他机器上的项目路径。
func validateProjectPath(p string) error {
	if p == "" {
		return nil
	}
	if err := checkLine("项目路径", p, 1, maxProjectPathLen); err != nil {
		return err
	}
	if !isAbsolutePath(p) {
		return invalidf("项目路径必须是绝对路径")
	}
	return nil
}

// ValidateLocalProjectPath 检查网页填写的工作目录在服务端是否真实存在。
// REST 的预校验和保存共用它；MCP 仍然只校验跨平台路径格式。
func ValidateLocalProjectPath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	if err := validateProjectPath(path); err != nil {
		return err
	}
	if !filepath.IsAbs(path) {
		return invalidf("工作目录必须是服务端的绝对路径")
	}
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, os.ErrNotExist), errors.Is(err, syscall.ENOTDIR):
		return ErrProjectPathNotFound
	case err != nil:
		return ErrProjectPathCheckFailed
	case !info.IsDir():
		return ErrProjectPathNotFound
	default:
		return nil
	}
}

// isAbsolutePath 判断是不是绝对路径：以斜线开头，或盘符加冒号加斜线（反斜线），或双反斜线开头。
func isAbsolutePath(p string) bool {
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, `\\`) {
		return true
	}
	if len(p) < 3 || p[1] != ':' || (p[2] != '/' && p[2] != '\\') {
		return false
	}
	letter := p[0] | 0x20 // 转小写再比较
	return letter >= 'a' && letter <= 'z'
}
