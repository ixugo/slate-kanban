package kanban

import (
	"context"
	"errors"
	"slices"

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

// CreateBoard 创建看板，并按模板和界面语言生成起步的列。人和 agent 都能创建，自动化规则不行；删除看板仍然只有人能做。
// 不填标识时按名称自动生成，被占用就换数字后缀重试；手填的标识被占用直接返回 409。
// 每次尝试各自是一个事务：PostgreSQL 里唯一键冲突会让整个事务作废，所以不能在同一个事务里重试。
func (c Core) CreateBoard(ctx context.Context, actor Actor, in CreateBoardInput) (*BoardDetail, error) {
	if err := requireWriter(actor); err != nil {
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

// createBoardOnce 在一个事务里用给定的标识创建看板、起步的列和规则，并追加 board.created 事件。
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
		rules, err := u.seedRules(ctx, b.ID, in.Locale, cols)
		if err != nil {
			return err
		}
		out = &BoardDetail{Board: b, Columns: cols, Rules: rules}
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

// GetBoard 返回看板、它的列和自动化规则（含停用的）。看板不存在返回 404。人和 agent 都可以读。
func (c Core) GetBoard(ctx context.Context, id int64) (*BoardDetail, error) {
	b, err := c.store.Board().GetByID(ctx, id)
	if err != nil {
		return nil, mapNotFound(err, ErrBoardNotFound, "读取看板")
	}
	cols, err := c.store.Column().ListByBoard(ctx, id)
	if err != nil {
		return nil, wrapDB("读取列", err)
	}
	rules, err := c.store.Rule().ListByBoard(ctx, id)
	if err != nil {
		return nil, wrapDB("读取规则", err)
	}
	return &BoardDetail{Board: b, Columns: cols, Rules: rules}, nil
}

// UpdateBoard 修改看板的名称、说明或项目路径，只更新入参里出现的字段。
// 人可以改全部三项；agent 只能在项目路径为空时写入项目路径，其他一律拒绝。
// 没有字段真的变化时直接返回当前看板，不写事件。
func (c Core) UpdateBoard(ctx context.Context, actor Actor, id int64, in UpdateBoardInput) (*Board, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	if err := in.normalize(); err != nil {
		return nil, err
	}
	if err := checkBoardPatchAllowed(actor, in); err != nil {
		return nil, err
	}
	var out *Board
	err := c.run(ctx, id, func(u *unit) error {
		// run 已经锁住了看板行，这里读到的一定是最新值，
		// 所以两个 agent 同时写项目路径时，后到的会看到前一个写入的值而被拒绝。
		b, err := u.board.GetByID(ctx, id)
		if err != nil {
			return mapNotFound(err, ErrBoardNotFound, "读取看板")
		}
		if actor.IsAgent() && b.ProjectPath != "" && *in.ProjectPath != b.ProjectPath {
			return ErrProjectPathLocked
		}
		out = b
		changed := in.apply(b)
		if len(changed) == 0 {
			return nil
		}
		b.UpdatedAt = u.now()
		if err := u.board.Save(ctx, b); err != nil {
			return wrapDB("保存看板", err)
		}
		return u.emitBoardUpdates(ctx, actor, id, changed)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// checkBoardPatchAllowed 在进入事务之前先挡掉明显越权的修改：
// 人可以改任何字段，agent 只能提交项目路径这一个字段，系统操作者不能改看板。
// "路径是否已有值"要在锁住看板行之后再判断，不在这里。
func checkBoardPatchAllowed(actor Actor, in UpdateBoardInput) error {
	switch {
	case actor.IsHuman():
		return nil
	case actor.IsAgent() && in.Name == nil && in.Description == nil && in.ProjectPath != nil:
		return nil
	}
	return ErrHumanOnly.WithMsg("agent 只能在项目路径为空时写入项目路径，其他修改只允许人执行")
}

// DeleteBoard 删除看板和它的全部数据：评论、卡片、规则、列、agent 的读取进度和事件。只有人能操作。
// 全部在一个事务里按依赖顺序删除，任何一步失败整体回滚，不会留下孤儿数据。
// 删除是幂等的：看板已经不存在也返回成功。事件随看板一起删了，所以提交后只广播一次唤醒信号，
// 让还在等待这个看板更新的人醒来，重新查询时得到"看板不存在"。
func (c Core) DeleteBoard(ctx context.Context, actor Actor, id int64) error {
	if err := requireHuman(actor); err != nil {
		return err
	}
	err := c.run(ctx, id, func(u *unit) error {
		return u.deleteBoard(ctx, id)
	})
	if errors.Is(err, ErrBoardNotFound) {
		return nil
	}
	return err
}

// deleteBoard 在事务里按依赖顺序删除看板的全部数据，最后删看板自己，并标记要广播。
// 一个看板的卡片数量预期在几千张以内，每类数据一条删除语句，所以是常数条语句。
func (u *unit) deleteBoard(ctx context.Context, id int64) error {
	b, err := u.board.GetByID(ctx, id)
	if err != nil {
		return mapNotFound(err, ErrBoardNotFound, "读取看板")
	}
	steps := []struct {
		what string
		del  func() error
	}{
		{"评论", func() error { return u.comment.DeleteByBoard(ctx, id) }},
		{"图片", func() error { return u.attachment.DeleteByBoard(ctx, id) }},
		{"卡片", func() error { return u.card.DeleteByBoard(ctx, id) }},
		{"规则", func() error { return u.rule.DeleteByBoard(ctx, id) }},
		{"列", func() error { return u.column.DeleteByBoard(ctx, id) }},
		{"读取进度", func() error { return u.cursor.DeleteByBoard(ctx, id) }},
		{"事件", func() error { return u.event.DeleteByBoard(ctx, id) }},
		{"看板", func() error { return u.board.Delete(ctx, b) }},
	}
	for _, s := range steps {
		if err := s.del(); err != nil {
			return wrapDB("删除看板的"+s.what, err)
		}
	}
	u.touch(id)
	return nil
}

// emitBoardUpdates 按改了什么写事件：标签库的变化单独写 board.labels_updated，
// 其余字段写 board.updated，同时改了两类就两条都写。原因同 emitCardUpdates。
func (u *unit) emitBoardUpdates(ctx context.Context, actor Actor, boardID int64, changed []string) error {
	rest := slices.DeleteFunc(slices.Clone(changed), func(f string) bool { return f == "labels" })
	if len(rest) > 0 {
		if err := u.emit(ctx, actor, newEvent{BoardID: boardID, Type: EventBoardUpdated, Payload: map[string]any{"fields": rest}}); err != nil {
			return err
		}
	}
	if len(rest) == len(changed) {
		return nil
	}
	return u.emit(ctx, actor, newEvent{BoardID: boardID, Type: EventBoardLabelsUpdated, Payload: map[string]any{"fields": []string{"labels"}}})
}
