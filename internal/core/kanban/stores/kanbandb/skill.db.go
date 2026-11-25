package kanbandb

import (
	"context"

	"gorm.io/gorm"

	"slate-kanban/internal/core/kanban"
)

var _ kanban.SkillStorer = Skill{}

// Skill 是服务级自定义 Skill 的数据库实现。
type Skill DB

func (d Skill) ListMetadata(ctx context.Context) ([]*kanban.Skill, error) {
	items := make([]*kanban.Skill, 0)
	err := d.metadataQuery(ctx).Order("name ASC").Find(&items).Error
	return items, err
}

func (d Skill) GetMetadata(ctx context.Context, name string) (*kanban.Skill, error) {
	var out kanban.Skill
	if err := d.metadataQuery(ctx).Where("name = ?", name).Take(&out).Error; err != nil {
		return nil, err
	}
	return &out, nil
}

func (d Skill) metadataQuery(ctx context.Context) *gorm.DB {
	return d.db.WithContext(ctx).Model(new(kanban.Skill)).Select("name", "description", "digest", "size", "updated_at")
}

func (d Skill) Get(ctx context.Context, name string) (*kanban.Skill, error) {
	var out kanban.Skill
	if err := d.db.WithContext(ctx).Where("name = ?", name).Take(&out).Error; err != nil {
		return nil, err
	}
	return &out, nil
}

func (d Skill) Create(ctx context.Context, model *kanban.Skill) error {
	return d.db.WithContext(ctx).Create(model).Error
}

func (d Skill) Update(ctx context.Context, name string, model *kanban.Skill) error {
	res := d.db.WithContext(ctx).Model(new(kanban.Skill)).Where("name = ?", name).UpdateColumns(map[string]any{
		"name": model.Name, "description": model.Description, "markdown": model.Markdown,
		"digest": model.Digest, "size": model.Size, "updated_at": model.UpdatedAt,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (d Skill) Delete(ctx context.Context, name string) error {
	return d.db.WithContext(ctx).Where("name = ?", name).Delete(new(kanban.Skill)).Error
}
