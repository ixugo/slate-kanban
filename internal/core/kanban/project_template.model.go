package kanban

import (
	"crypto/rand"
	"time"
)

const (
	// ProjectTemplateFormat 是项目模板 JSON 信封里的 format 固定值。
	ProjectTemplateFormat = "slate-project-template"
	// ProjectTemplateVersion 是当前支持的 schema 版本号。
	ProjectTemplateVersion = 1
	// MaxProjectTemplateImportBytes 是导入模板 JSON 允许的最大字节数。
	MaxProjectTemplateImportBytes = 512 * 1024
	// maxProjectTemplateNameLen 自定义模板名称的最大字符数。
	maxProjectTemplateNameLen = maxBoardNameLen
)

// ProjectTemplate 是模板项目的元数据，保留导入定义及对外使用的稳定模板 ID。
type ProjectTemplate struct {
	BoardID    int64     `gorm:"column:board_id;notNull;default:0;uniqueIndex:uq_project_templates_board_id,where:board_id > 0;comment:对应的模板项目 ID" json:"-"`
	ID         int64     `gorm:"primaryKey" json:"id"`
	Name       string    `gorm:"column:name;size:100;notNull;default:'';uniqueIndex;comment:模板名称，区分大小写唯一" json:"name"`
	Definition string    `gorm:"column:definition;type:text;notNull;default:'';comment:标签、工程师、列的 JSON" json:"-"`
	CreatedAt  time.Time `gorm:"column:created_at;notNull;default:CURRENT_TIMESTAMP;comment:创建时间" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;notNull;default:CURRENT_TIMESTAMP;comment:更新时间" json:"updated_at"`
}

// TableName 指定表名。
func (*ProjectTemplate) TableName() string {
	return "project_templates"
}

// NewTemplateBoard 为模板构造单独的项目记录。项目标识校验拒绝 ~，因此不会占用用户的标识。
func (t ProjectTemplate) NewTemplateBoard() *Board {
	return &Board{
		Name: t.Name, Slug: "~" + rand.Text()[:maxSlugLen-1], IsTemplate: true,
		CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

// ProjectTemplateSummary 是列表项，不含完整定义。
type ProjectTemplateSummary struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updated_at"`
	Builtin   bool      `json:"builtin"`
}

// ProjectTemplateFile 是导入导出共用的 JSON 信封（不含数据库 id）。
type ProjectTemplateFile struct {
	Format    string                    `json:"format"`
	Version   int                       `json:"version"`
	Name      string                    `json:"name"`
	Tags      []ProjectTemplateTag      `json:"tags"`
	Engineers []ProjectTemplateEngineer `json:"engineers"`
	Columns   []ProjectTemplateColumn   `json:"columns"`
}

// ProjectTemplateBody 是存进数据库 definition 字段的主体（不含信封元数据）。
type ProjectTemplateBody struct {
	Tags      []ProjectTemplateTag      `json:"tags"`
	Engineers []ProjectTemplateEngineer `json:"engineers"`
	Columns   []ProjectTemplateColumn   `json:"columns"`
}

// ProjectTemplateTag 是模板里的一个标签项。
type ProjectTemplateTag struct {
	Text  string `json:"text"`
	Color string `json:"color"`
}

// ProjectTemplateEngineer 是模板里的一位工程师。
type ProjectTemplateEngineer struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Prompt string `json:"prompt"`
	Avatar string `json:"avatar,omitempty"`
}

// ProjectTemplateColumn 是模板里的一列；next_column 是列在下标上的引用，-1 表示没有下一列。
type ProjectTemplateColumn struct {
	Name        string `json:"name"`
	Instruction string `json:"instruction"`
	IsDone      bool   `json:"is_done"`
	NextColumn  int    `json:"next_column"`
}

// ProjectTemplateDetail 是导入成功或读取定义时的返回，带解析后的结构。
type ProjectTemplateDetail struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updated_at"`
	ProjectTemplateBody
}
