package kanban

import (
	"context"
	"time"
)

const archivePageSize = 20

// ArchivedCardsPage 是归档任务的一页，最新归档的任务在前。
type ArchivedCardsPage struct {
	Cards   []*Card `json:"cards"`
	HasMore bool    `json:"has_more"`
}

// archiveRelations 保留归档时的显示信息。Tags 非 nil 表示已捕获快照，即使当时没有标签。
type archiveRelations struct {
	Tags         []*Tag     `json:"tags"`
	Dependencies []*CardRef `json:"dependencies"`
	Column       string     `json:"column"`
	Engineer     string     `json:"engineer"`
}

func archivedTags(card *Card, tags []*Tag) []*Tag {
	byID := make(map[int64]*Tag, len(tags))
	for _, tag := range tags {
		byID[tag.ID] = tag
	}
	out := make([]*Tag, 0, len(card.TagIDs))
	for _, id := range card.TagIDs {
		if tag := byID[id]; tag != nil {
			out = append(out, tag)
		}
	}
	return out
}

func (u *unit) captureArchiveRelations(ctx context.Context, card *Card, cols []*Column) (archiveRelations, error) {
	tags, err := u.tag.ListByBoard(ctx, card.BoardID)
	if err != nil {
		return archiveRelations{}, wrapDB("读取标签库", err)
	}
	deps, err := loadDependencies(ctx, u.card, card)
	if err != nil {
		return archiveRelations{}, err
	}
	ix := newBoardIndex(u.now(), time.Local, cols, nil, deps)
	out := archiveRelations{
		Tags:         archivedTags(card, tags),
		Dependencies: dependencyRefs(ix, deps),
	}
	if col := ix.columns[card.ColumnID]; col != nil {
		out.Column = col.Name
	}
	if card.EngineerID != 0 {
		engineers, err := u.engineer.ListByBoard(ctx, card.BoardID)
		if err != nil {
			return archiveRelations{}, wrapDB("读取工程师", err)
		}
		for _, engineer := range engineers {
			if engineer.ID == card.EngineerID {
				out.Engineer = engineer.Name
				break
			}
		}
	}
	return out, nil
}

func checkCardUnarchived(card *Card) error {
	if card.ArchivedAt != nil {
		return ErrCardArchived
	}
	return nil
}

// ArchiveCard 保留任务和历史，结束持有者的租约。重复归档不会改变归档时间或写入新事件。
func (c Core) ArchiveCard(ctx context.Context, actor Actor, cardID int64) (*Card, error) {
	if err := requireHuman(actor); err != nil {
		return nil, err
	}
	boardID, err := c.boardOfCard(ctx, cardID)
	if err != nil {
		return nil, err
	}
	var out *Card
	err = c.run(ctx, boardID, func(u *unit) error {
		card, err := u.loadCard(ctx, boardID, cardID)
		if err != nil {
			return err
		}
		out = card
		if card.ArchivedAt != nil {
			return nil
		}
		if err := u.clearClaim(ctx, actor, card, releaseRevoked); err != nil {
			return err
		}
		cols, err := u.column.ListByBoard(ctx, boardID)
		if err != nil {
			return wrapDB("读取列", err)
		}
		if col := columnByID(cols, card.ColumnID); col != nil {
			card.ArchivedDone = col.IsDone
		}
		card.Archive, err = u.captureArchiveRelations(ctx, card, cols)
		if err != nil {
			return err
		}
		card.ArchivedAt = new(u.now())
		return u.saveCardChange(ctx, actor, card, EventCardArchived, nil)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListArchivedCards 只读取指定看板的归档任务。下一页用上一页最后一个任务主键定位。
func (c Core) ListArchivedCards(ctx context.Context, boardID, beforeID int64) (*ArchivedCardsPage, error) {
	if beforeID < 0 {
		return nil, invalidf("before 不能是负数")
	}
	if _, err := c.store.Board().GetByID(ctx, boardID); err != nil {
		return nil, mapNotFound(err, ErrBoardNotFound, "读取看板")
	}
	var before *Card
	if beforeID > 0 {
		card, err := c.GetCard(ctx, beforeID)
		if err != nil {
			return nil, err
		}
		if card.BoardID != boardID || card.ArchivedAt == nil {
			return nil, ErrCardNotFound
		}
		before = card
	}
	cards, err := c.store.Card().ListArchivedByBoard(ctx, boardID, before, archivePageSize+1)
	if err != nil {
		return nil, wrapDB("读取归档任务", err)
	}
	return &ArchivedCardsPage{Cards: cards[:min(len(cards), archivePageSize)], HasMore: len(cards) > archivePageSize}, nil
}
