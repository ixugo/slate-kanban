package kanbandb

import (
	"context"

	"github.com/ixugo/goddd/pkg/orm"
	"gorm.io/gorm/clause"

	"slate-kanban/internal/core/kanban"
)

var (
	_ kanban.EventStorer  = Event{}
	_ kanban.CursorStorer = Cursor{}
)

// Event 是事件日志的数据库实现，和 DB 同一个底层结构，通过类型转换得到。
type Event DB

// WithTx 返回绑定到事务的副本。
func (d Event) WithTx(tx orm.Tx) kanban.EventStorer {
	return Event{db: orm.GormDB(tx)}
}

// Create 追加一条事件。
func (d Event) Create(ctx context.Context, model *kanban.Event) error {
	return d.db.WithContext(ctx).Create(model).Error
}

// ListAfter 返回序号大于 afterSeq 的事件，按序号升序。
func (d Event) ListAfter(ctx context.Context, boardID, afterSeq int64, limit int) ([]*kanban.Event, error) {
	items := make([]*kanban.Event, 0)
	err := d.db.WithContext(ctx).
		Where("board_id = ? AND id > ?", boardID, afterSeq).
		Order("id ASC").Limit(limit).Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// ListBefore 返回序号小于 beforeSeq（0 表示不限）且满足筛选条件的事件，按序号降序。
// 条件都是等值或 IN，在 SQLite 和 PostgreSQL 上写法一致；看板和序号走 (board_id, id) 索引。
func (d Event) ListBefore(ctx context.Context, boardID, beforeSeq int64, f kanban.ActivityFilter, limit int) ([]*kanban.Event, error) {
	q := d.db.WithContext(ctx).Where("board_id = ?", boardID)
	if beforeSeq > 0 {
		q = q.Where("id < ?", beforeSeq)
	}
	if len(f.Types) > 0 {
		q = q.Where("type IN ?", f.Types)
	}
	if f.ActorKind != "" {
		q = q.Where("actor_kind = ?", f.ActorKind)
	}
	if f.Actor != "" {
		q = q.Where("actor = ?", f.Actor)
	}
	items := make([]*kanban.Event, 0)
	if err := q.Order("id DESC").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// Actors 返回看板事件里出现过的不同操作者。按别名排序，SELECT DISTINCT 在 PostgreSQL 上要求排序项出现在选择列表里。
func (d Event) Actors(ctx context.Context, boardID int64) ([]kanban.ActivityActor, error) {
	actors := make([]kanban.ActivityActor, 0)
	err := d.db.WithContext(ctx).Model(&kanban.Event{}).Where("board_id = ?", boardID).
		Select("DISTINCT actor_kind AS kind, actor AS name").Order("kind").Order("name").Scan(&actors).Error
	if err != nil {
		return nil, err
	}
	return actors, nil
}

// ListByCard 返回卡片的事件，最新的在前。
func (d Event) ListByCard(ctx context.Context, cardID int64, limit int) ([]*kanban.Event, error) {
	items := make([]*kanban.Event, 0)
	err := d.db.WithContext(ctx).Where("card_id = ?", cardID).
		Order("id DESC").Limit(limit).Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// LatestSeq 返回看板里最大的事件序号。COALESCE 让没有事件时得到 0，而不是 NULL。
func (d Event) LatestSeq(ctx context.Context, boardID int64) (int64, error) {
	var seq int64
	err := d.db.WithContext(ctx).Model(&kanban.Event{}).Where("board_id = ?", boardID).
		Select("COALESCE(MAX(id), 0)").Scan(&seq).Error
	return seq, err
}

// MinSeq 返回看板里还保留着的最小事件序号。
func (d Event) MinSeq(ctx context.Context, boardID int64) (int64, error) {
	var seq int64
	err := d.db.WithContext(ctx).Model(&kanban.Event{}).Where("board_id = ?", boardID).
		Select("COALESCE(MIN(id), 0)").Scan(&seq).Error
	return seq, err
}

// Prune 只保留最新的 keep 条事件。先找出第 keep+1 新的那条事件的序号作为分界线，
// 再删除序号不大于它的事件；用两条简单的语句，在 SQLite 和 PostgreSQL 上写法一致。
func (d Event) Prune(ctx context.Context, boardID int64, keep int) error {
	var boundary []int64
	err := d.db.WithContext(ctx).Model(&kanban.Event{}).Where("board_id = ?", boardID).
		Order("id DESC").Offset(keep).Limit(1).Pluck("id", &boundary).Error
	if err != nil || len(boundary) == 0 {
		return err
	}
	return d.db.WithContext(ctx).
		Where("board_id = ? AND id <= ?", boardID, boundary[0]).Delete(&kanban.Event{}).Error
}

// DeleteByBoard 删除看板的全部事件。
func (d Event) DeleteByBoard(ctx context.Context, boardID int64) error {
	return d.db.WithContext(ctx).Where("board_id = ?", boardID).Delete(&kanban.Event{}).Error
}

// Cursor 是 agent 读取进度的数据库实现，和 DB 同一个底层结构，通过类型转换得到。
type Cursor DB

// WithTx 返回绑定到事务的副本。
func (d Cursor) WithTx(tx orm.Tx) kanban.CursorStorer {
	return Cursor{db: orm.GormDB(tx)}
}

// Get 读取某个 agent 在某个看板上的进度。
func (d Cursor) Get(ctx context.Context, boardID int64, agent string) (*kanban.AgentCursor, error) {
	var out kanban.AgentCursor
	err := d.db.WithContext(ctx).Where("board_id = ? AND agent = ?", boardID, agent).Take(&out).Error
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// Advance 先尝试新建一条进度记录（已存在就什么都不做），再用带条件的 UPDATE 只在新值更大时推进，
// 最后读回结果。两条语句各自都是原子的，并发推进时结果总是其中最大的值。
// 不用 ON CONFLICT DO UPDATE 配合 MAX 或 GREATEST，是因为这两个函数在 SQLite 和 PostgreSQL 上写法不同。
func (d Cursor) Advance(ctx context.Context, boardID int64, agent string, seq int64) (int64, error) {
	db := d.db.WithContext(ctx)
	fresh := &kanban.AgentCursor{BoardID: boardID, Agent: agent, LastSeq: seq}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(fresh).Error; err != nil {
		return 0, err
	}
	err := db.Model(&kanban.AgentCursor{}).
		Where("board_id = ? AND agent = ? AND last_seq < ?", boardID, agent, seq).
		Update("last_seq", seq).Error
	if err != nil {
		return 0, err
	}
	cur, err := d.Get(ctx, boardID, agent)
	if err != nil {
		return 0, err
	}
	return cur.LastSeq, nil
}

// DeleteByBoard 删除看板的全部进度。
func (d Cursor) DeleteByBoard(ctx context.Context, boardID int64) error {
	return d.db.WithContext(ctx).Where("board_id = ?", boardID).Delete(&kanban.AgentCursor{}).Error
}
