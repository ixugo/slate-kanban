package kanban

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const archivePageSize = 20

// ArchivedCardsPage 是归档任务的一页，最新归档的任务在前。
type ArchivedCardsPage struct {
	Cards      []*Card `json:"cards"`
	HasMore    bool    `json:"has_more"`
	NextCursor string  `json:"next_cursor,omitempty"`
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

// ArchivedCardsQuery 用稳定游标续页。Before 是旧客户端传入的上一页末尾任务主键。
type ArchivedCardsQuery struct {
	Before int64
	Cursor string
}

// ListArchivedCards 按归档时间和任务主键倒序读取，续页不依赖上一页任务仍然存在。
func (c Core) ListArchivedCards(ctx context.Context, boardID int64, query ArchivedCardsQuery) (*ArchivedCardsPage, error) {
	if query.Before < 0 {
		return nil, invalidf("before 不能是负数")
	}
	if query.Before != 0 && query.Cursor != "" {
		return nil, invalidf("before 和 cursor 只能提供一个")
	}
	if _, err := c.store.Board().GetByID(ctx, boardID); err != nil {
		return nil, mapNotFound(err, ErrBoardNotFound, "读取看板")
	}
	var before *Card
	if query.Cursor != "" {
		position, err := parseArchiveCursor(boardID, query.Cursor)
		if err != nil {
			return nil, err
		}
		before = position
	} else if query.Before > 0 {
		card, err := c.GetCard(ctx, query.Before)
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
	page := &ArchivedCardsPage{Cards: cards[:min(len(cards), archivePageSize)], HasMore: len(cards) > archivePageSize}
	if page.HasMore {
		last := page.Cards[len(page.Cards)-1]
		page.NextCursor = fmt.Sprintf("%d/%d/%s", boardID, last.ID, last.ArchivedAt.Format(time.RFC3339Nano))
	}
	return page, nil
}

func parseArchiveCursor(boardID int64, value string) (*Card, error) {
	board, position, hasBoard := strings.Cut(value, "/")
	id, timestamp, hasPosition := strings.Cut(position, "/")
	cursorBoardID, boardErr := strconv.ParseInt(board, 10, 64)
	cardID, idErr := strconv.ParseInt(id, 10, 64)
	archivedAt, timeErr := time.Parse(time.RFC3339Nano, timestamp)
	if !hasBoard || !hasPosition || boardErr != nil || idErr != nil || timeErr != nil || cursorBoardID != boardID || cardID <= 0 {
		return nil, invalidf("归档列表游标无效")
	}
	return &Card{ID: cardID, ArchivedAt: &archivedAt}, nil
}
