package kanban

import (
	"context"
	"fmt"
	"slices"

	"github.com/ixugo/goddd/pkg/orm"
)

// columnSortStep 相邻两列排序值的间距。用间距而不是连续整数，
// 是为了新增列时直接取"最大值加一个间距"，不用改动其他列。
const columnSortStep = 1024

// ColumnStorer 是看板列的数据持久化接口。
// 列的修改都发生在已经锁住看板行的事务里，所以更新直接用 Save，不再重复加行锁。
type ColumnStorer interface {
	// WithTx 返回绑定到事务的副本，事务内的所有读写都必须用它。
	WithTx(orm.Tx) ColumnStorer
	// Create 新增一列。
	Create(context.Context, *Column) error
	// Save 把整列写回数据库，布尔值和 0 值也会写入，用于修改和回填下一列。
	Save(context.Context, *Column) error
	// Delete 按主键删除，幂等：列不存在也返回成功。
	Delete(context.Context, *Column) error
	// GetByID 按主键读取，不存在时返回可用 orm.IsErrRecordNotFound 判断的错误。
	GetByID(context.Context, int64) (*Column, error)
	// ListByBoard 返回看板的全部列，按 Sort 升序，Sort 相同按主键升序。
	ListByBoard(context.Context, int64) ([]*Column, error)
	// SetSort 只更新排序值，用于调整列顺序。
	SetSort(ctx context.Context, id, sort int64) error
	// ClearNextColumn 把同一看板里所有指向 columnID 的下一列引用清成 0。
	ClearNextColumn(ctx context.Context, boardID, columnID int64) error
	// DeleteByBoard 删除看板的全部列，用于删除看板。
	DeleteByBoard(context.Context, int64) error
}

