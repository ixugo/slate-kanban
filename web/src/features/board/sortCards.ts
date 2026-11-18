// 列表视图的卡片排序：纯函数，默认跟看板列顺序一致，支持按列/优先级/截止等切换升降序。

import type { CardView, Column } from '@/lib/api/types'

/** 列表表头对应的排序字段 */
export type SortKey = 'key' | 'title' | 'column' | 'priority' | 'due' | 'updated'

/** 升降序 */
export type SortDir = 'asc' | 'desc'

/** 当前排序状态（不持久化） */
export interface SortState {
  key: SortKey
  dir: SortDir
}

/** 列表默认排序：先按列的 sort，再按卡片 position（与看板视图一致） */
export const DEFAULT_SORT: SortState = { key: 'column', dir: 'asc' }

/** 找不到列时用的兜底排序值，保证未知列_id 排在已知列后面 */
const UNKNOWN_COLUMN_RANK = Number.MAX_SAFE_INTEGER

/**
 * 取出列在看板里的顺序权重；列对象按 sort 排，不是按名字母序。
 */
export function columnRank(columns: Column[], columnId: number): number {
  const col = columns.find((c) => c.id === columnId)
  return col ? col.sort : UNKNOWN_COLUMN_RANK
}

/**
 * 主排序平局时的回退：列顺序 → position → id，保证结果稳定且与看板默认顺序一致。
 */
export function tieBreakCards(a: CardView, b: CardView, columns: Column[]): number {
  const col = columnRank(columns, a.column_id) - columnRank(columns, b.column_id)
  if (col !== 0) return col
  if (a.position !== b.position) return a.position - b.position
  return a.id - b.id
}

/** 比较优先级：0 表示未设置，不论升降序都永远排在有优先级的后面 */
export function comparePriority(a: CardView, b: CardView, dir: SortDir): number {
  const aNone = a.priority === 0
  const bNone = b.priority === 0
  if (aNone && bNone) return 0
  if (aNone) return 1
  if (bNone) return -1
  // 升序 = 高→低（3,2,1）；降序 = 低→高（1,2,3），0 已在上面排除
  const highFirst = b.priority - a.priority
  return dir === 'asc' ? highFirst : -highFirst
}

/** 比较截止时间：没有截止的永远排最后 */
export function compareDue(a: CardView, b: CardView, dir: SortDir): number {
  const aEmpty = !a.due_at
  const bEmpty = !b.due_at
  if (aEmpty && bEmpty) return 0
  if (aEmpty) return 1
  if (bEmpty) return -1
  const diff = Date.parse(a.due_at!) - Date.parse(b.due_at!)
  return dir === 'asc' ? diff : -diff
}

/** 按列排序：列用 sort 字段；同列内再按 position */
function compareColumn(a: CardView, b: CardView, columns: Column[], dir: SortDir): number {
  const ra = columnRank(columns, a.column_id)
  const rb = columnRank(columns, b.column_id)
  if (ra !== rb) return dir === 'asc' ? ra - rb : rb - ra
  const pos = a.position - b.position
  if (pos !== 0) return dir === 'asc' ? pos : -pos
  return a.id - b.id
}

/** 按主键字段比较一对卡片；标题按本地化不区分大小写 */
function compareByKey(
  a: CardView,
  b: CardView,
  columns: Column[],
  key: SortKey,
  dir: SortDir,
): number {
  switch (key) {
    case 'column':
      return compareColumn(a, b, columns, dir)
    case 'priority':
      return comparePriority(a, b, dir)
    case 'due':
      return compareDue(a, b, dir)
    case 'key': {
      const diff = a.number - b.number
      return dir === 'asc' ? diff : -diff
    }
    case 'title': {
      const diff = a.title.localeCompare(b.title, undefined, { sensitivity: 'base' })
      return dir === 'asc' ? diff : -diff
    }
    case 'updated': {
      const diff = Date.parse(a.updated_at) - Date.parse(b.updated_at)
      return dir === 'asc' ? diff : -diff
    }
    default:
      return 0
  }
}

/**
 * 对卡片数组做稳定排序。
 * 假设卡片量在几十到几百张（不做虚拟滚动）；复杂度 O(n log n)。
 */
export function sortCards(cards: CardView[], columns: Column[], state: SortState): CardView[] {
  const indexed = cards.map((card, index) => ({ card, index }))
  indexed.sort((left, right) => {
    let cmp = compareByKey(left.card, right.card, columns, state.key, state.dir)
    if (cmp === 0) cmp = tieBreakCards(left.card, right.card, columns)
    if (cmp === 0) return left.index - right.index
    return cmp
  })
  return indexed.map((row) => row.card)
}

/** 点列头时切换排序：同列翻转升降序，换列则升序起始 */
export function nextSortState(current: SortState, clicked: SortKey): SortState {
  if (current.key === clicked) {
    return { key: clicked, dir: current.dir === 'asc' ? 'desc' : 'asc' }
  }
  return { key: clicked, dir: 'asc' }
}

/** 列头的 aria-sort 值 */
export function ariaSortFor(key: SortKey, state: SortState): 'ascending' | 'descending' | 'none' {
  if (state.key !== key) return 'none'
  return state.dir === 'asc' ? 'ascending' : 'descending'
}