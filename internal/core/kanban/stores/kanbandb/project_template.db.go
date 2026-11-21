package kanbandb

import (
	"context"

	"github.com/ixugo/goddd/pkg/orm"

	"slate-kanban/internal/core/kanban"
)

var _ kanban.ProjectTemplateStorer = ProjectTemplate{}

// ProjectTemplate 是自定义项目模板的数据库实现。
type ProjectTemplate DB

// WithTx 返回绑定到事务的副本。
func (d ProjectTemplate) WithTx(tx orm.Tx) kanban.ProjectTemplateStorer {
	return ProjectTemplate{db: orm.GormDB(tx)}
}

// Create 新增一条模板。
func (d ProjectTemplate) Create(ctx context.Context, model *kanban.ProjectTemplate) error {
	return d.db.WithContext(ctx).Create(model).Error
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
	if err := d.db.WithContext(ctx).Where("id = ?", id).Take(&out).Error; err != nil {
		return nil, err
	}
	return &out, nil
}

// GetByName 按名称读取，区分大小写。
func (d ProjectTemplate) GetByName(ctx context.Context, name string) (*kanban.ProjectTemplate, error) {
	var out kanban.ProjectTemplate
	if err := d.db.WithContext(ctx).Where("name = ?", name).Take(&out).Error; err != nil {
		return nil, err
	}
	return &out, nil
}

// List 返回全部模板，按名称升序。
func (d ProjectTemplate) List(ctx context.Context) ([]*kanban.ProjectTemplate, error) {
	items := make([]*kanban.ProjectTemplate, 0)
	err := d.db.WithContext(ctx).Order("name ASC").Find(&items).Error
	return items, err
}
