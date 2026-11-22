package kanbandb

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"slate-kanban/internal/core/kanban"
)

// migrateProjectTemplates 保留旧版自定义模板 ID，关联对应的模板项目，并创建保留的内置研发模板。
func (d DB) migrateProjectTemplates() error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		builtin := &kanban.ProjectTemplate{Name: kanban.TemplateDev}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "name"}}, DoNothing: true}).Create(builtin).Error; err != nil {
			return fmt.Errorf("写入内置研发模板: %w", err)
		}
		var pending []*kanban.ProjectTemplate
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("board_id = 0").Find(&pending).Error; err != nil {
			return fmt.Errorf("读取未关联项目的模板: %w", err)
		}
		for _, template := range pending {
			board := template.NewTemplateBoard()
			if err := tx.Create(board).Error; err != nil {
				return fmt.Errorf("创建模板项目 %d: %w", template.ID, err)
			}
			if err := tx.Model(template).UpdateColumn("board_id", board.ID).Error; err != nil {
				return fmt.Errorf("关联模板项目 %d: %w", template.ID, err)
			}
		}
		return nil
	})
}