// CreateColumn 在看板最右边新增一列，排序值取现有最大值加一个间距。只有人能操作。
// 校验和写入都在锁住看板行的事务里，所以并发新增也不会超过列数上限，或者出现同名列。
func (c Core) CreateColumn(ctx context.Context, actor Actor, boardID int64, in CreateColumnInput) (*Column, error) {
	if err := requireHuman(actor); err != nil {
		return nil, err
	}
	if err := in.normalize(); err != nil {
		return nil, err
	}
	var out *Column
	err := c.run(ctx, boardID, func(u *unit) error {
		cols, err := u.column.ListByBoard(ctx, boardID)
		if err != nil {
			return wrapDB("读取列", err)
		}
		if err := checkNewColumn(cols, in); err != nil {
			return err
		}
		now := u.now()
		col := &Column{
			BoardID: boardID, Name: in.Name, Instruction: in.Instruction,
			IsDone: in.IsDone, NextColumnID: in.NextColumnID,
			Sort: nextColumnSort(cols), CreatedAt: now, UpdatedAt: now,
		}
		if err := u.column.Create(ctx, col); err != nil {
			return wrapDB("创建列", err)
		}
		out = col
		return u.emit(ctx, actor, newEvent{
			BoardID: boardID, Type: EventColumnCreated,
			Payload: map[string]any{"column_id": col.ID, "name": col.Name},
		})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// checkNewColumn 检查新增列是否违反看板里的约束：列数上限、名称重复、下一列不合法。
func checkNewColumn(cols []*Column, in CreateColumnInput) error {
	if len(cols) >= maxColumnsPerBoard {
		return ErrColumnLimit
	}
	if columnNameTaken(cols, in.Name, 0) {
		return ErrColumnNameTaken
	}
	return checkNextColumn(cols, 0, in.NextColumnID)
}

// nextColumnSort 返回新列的排序值：现有最大排序值加一个间距，没有列时是第一个间距。
func nextColumnSort(cols []*Column) int64 {
	var maxSort int64
	for _, col := range cols {
		maxSort = max(maxSort, col.Sort)
	}
	return maxSort + columnSortStep
}

// UpdateColumn 修改列，只更新入参里出现的字段。只有人能操作。
// 没有字段真的变化时直接返回当前的列，不写事件。
func (c Core) UpdateColumn(ctx context.Context, actor Actor, boardID, columnID int64, in UpdateColumnInput) (*Column, error) {
	if err := requireHuman(actor); err != nil {
		return nil, err
	}
	if err := in.normalize(); err != nil {
		return nil, err
	}
	var out *Column
	err := c.run(ctx, boardID, func(u *unit) error {
		cols, err := u.column.ListByBoard(ctx, boardID)
		if err != nil {
			return wrapDB("读取列", err)
		}
		idx := slices.IndexFunc(cols, func(col *Column) bool { return col.ID == columnID })
		if idx < 0 {
			return ErrColumnNotFound
		}
		col := cols[idx]
		out = col
		if err := checkColumnPatch(cols, col, in); err != nil {
			return err
		}
		changed := in.apply(col)
		if len(changed) == 0 {
			return nil
		}
		col.UpdatedAt = u.now()
		if err := u.column.Save(ctx, col); err != nil {
			return wrapDB("保存列", err)
		}
		return u.emit(ctx, actor, newEvent{
			BoardID: boardID, Type: EventColumnUpdated,
			Payload: map[string]any{"column_id": col.ID, "name": col.Name, "fields": changed},
		})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// checkColumnPatch 检查修改是否违反看板里的约束：改名后不能和别的列重名，下一列必须合法。
func checkColumnPatch(cols []*Column, col *Column, in UpdateColumnInput) error {
	if in.Name != nil && columnNameTaken(cols, *in.Name, col.ID) {
		return ErrColumnNameTaken
	}
	if in.NextColumnID != nil {
		return checkNextColumn(cols, col.ID, *in.NextColumnID)
	}
	return nil
}

// ReorderColumns 按给定的 id 顺序重排看板的列。只有人能操作。
// 请求必须带完整的列 id 列表：和当前列集合不完全一致（少了、多了、重复、别的看板的列）
// 就返回 409，因为这说明请求依据的是过期的数据，硬排会丢列或者排乱。
// 顺序没变时不写事件。返回重排后的列。
func (c Core) ReorderColumns(ctx context.Context, actor Actor, boardID int64, ids []int64) ([]*Column, error) {
	if err := requireHuman(actor); err != nil {
		return nil, err
	}
	var out []*Column
	err := c.run(ctx, boardID, func(u *unit) error {
		cols, err := u.column.ListByBoard(ctx, boardID)
		if err != nil {
			return wrapDB("读取列", err)
		}
		if !sameColumnSet(cols, ids) {
			return ErrColumnOrderStale
		}
		ordered, changed, err := u.applyColumnOrder(ctx, cols, ids)
		if err != nil {
			return err
		}
		out = ordered
		if !changed {
			return nil
		}
		return u.emit(ctx, actor, newEvent{
			BoardID: boardID, Type: EventColumnReordered, Payload: map[string]any{"ids": ids},
		})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// sameColumnSet 判断 ids 是不是恰好等于当前所有列的 id，每个只出现一次。
func sameColumnSet(cols []*Column, ids []int64) bool {
	if len(ids) != len(cols) {
		return false
	}
	current := make(map[int64]bool, len(cols))
	for _, col := range cols {
		current[col.ID] = true
	}
	for _, id := range ids {
		if !current[id] {
			return false
		}
		delete(current, id) // 删掉再遇到同一个 id 就会判为不存在，从而发现重复
	}
	return true
}

// applyColumnOrder 把排序值按 ids 的顺序重写成间距的整数倍，返回重排后的列，
// 以及顺序有没有真的变化。顺序本来就和 ids 一致时什么都不写，原样返回当前的列。
func (u *unit) applyColumnOrder(ctx context.Context, cols []*Column, ids []int64) ([]*Column, bool, error) {
	byID := make(map[int64]*Column, len(cols))
	same := true
	for i, col := range cols {
		byID[col.ID] = col
		same = same && ids[i] == col.ID
	}
	if same {
		return cols, false, nil
	}
	ordered := make([]*Column, len(ids))
	for i, id := range ids {
		col := byID[id]
		col.Sort = int64(i+1) * columnSortStep
		if err := u.column.SetSort(ctx, col.ID, col.Sort); err != nil {
			return nil, false, wrapDB("调整列顺序", err)
		}
		ordered[i] = col
	}
	return ordered, true, nil
}

// DeleteColumn 删除一列。只有人能操作。列是空的就直接删；列里有卡片时必须用 moveTo 指定
// 把它们移入的另一列（原顺序追加到那一列末尾），否则返回 409，防止卡片因为删列而消失。
// 其他列指向它的"下一列"会被清掉，引用它的启用中的规则会被停用，因为这些规则已经不可能正常工作。
// 列已经不存在时返回成功（幂等）。因为删列而搬走的卡片不触发自动化规则，这是结构调整，
// 不是有人在流程里推进卡片；事件里只记搬走的张数，不为每张卡片单独写事件。
func (c Core) DeleteColumn(ctx context.Context, actor Actor, boardID, columnID, moveTo int64) error {
	if err := requireHuman(actor); err != nil {
		return err
	}
	return c.run(ctx, boardID, func(u *unit) error {
		return u.deleteColumn(ctx, actor, boardID, columnID, moveTo)
	})
}

// deleteColumn 在事务里完成 DeleteColumn：先搬走卡片，再清理指向这一列的引用，最后删列并追加事件。
func (u *unit) deleteColumn(ctx context.Context, actor Actor, boardID, columnID, moveTo int64) error {
	cols, err := u.column.ListByBoard(ctx, boardID)
	if err != nil {
		return wrapDB("读取列", err)
	}
	col := columnByID(cols, columnID)
	if col == nil {
		return nil
	}
	toID, moved, err := u.evacuateColumn(ctx, actor, cols, col, moveTo)
	if err != nil {
		return err
	}
	if err := u.column.ClearNextColumn(ctx, boardID, columnID); err != nil {
		return wrapDB("清除指向被删列的下一列", err)
	}
	if err := u.disableRulesUsing(ctx, actor, boardID, columnID); err != nil {
		return err
	}
	if err := u.column.Delete(ctx, col); err != nil {
		return wrapDB("删除列", err)
	}
	return u.emit(ctx, actor, newEvent{
		BoardID: boardID, Type: EventColumnDeleted,
		Payload: map[string]any{"column_id": col.ID, "name": col.Name, "moved_to_id": toID, "moved_cards": moved},
	})
}

// evacuateColumn 把列里的卡片按原顺序追加到 moveTo 指定的列末尾，返回目标列的主键和搬走的张数。
// 列是空的不需要目标列，返回 0 和 0。领取的处理和移动卡片一致：agent 操作时清掉，人操作时保留。
// 复杂度是这一列的卡片数，一列的卡片数预期在几百张以内。
func (u *unit) evacuateColumn(ctx context.Context, actor Actor, cols []*Column, col *Column, moveTo int64) (int64, int, error) {
	cards, err := u.card.ListByColumn(ctx, col.ID)
	if err != nil {
		return 0, 0, wrapDB("读取列里的任务", err)
	}
	if len(cards) == 0 {
		return 0, 0, nil
	}
	if moveTo == 0 {
		return 0, 0, ErrColumnNotEmpty.WithMsg(fmt.Sprintf("「%s」列里还有 %d 个任务，请指定把它们移入的列", col.Name, len(cards)))
	}
	target := columnByID(cols, moveTo)
	if target == nil || target.ID == col.ID {
		return 0, 0, invalidf("移入的列不存在，或者不属于这个看板，也不能是要删除的列本身")
	}
	pos, err := u.appendPosition(ctx, target.ID)
	if err != nil {
		return 0, 0, err
	}
	for _, card := range cards {
		if err := u.clearClaimOnMove(ctx, actor, card); err != nil {
			return 0, 0, err
		}
		card.ColumnID, card.Position = target.ID, pos
		pos += positionStep
		if err := u.saveCard(ctx, actor, card); err != nil {
			return 0, 0, err
		}
	}
	return target.ID, len(cards), nil
}

// disableRulesUsing 停用引用了这一列的启用中的规则（触发所在的列，或者移动列动作的目标列），
// 每条追加一个 rule.updated 事件。已经停用的规则不用动：想重新启用时，保存前的校验会指出列不存在。
func (u *unit) disableRulesUsing(ctx context.Context, actor Actor, boardID, columnID int64) error {
	rules, err := u.rule.ListByBoard(ctx, boardID)
	if err != nil {
		return wrapDB("读取规则", err)
	}
	for _, r := range rules {
		if !r.Enabled || !ruleUsesColumn(r, columnID) {
			continue
		}
		r.Enabled, r.UpdatedAt = false, u.now()
		if err := u.rule.Save(ctx, r); err != nil {
			return wrapDB("停用规则", err)
		}
		err := u.emit(ctx, actor, newEvent{BoardID: boardID, Type: EventRuleUpdated, Payload: rulePayload(r)})
		if err != nil {
			return err
		}
	}
	return nil
}

// ruleUsesColumn 判断规则是否引用了这一列：触发所在的列，或者某个移动列动作的目标列。
func ruleUsesColumn(r *Rule, columnID int64) bool {
	return r.ColumnID == columnID || slices.ContainsFunc(r.Actions, func(a RuleAction) bool {
		return a.Type == ActionMoveToColumn && a.ColumnID == columnID
	})
}
