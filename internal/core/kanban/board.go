package kanban

import (
	"context"
	"errors"

	"github.com/ixugo/goddd/pkg/orm"
)

// BoardDetail 是看板连同它的列和规则。内嵌 *Board，序列化成 JSON 时看板自己的字段在最外层。
type BoardDetail struct {
	*Board
	Columns []*Column `json:"columns"`
	Rules   []*Rule   `json:"rules"`
}

// BoardStorer 是看板的数据持久化接口。
type BoardStorer interface {
	// WithTx 返回绑定到事务的副本，事务内的所有读写都必须用它。
	WithTx(orm.Tx) BoardStorer
	// Create 新增一个看板。slug 重复时返回可用 orm.IsDuplicatedKey 判断的错误。
	Create(context.Context, *Board) error
	// Save 把整个看板写回数据库，布尔值和 0 值也会写入。
	// 调用方应该已经通过事务执行器锁住了看板行，所以这里不再重复加锁。
	Save(context.Context, *Board) error
	// Delete 按主键删除，幂等：看板不存在也返回成功。
	Delete(context.Context, *Board) error
	// GetByID 按主键读取，不存在时返回可用 orm.IsErrRecordNotFound 判断的错误。
	GetByID(context.Context, int64) (*Board, error)
	// GetBySlug 按 slug 读取，不存在时错误同 GetByID。
	GetBySlug(context.Context, string) (*Board, error)
	// List 返回全部看板，按创建顺序排列。看板数量很少，所以不分页。
	List(context.Context) ([]*Board, error)
	// Lock 锁住看板行并返回最新数据，是同一看板的写事务排队的入口。
	// 只有 PostgreSQL 会真正加行锁；SQLite 只有一个连接，写事务本来就是串行的。
	Lock(context.Context, int64) (*Board, error)
	// IncrCardSeq 把卡片编号计数加一并返回新值，看板不存在时返回找不到记录的错误。
	IncrCardSeq(context.Context, int64) (int64, error)
}

// CreateBoard 创建看板，并按模板和界面语言生成起步的列。只有人能创建。
// 不填标识时按名称自动生成，被占用就换数字后缀重试；手填的标识被占用直接返回 409。
// 每次尝试各自是一个事务：PostgreSQL 里唯一键冲突会让整个事务作废，所以不能在同一个事务里重试。
func (c Core) CreateBoard(ctx context.Context, actor Actor, in CreateBoardInput) (*BoardDetail, error) {
	if err := requireHuman(actor); err != nil {
		return nil, err
	}
	if err := in.normalize(); err != nil {
		return nil, err
	}
	if in.Slug != "" {
		return c.createBoardOnce(ctx, actor, in, in.Slug)
	}
	for attempt := range maxSlugAttempts {
		detail, err := c.createBoardOnce(ctx, actor, in, generateSlug(in.Name, attempt))
		if !errors.Is(err, ErrBoardSlugTaken) {
			return detail, err
		}
	}
	return nil, ErrBoardSlugTaken.Withf("自动生成的标识连续 %d 次都被占用，请手动指定", maxSlugAttempts)
}

// createBoardOnce 在一个事务里用给定的标识创建看板、起步的列，并追加 board.created 事件。
// 看板还不存在，没有行可锁，所以传 0 给 run。
func (c Core) createBoardOnce(ctx context.Context, actor Actor, in CreateBoardInput, slug string) (*BoardDetail, error) {
	var out *BoardDetail
	err := c.run(ctx, 0, func(u *unit) error {
		now := u.now()
		b := &Board{
			Name: in.Name, Slug: slug, Description: in.Description, ProjectPath: in.ProjectPath,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := u.board.Create(ctx, b); err != nil {
			if orm.IsDuplicatedKey(err) {
				return ErrBoardSlugTaken
			}
			return wrapDB("创建看板", err)
		}
		cols, err := u.seedColumns(ctx, b.ID, in.Template, in.Locale)
		if err != nil {
			return err
		}
		out = &BoardDetail{Board: b, Columns: cols, Rules: []*Rule{}}
		return u.emit(ctx, actor, newEvent{
			BoardID: b.ID, Type: EventBoardCreated,
			Payload: map[string]any{"name": b.Name, "slug": b.Slug},
		})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListBoards 返回全部看板，按创建顺序。看板数量很少，所以不分页。人和 agent 都可以读。
func (c Core) ListBoards(ctx context.Context) ([]*Board, error) {
	items, err := c.store.Board().List(ctx)
	if err != nil {
		return nil, wrapDB("读取看板列表", err)
	}
	return items, nil
}

// GetBoard 返回看板和它的列。看板不存在返回 404。人和 agent 都可以读。
func (c Core) GetBoard(ctx context.Context, id int64) (*BoardDetail, error) {
	b, err := c.store.Board().GetByID(ctx, id)
	if err != nil {
		return nil, mapNotFound(err, ErrBoardNotFound, "读取看板")
	}
	cols, err := c.store.Column().ListByBoard(ctx, id)
	if err != nil {
		return nil, wrapDB("读取列", err)
	}
	return &BoardDetail{Board: b, Columns: cols, Rules: []*Rule{}}, nil
}
