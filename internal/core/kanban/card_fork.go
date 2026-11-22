package kanban

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ForkCard 在同一看板的收件箱里创建任务，只继承继续工作所需的属性。
func (c Core) ForkCard(ctx context.Context, actor Actor, cardID int64) (*Card, error) {
	if err := requireHuman(actor); err != nil {
		return nil, err
	}
	boardID, err := c.boardOfCard(ctx, cardID)
	if err != nil {
		return nil, err
	}
	var out *Card
	err = c.run(ctx, boardID, func(u *unit) error {
		source, err := u.loadCard(ctx, boardID, cardID)
		if err != nil {
			return err
		}
		columns, err := u.column.ListByBoard(ctx, boardID)
		if err != nil {
			return wrapDB("读取列", err)
		}
		inbox := findInbox(columns)
		if inbox == nil {
			return ErrInboxNotFound
		}
		if source.ArchivedAt != nil {
			if err := u.checkForkRelations(ctx, source); err != nil {
				return err
			}
		}
		// Fork 的落点和继承字段由调用契约决定，不触发可能改写它们的创建规则。
		out, err = u.createCard(ctx, actor, boardID, CreateCardInput{
			ColumnID:    inbox.ID,
			Title:       source.Title,
			Description: fmt.Sprintf("接着任务 #%d 继续", source.Number),
			Priority:    source.Priority,
			TagIDs:      source.TagIDs,
			WaitsOn:     source.WaitsOn,
			URL:         source.URL,
			HumanOnly:   source.HumanOnly,
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// checkForkRelations 拒绝副本中无法复用的历史标签或依赖。
func (u *unit) checkForkRelations(ctx context.Context, source *Card) error {
	if err := u.checkTagIDs(ctx, source.BoardID, source.TagIDs); err != nil {
		if errors.Is(err, ErrInvalidInput) {
			return ErrForkRelationUnavailable
		}
		return err
	}
	if len(source.WaitsOn) > 0 {
		if err := u.checkWaitsOn(ctx, source.BoardID, 0, source.WaitsOn); err != nil {
			if errors.Is(err, ErrInvalidInput) {
				return ErrForkRelationUnavailable
			}
			return err
		}
	}
	return nil
}

// findInbox recognizes the built-in names, or a renamed column keeping its built-in inbox instructions.
func findInbox(columns []*Column) *Column {
	matches := func(column *Column, match func(columnText) bool) bool {
		if column.IsDone {
			return false
		}
		for _, text := range templateTexts {
			if match(text.Columns[textIdea]) {
				return true
			}
		}
		return false
	}
	for _, column := range columns {
		if matches(column, func(text columnText) bool {
			return strings.EqualFold(strings.TrimSpace(column.Name), text.Name)
		}) {
			return column
		}
	}
	for _, column := range columns {
		if matches(column, func(text columnText) bool { return column.Instruction == text.Instruction }) {
			return column
		}
	}
	return nil
}
