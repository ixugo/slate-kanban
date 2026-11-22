package kanbandb

import (
	"context"

	"github.com/ixugo/goddd/pkg/orm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"slate-kanban/internal/core/kanban"
)

var _ kanban.ProjectTemplateStorer = ProjectTemplate{}

// ProjectTemplate 是项目模板的数据库实现。
type ProjectTemplate DB

// WithTx 返回绑定到事务的副本。
func (d ProjectTemplate) WithTx(tx orm.Tx) kanban.ProjectTemplateStorer {
	return ProjectTemplate{db: orm.GormDB(tx)}
}

// Create 新增一条模板。
func (d ProjectTemplate) Create(ctx context.Context, model *kanban.ProjectTemplate) error {
	return d.db.WithContext(ctx).Create(model).Error
}

// Save 只写入可覆盖的定义及时间，保留原有模板和项目 ID。
func (d ProjectTemplate) Save(ctx context.Context, model *kanban.ProjectTemplate) error {
	return d.db.WithContext(ctx).Model(model).UpdateColumns(map[string]any{
		"definition": model.Definition, "updated_at": model.UpdatedAt,
	}).Error
}

// Delete 按主键删除，幂等。
func (d ProjectTemplate) Delete(ctx context.Context, id int64) error {
	res := d.db.WithContext(ctx).Delete(&kanban.ProjectTemplate{}, id)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

// GetByID 按主键读取。
func (d ProjectTemplate) GetByID(ctx context.Context, id int64) (*kanban.ProjectTemplate, error) {
	var out kanban.ProjectTemplate
	if err := d.query(ctx).Where("project_templates.id = ?", id).Take(&out).Error; err != nil {
		return nil, err
	}
	return &out, nil
}

// LockByName 按名称读取并锁定模板，区分大小写。
func (d ProjectTemplate) LockByName(ctx context.Context, name string) (*kanban.ProjectTemplate, error) {
	var out kanban.ProjectTemplate
	if err := d.query(ctx).
		Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: clause.CurrentTable}}).
		Where("project_templates.name = ?", name).Take(&out).Error; err != nil {
		return nil, err
	}
	return &out, nil
}

// List 返回全部模板，按名称升序。
func (d ProjectTemplate) List(ctx context.Context) ([]*kanban.ProjectTemplate, error) {
	items := make([]*kanban.ProjectTemplate, 0)
	err := d.query(ctx).Order("project_templates.name ASC").Find(&items).Error
	return items, err
}

// query 通过模板项目标记限定查询，避免把普通项目或不完整关联当作可用模板。
func (d ProjectTemplate) query(ctx context.Context) *gorm.DB {
	return d.db.WithContext(ctx).Model(new(kanban.ProjectTemplate)).Select("project_templates.*").
		Joins("JOIN boards ON boards.id = project_templates.board_id AND boards.is_template = ?", true)
}
