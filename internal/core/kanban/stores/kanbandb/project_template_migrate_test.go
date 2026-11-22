package kanbandb_test

import (
	"testing"
	"time"

	"github.com/ixugo/goddd/pkg/orm"
	"gorm.io/gorm"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/kanbantest"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
)

// legacyProjectTemplateRow 对应加入模板项目关联前的自定义模板表结构。
type legacyProjectTemplateRow struct {
	ID         int64     `gorm:"primaryKey"`
	Name       string    `gorm:"column:name;size:100;notNull;default:'';uniqueIndex"`
	Definition string    `gorm:"column:definition;type:text;notNull;default:''"`
	CreatedAt  time.Time `gorm:"column:created_at;notNull;default:CURRENT_TIMESTAMP"`
	UpdatedAt  time.Time `gorm:"column:updated_at;notNull;default:CURRENT_TIMESTAMP"`
}

func (legacyProjectTemplateRow) TableName() string { return "project_templates" }

func TestAutoMigrateTemplateProjectsPreservesLegacyTemplate(t *testing.T) {
	kanbantest.ForEachRawBackend(t, func(t *testing.T, raw *gorm.DB) {
		if err := raw.AutoMigrate(new(legacyProjectTemplateRow)); err != nil {
			t.Fatal(err)
		}
		const definition = `{"columns":[{"name":"旧模板列","next_column":-1}]}`
		legacy := &legacyProjectTemplateRow{Name: "旧版模板", Definition: definition}
		if err := raw.Create(legacy).Error; err != nil {
			t.Fatal(err)
		}

		ctx := t.Context()
		store := kanbandb.NewDB(raw).AutoMigrate(true)
		migrated, err := store.ProjectTemplate().GetByID(ctx, legacy.ID)
		if err != nil || migrated.Name != legacy.Name || migrated.Definition != definition || migrated.BoardID == 0 {
			t.Fatalf("旧模板应保留 ID、定义并关联模板项目: %+v %v", migrated, err)
		}
		var marked kanban.Board
		if err := raw.Take(&marked, migrated.BoardID).Error; err != nil || !marked.IsTemplate {
			t.Fatalf("模板应对应带标记的项目: %+v %v", marked, err)
		}
		if _, err := store.Board().GetByID(ctx, marked.ID); !orm.IsErrRecordNotFound(err) {
			t.Fatalf("普通项目详情不应读取模板项目: %v", err)
		}
		boards, err := store.Board().List(ctx)
		if err != nil || len(boards) != 0 {
			t.Fatalf("普通项目列表不应读取模板项目: %v %+v", err, boards)
		}

		store.AutoMigrate(true)
		items, err := store.ProjectTemplate().List(ctx)
		if err != nil || len(items) != 2 || items[1].ID != legacy.ID || items[1].BoardID != marked.ID {
			t.Fatalf("再次迁移不应重复创建或变更关联: %v %+v", err, items)
		}
	})
}
