package kanban

import (
	"context"
	"slices"
)

type eventDependencyRef struct {
	ID     int64 `json:"id"`
	Number int64 `json:"number"`
}

// cardUpdatePreviousValues 仅保存已变字段的旧值。工程师名称和依赖编号随事件留存，
// 以便名册变更或任务删除后仍能展示。
func (u *unit) cardUpdatePreviousValues(ctx context.Context, before *Card, changed []string) (map[string]any, error) {
	previous := make(map[string]any)
	for _, name := range changed {
		switch name {
		case "branch":
			previous[name] = before.Branch
		case "url":
			previous[name] = before.URL
		case "human_only":
			previous[name] = before.HumanOnly
		case "priority":
			previous[name] = before.Priority
		case "engineer_id":
			engineer, err := u.previousEngineerName(ctx, before.BoardID, before.EngineerID)
			if err != nil {
				return nil, err
			}
			previous[name] = engineer
		case "waits_on":
			deps, err := loadDependencies(ctx, u.card, before)
			if err != nil {
				return nil, err
			}
			refs := make([]eventDependencyRef, 0, len(deps))
			for _, dep := range deps {
				if dep.BoardID == before.BoardID {
					refs = append(refs, eventDependencyRef{ID: dep.ID, Number: dep.Number})
				}
			}
			previous[name] = refs
		}
	}
	return previous, nil
}

func (u *unit) previousEngineerName(ctx context.Context, boardID, engineerID int64) (string, error) {
	if engineerID == 0 {
		return "", nil
	}
	engineers, err := u.engineer.ListByBoard(ctx, boardID)
	if err != nil {
		return "", wrapDB("读取工程师名册", err)
	}
	i := slices.IndexFunc(engineers, func(e *Engineer) bool { return e.ID == engineerID })
	if i < 0 {
		return "", nil
	}
	return engineers[i].Name, nil
}
