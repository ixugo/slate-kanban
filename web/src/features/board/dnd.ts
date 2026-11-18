// 拖拽的纯逻辑：把"拖完之后的顺序"换算成服务端认的相对位置，并在本地预测移动后的快照。
// 全部是不依赖界面的纯函数，方便单独测试；界面里的 dnd-kit 只负责拿到拖动事件再调用这里。

import { arrayMove } from '@dnd-kit/sortable'
import type { CardView, Column, Snapshot } from '@/lib/api/types'

/** 卡片位置的基本间距，和服务端 internal/core/kanban/card_position.go 的 positionStep 一致 */
export const POSITION_STEP = 1024

/** 每一列里卡片 id 的顺序，键是列 id */
export type Items = Record<number, number[]>

/** 一次移动：落在哪一列，紧挨着哪张卡片；两个邻居都没有表示这一列是空的 */
export interface MovePlan {
  columnId: number
  /** 放在这张卡片的前面 */
  beforeId?: number
  /** 放在这张卡片的后面 */
  afterId?: number
}

/** 卡片的显示顺序：先按位置，位置相同按 id，保证每次刷新顺序一致 */
export function compareCards(a: CardView, b: CardView): number {
  return a.position - b.position || a.id - b.id
}

/**
 * 按列整理卡片 id 的顺序。visibleIds 为 null 表示全部可见，否则只保留里面的卡片（搜索时用）。
 * 属于不存在的列的卡片（数据不一致时才会出现）被忽略。复杂度 O(n log n)。
 */
export function buildItems(columns: Column[], cards: CardView[], visibleIds: ReadonlySet<number> | null): Items {
  const items: Items = {}
  for (const column of columns) items[column.id] = []
  for (const card of [...cards].sort(compareCards)) {
    if (visibleIds && !visibleIds.has(card.id)) continue
    items[card.column_id]?.push(card.id)
  }
  return items
}

/** 找出卡片当前在哪一列；不在任何一列返回 null */
export function findColumnOf(items: Items, cardId: number): number | null {
  for (const [columnId, ids] of Object.entries(items)) {
    if (ids.includes(cardId)) return Number(columnId)
  }
  return null
}

/** 把卡片从当前列拿出来，插入目标列的第 index 个位置（超出范围放到末尾），拖动过程中的实时预览用它 */
export function moveToColumn(items: Items, cardId: number, toColumnId: number, index: number): Items {
  const fromColumnId = findColumnOf(items, cardId)
  if (fromColumnId === null || !(toColumnId in items)) return items
  const next: Items = { ...items, [fromColumnId]: items[fromColumnId].filter((id) => id !== cardId) }
  const target = [...next[toColumnId]]
  target.splice(Math.max(0, Math.min(index, target.length)), 0, cardId)
  next[toColumnId] = target
  return next
}

/**
 * 松手时的最终顺序：落在同一列另一张卡片上，就把卡片挪到那张卡片的位置；
 * 落在列本身上（空列或者列的空白处）不再调整，跨列的情况拖动过程中已经预览过了。
 */
export function finalizeDrop(items: Items, cardId: number, overId: number, overIsColumn: boolean): Items {
  if (overIsColumn) return items
  const columnId = findColumnOf(items, cardId)
  if (columnId === null || findColumnOf(items, overId) !== columnId) return items
  const ids = items[columnId]
  const from = ids.indexOf(cardId)
  const to = ids.indexOf(overId)
  return from === to ? items : { ...items, [columnId]: arrayMove(ids, from, to) }
}

/**
 * 把拖完之后的顺序换算成服务端的移动参数：后面有卡就放在它前面，否则前面有卡就放在它后面，
 * 列是空的只带列。同一列里前后邻居都没变（原地放下）返回 null，调用方不必发请求。
 */
export function planMove(items: Items, original: Items, cardId: number): MovePlan | null {
  const columnId = findColumnOf(items, cardId)
  if (columnId === null) return null
  const ids = items[columnId]
  const index = ids.indexOf(cardId)
  const before = ids[index - 1]
  const after = ids[index + 1]
  if (findColumnOf(original, cardId) === columnId) {
    const oldIds = original[columnId]
    const oldIndex = oldIds.indexOf(cardId)
    if (oldIds[oldIndex - 1] === before && oldIds[oldIndex + 1] === after) return null
  }
  if (after !== undefined) return { columnId, beforeId: after }
  if (before !== undefined) return { columnId, afterId: before }
  return { columnId }
}

/** 新位置：和服务端 positionBetween 同样的规则，两个邻居取中点，只有一边就在那边加减一个间距，都没有是第一个间距 */
export function computePosition(prev?: number, next?: number): number {
  if (prev === undefined && next === undefined) return POSITION_STEP
  if (prev === undefined) return (next as number) - POSITION_STEP
  if (next === undefined) return prev + POSITION_STEP
  return (prev + next) / 2
}

/**
 * 在本地预测移动后的快照，用于乐观更新：换列时同步列名、完成状态，并清掉领取（服务端换列会收回领取）。
 * 预测不了自动化规则的副作用，接口成功后实时流会触发重取，以服务端的结果为准。找不到卡片或列时原样返回。
 */
export function applyMoveToSnapshot(snapshot: Snapshot, cardId: number, plan: MovePlan): Snapshot {
  const card = snapshot.cards.find((c) => c.id === cardId)
  const column = snapshot.columns.find((c) => c.id === plan.columnId)
  if (!card || !column) return snapshot
  const target = snapshot.cards.filter((c) => c.column_id === column.id && c.id !== cardId).sort(compareCards)
  let index = target.length
  if (plan.beforeId !== undefined) {
    const found = target.findIndex((c) => c.id === plan.beforeId)
    if (found >= 0) index = found
  } else if (plan.afterId !== undefined) {
    const found = target.findIndex((c) => c.id === plan.afterId)
    if (found >= 0) index = found + 1
  }
  const position = computePosition(target[index - 1]?.position, target[index]?.position)
  const moved: CardView =
    column.id === card.column_id
      ? { ...card, position }
      : {
          ...card,
          position,
          column_id: column.id,
          column: column.name,
          done: column.is_done,
          claimed_by: '',
          claim_expires_at: null,
          claim_active: false,
        }
  return { ...snapshot, cards: snapshot.cards.map((c) => (c.id === cardId ? moved : c)) }
}
