package kanban

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/reason"
)

// ListProjectTemplates 返回全部自定义项目模板摘要，人和 agent 都可以读。
func (c Core) ListProjectTemplates(ctx context.Context, actor Actor) ([]ProjectTemplateSummary, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	items, err := c.store.ProjectTemplate().List(ctx)
	if err != nil {
		return nil, wrapDB("读取项目模板列表", err)
	}
	out := make([]ProjectTemplateSummary, len(items))
	for i, t := range items {
		out[i] = ProjectTemplateSummary{ID: t.ID, Name: t.Name, UpdatedAt: t.UpdatedAt}
	}
	return out, nil
}

// ExportProjectTemplateFromBoard 把看板的标签、工程师和列导出为模板 JSON 结构，默认名为看板名称。
func (c Core) ExportProjectTemplateFromBoard(ctx context.Context, actor Actor, boardID int64) (*ProjectTemplateFile, error) {
	if err := requireHuman(actor); err != nil {
		return nil, err
	}
	detail, err := c.GetBoard(ctx, boardID)
	if err != nil {
		return nil, err
	}
	cols := slices.Clone(detail.Columns)
	slices.SortFunc(cols, func(a, b *Column) int {
		if a.Sort != b.Sort {
			if a.Sort < b.Sort {
				return -1
			}
			return 1
		}
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	idToIndex := make(map[int64]int, len(cols))
	for i, col := range cols {
		idToIndex[col.ID] = i
	}
	outCols := make([]ProjectTemplateColumn, len(cols))
	for i, col := range cols {
		next := -1
		if col.NextColumnID != 0 {
			if idx, ok := idToIndex[col.NextColumnID]; ok {
				next = idx
			}
		}
		outCols[i] = ProjectTemplateColumn{
			Name: col.Name, Instruction: col.Instruction, IsDone: col.IsDone, NextColumn: next,
		}
	}
	tags := make([]ProjectTemplateTag, len(detail.Tags))
	for i, t := range detail.Tags {
		tags[i] = ProjectTemplateTag{Text: t.Text, Color: t.Color}
	}
	engineers := make([]ProjectTemplateEngineer, len(detail.Engineers))
	for i, e := range detail.Engineers {
		engineers[i] = ProjectTemplateEngineer{Name: e.Name, Kind: e.Kind, Prompt: e.Prompt, Avatar: e.Avatar}
	}
	return &ProjectTemplateFile{
		Format: ProjectTemplateFormat, Version: ProjectTemplateVersion, Name: detail.Name,
		Tags: tags, Engineers: engineers, Columns: outCols,
	}, nil
}

// ImportProjectTemplate 把 JSON 导入为自定义模板；replace 为真时同名先删后建，在同一事务里完成。
func (c Core) ImportProjectTemplate(ctx context.Context, actor Actor, raw []byte, replace bool) (*ProjectTemplateDetail, error) {
	if err := requireHuman(actor); err != nil {
		return nil, err
	}
	file, err := ParseProjectTemplateJSON(raw)
	if err != nil {
		return nil, err
	}
	body := projectTemplateBodyFromFile(file)
	definition, err := json.Marshal(body)
	if err != nil {
		return nil, reason.ErrJSON.WithCause(err).Withf("编码模板定义")
	}
	var out *ProjectTemplateDetail
	err = c.run(ctx, 0, func(u *unit) error {
		existing, err := u.projectTemplate.GetByName(ctx, file.Name)
		if err != nil && !orm.IsErrRecordNotFound(err) {
			return wrapDB("查询模板", err)
		}
		if existing != nil {
			if !replace {
				return ErrProjectTemplateNameTaken
			}
			if err := u.projectTemplate.Delete(ctx, existing.ID); err != nil {
				return wrapDB("删除旧模板", err)
			}
		}
		now := u.now()
		row := &ProjectTemplate{Name: file.Name, Definition: string(definition), CreatedAt: now, UpdatedAt: now}
		if err := u.projectTemplate.Create(ctx, row); err != nil {
			if orm.IsDuplicatedKey(err) {
				return ErrProjectTemplateNameTaken
			}
			return wrapDB("创建模板", err)
		}
		out = &ProjectTemplateDetail{
			ID: row.ID, Name: row.Name, UpdatedAt: row.UpdatedAt, ProjectTemplateBody: body,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteProjectTemplate 删除自定义模板，只有人能操作；不存在也返回成功。
func (c Core) DeleteProjectTemplate(ctx context.Context, actor Actor, id int64) error {
	if err := requireHuman(actor); err != nil {
		return err
	}
	return c.run(ctx, 0, func(u *unit) error {
		return u.projectTemplate.Delete(ctx, id)
	})
}

// loadProjectTemplateBody 按主键读出模板并解析 definition 字段。
func (c Core) loadProjectTemplateBody(ctx context.Context, id int64) (*ProjectTemplate, *ProjectTemplateBody, error) {
	row, err := c.store.ProjectTemplate().GetByID(ctx, id)
	if err != nil {
		return nil, nil, mapNotFound(err, ErrProjectTemplateNotFound, "读取项目模板")
	}
	var body ProjectTemplateBody
	if err := json.Unmarshal([]byte(row.Definition), &body); err != nil {
		return nil, nil, reason.ErrJSON.WithCause(err).Withf("解析模板定义")
	}
	return row, &body, nil
}

// projectTemplateBodyFromFile 从信封里抽出存库用的主体。
func projectTemplateBodyFromFile(file *ProjectTemplateFile) ProjectTemplateBody {
	return ProjectTemplateBody{Tags: file.Tags, Engineers: file.Engineers, Columns: file.Columns}
}

// materializeProjectTemplate 按模板主体为新看板创建标签、工程师和列，不含自动化规则。
func (u *unit) materializeProjectTemplate(ctx context.Context, boardID int64, body *ProjectTemplateBody) ([]*Tag, []*Engineer, []*Column, error) {
	now := u.now()
	tags := make([]*Tag, len(body.Tags))
	for i, t := range body.Tags {
		tags[i] = &Tag{
			BoardID: boardID, Text: t.Text, Color: t.Color, Sort: int64(i + 1), CreatedAt: now, UpdatedAt: now,
		}
		if err := u.tag.Create(ctx, tags[i]); err != nil {
			return nil, nil, nil, wrapDB("从模板创建标签", err)
		}
	}
	engineers := make([]*Engineer, len(body.Engineers))
	for i, e := range body.Engineers {
		avatar := e.Avatar
		if avatar == "" {
			avatar = newAvatarSeed()
		}
		engineers[i] = &Engineer{
			BoardID: boardID, Name: e.Name, Kind: e.Kind, Prompt: e.Prompt, Avatar: avatar,
			Sort: int64(i + 1), CreatedAt: now, UpdatedAt: now,
		}
		if err := u.engineer.Create(ctx, engineers[i]); err != nil {
			return nil, nil, nil, wrapDB("从模板创建工程师", err)
		}
	}
	cols := make([]*Column, len(body.Columns))
	for i, c := range body.Columns {
		cols[i] = &Column{
			BoardID: boardID, Name: c.Name, Instruction: c.Instruction, IsDone: c.IsDone,
			Sort: int64(i+1) * columnSortStep, CreatedAt: now, UpdatedAt: now,
		}
		if err := u.column.Create(ctx, cols[i]); err != nil {
			return nil, nil, nil, wrapDB("从模板创建列", err)
		}
	}
	for i, c := range body.Columns {
		if c.NextColumn < 0 {
			continue
		}
		cols[i].NextColumnID = cols[c.NextColumn].ID
		if err := u.column.Save(ctx, cols[i]); err != nil {
			return nil, nil, nil, wrapDB("从模板回填下一列", err)
		}
	}
	return tags, engineers, cols, nil
}
