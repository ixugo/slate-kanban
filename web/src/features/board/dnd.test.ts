import { describe, expect, it } from 'vitest'
import { makeCard, makeColumn, makeSnapshot } from '@/test/factories'
import {
  POSITION_STEP,
  applyMoveToSnapshot,
  buildItems,
  computePosition,
  findColumnOf,
  finalizeDrop,
  moveToColumn,
  planMove,
  type Items,
} from './dnd'

const columns = [makeColumn({ id: 1 }), makeColumn({ id: 2 }), makeColumn({ id: 3, is_done: true })]

describe('buildItems', () => {
  it('每列的卡片按位置排序，位置相同按 id，空列也有一个空数组', () => {
    const cards = [
      makeCard({ id: 5, column_id: 1, position: 3000 }),
      makeCard({ id: 7, column_id: 1, position: 1000 }),
      makeCard({ id: 4, column_id: 1, position: 1000 }),
      makeCard({ id: 6, column_id: 2, position: 500 }),
    ]
    expect(buildItems(columns, cards, null)).toEqual({ 1: [4, 7, 5], 2: [6], 3: [] })
  })

  it('只保留可见的卡片（搜索时）', () => {
    const cards = [makeCard({ id: 1, column_id: 1 }), makeCard({ id: 2, column_id: 1 }), makeCard({ id: 3, column_id: 2 })]
    expect(buildItems(columns, cards, new Set([2, 3]))).toEqual({ 1: [2], 2: [3], 3: [] })
    expect(buildItems(columns, cards, new Set())).toEqual({ 1: [], 2: [], 3: [] })
  })

  it('属于不存在的列的卡片被忽略，不会报错', () => {
    expect(buildItems(columns, [makeCard({ id: 1, column_id: 99 })], null)).toEqual({ 1: [], 2: [], 3: [] })
  })

  it('不修改传入的数组', () => {
    const cards = [makeCard({ id: 2, position: 2000 }), makeCard({ id: 1, position: 1000 })]
    buildItems(columns, cards, null)
    expect(cards.map((c) => c.id)).toEqual([2, 1])
  })
})

describe('findColumnOf / moveToColumn', () => {
  const items: Items = { 1: [10, 11, 12], 2: [20], 3: [] }

  it('找到卡片所在的列，找不到返回 null', () => {
    expect(findColumnOf(items, 20)).toBe(2)
    expect(findColumnOf(items, 99)).toBeNull()
  })

  it('移到别的列的指定下标', () => {
    expect(moveToColumn(items, 11, 2, 0)).toEqual({ 1: [10, 12], 2: [11, 20], 3: [] })
    expect(moveToColumn(items, 11, 2, 1)).toEqual({ 1: [10, 12], 2: [20, 11], 3: [] })
  })

  it('下标越界时放到末尾或开头，移进空列', () => {
    expect(moveToColumn(items, 10, 2, 99)).toEqual({ 1: [11, 12], 2: [20, 10], 3: [] })
    expect(moveToColumn(items, 10, 2, -5)).toEqual({ 1: [11, 12], 2: [10, 20], 3: [] })
    expect(moveToColumn(items, 10, 3, 0)).toEqual({ 1: [11, 12], 2: [20], 3: [10] })
  })

  it('负下标按开头处理，而不是从末尾倒数', () => {
    const two: Items = { 1: [10], 2: [20, 21], 3: [] }
    expect(moveToColumn(two, 10, 2, -1)).toEqual({ 1: [], 2: [10, 20, 21], 3: [] })
  })

  it('卡片或目标列不存在时原样返回，并且不修改原数据', () => {
    expect(moveToColumn(items, 99, 2, 0)).toBe(items)
    expect(moveToColumn(items, 10, 99, 0)).toBe(items)
    moveToColumn(items, 10, 2, 0)
    expect(items).toEqual({ 1: [10, 11, 12], 2: [20], 3: [] })
  })
})

describe('finalizeDrop', () => {
  const items: Items = { 1: [10, 11, 12], 2: [20], 3: [] }

  it('同一列里落在另一张卡片上：挪到它的位置（向下、向上）', () => {
    expect(finalizeDrop(items, 10, 12, false)).toEqual({ 1: [11, 12, 10], 2: [20], 3: [] })
    expect(finalizeDrop(items, 12, 10, false)).toEqual({ 1: [12, 10, 11], 2: [20], 3: [] })
  })

  it('落在自己身上或者列上，不改变顺序', () => {
    expect(finalizeDrop(items, 11, 11, false)).toBe(items)
    expect(finalizeDrop(items, 11, 2, true)).toBe(items)
  })

  it('列 id 和卡片 id 数字相同时，落在列上也不会被当成落在那张卡片上', () => {
    const clash: Items = { 1: [1, 2, 3], 2: [] }
    expect(finalizeDrop(clash, 3, 1, true)).toBe(clash)
    expect(finalizeDrop(clash, 3, 1, false)).toEqual({ 1: [3, 1, 2], 2: [] })
  })

  it('落在别的列的卡片上（预览阶段没来得及移动）不改变顺序', () => {
    expect(finalizeDrop(items, 10, 20, false)).toBe(items)
    expect(finalizeDrop(items, 99, 10, false)).toBe(items)
  })
})

describe('planMove', () => {
  const original: Items = { 1: [10, 11, 12], 2: [20, 21], 3: [] }

  it('同列下移：后面有卡就放在它前面', () => {
    const after: Items = { 1: [11, 10, 12], 2: [20, 21], 3: [] }
    expect(planMove(after, original, 10)).toEqual({ columnId: 1, beforeId: 12 })
  })

  it('同列移到最后：放在最后一张的后面', () => {
    const after: Items = { 1: [11, 12, 10], 2: [20, 21], 3: [] }
    expect(planMove(after, original, 10)).toEqual({ columnId: 1, afterId: 12 })
  })

  it('同列移到最前：放在原第一张的前面', () => {
    const after: Items = { 1: [12, 10, 11], 2: [20, 21], 3: [] }
    expect(planMove(after, original, 12)).toEqual({ columnId: 1, beforeId: 10 })
  })

  it('跨列插到中间：放在后一张的前面', () => {
    const after: Items = { 1: [10, 12], 2: [20, 11, 21], 3: [] }
    expect(planMove(after, original, 11)).toEqual({ columnId: 2, beforeId: 21 })
  })

  it('跨列放到列尾：放在最后一张的后面', () => {
    const after: Items = { 1: [10, 12], 2: [20, 21, 11], 3: [] }
    expect(planMove(after, original, 11)).toEqual({ columnId: 2, afterId: 21 })
  })

  it('拖进空列：只带列', () => {
    const after: Items = { 1: [10, 12], 2: [20, 21], 3: [11] }
    expect(planMove(after, original, 11)).toEqual({ columnId: 3 })
  })

  it('原地放下返回 null；列里只有它自己时也一样', () => {
    expect(planMove(original, original, 11)).toBeNull()
    expect(planMove({ 1: [10], 2: [], 3: [] }, { 1: [10], 2: [], 3: [] }, 10)).toBeNull()
  })

  it('后邻居没变但前邻居变了，也要发请求（两个邻居都没变才算原地）', () => {
    const after: Items = { 1: [12, 10, 11], 2: [20, 21], 3: [] }
    expect(planMove(after, original, 10)).toEqual({ columnId: 1, beforeId: 11 })
  })

  it('前邻居没变但后邻居变了，也要发请求', () => {
    const after: Items = { 1: [10, 11, 13], 2: [20, 21], 3: [] }
    expect(planMove(after, original, 11)).toEqual({ columnId: 1, beforeId: 13 })
  })

  it('换了列即使邻居数量相同也要发请求', () => {
    const solo: Items = { 1: [10], 2: [], 3: [] }
    const moved: Items = { 1: [], 2: [10], 3: [] }
    expect(planMove(moved, solo, 10)).toEqual({ columnId: 2 })
  })

  it('卡片不在任何一列时返回 null', () => {
    expect(planMove(original, original, 99)).toBeNull()
  })

  it('搜索过滤时按可见的邻居计算', () => {
    const visibleOriginal: Items = { 1: [10, 12], 2: [], 3: [] }
    const visibleAfter: Items = { 1: [12, 10], 2: [], 3: [] }
    expect(planMove(visibleAfter, visibleOriginal, 10)).toEqual({ columnId: 1, afterId: 12 })
  })
})

describe('computePosition', () => {
  it('没有邻居是第一个间距，只有一边就在那边加减一个间距，两边取中点', () => {
    expect(computePosition()).toBe(POSITION_STEP)
    expect(computePosition(2048, undefined)).toBe(3072)
    expect(computePosition(undefined, 2048)).toBe(1024)
    expect(computePosition(1024, 2048)).toBe(1536)
    expect(computePosition(0, 0)).toBe(0)
  })

  it('位置是 0 也算有这个邻居，不能当成没有', () => {
    expect(computePosition(0, undefined)).toBe(1024)
    expect(computePosition(undefined, 0)).toBe(-1024)
  })
})

describe('applyMoveToSnapshot', () => {
  const snapshot = makeSnapshot({
    cards: [
      makeCard({ id: 1, column_id: 1, position: 1024 }),
      makeCard({ id: 2, column_id: 1, position: 2048 }),
      makeCard({ id: 3, column_id: 2, position: 1024, claimed_by: 'codex', claim_active: true, claim_expires_at: '2026-11-18T09:00:00Z' }),
      makeCard({ id: 4, column_id: 3, position: 1024, done: true, column: '完成' }),
    ],
  })
  const pos = (s: ReturnType<typeof makeSnapshot>, id: number) => s.cards.find((c) => c.id === id)?.position

  it('同列移到另一张前面：位置取两张之间的中点，其余字段不变', () => {
    const next = applyMoveToSnapshot(snapshot, 2, { columnId: 1, beforeId: 1 })
    expect(pos(next, 2)).toBe(1024 - POSITION_STEP)
    expect(next.cards.find((c) => c.id === 2)?.column_id).toBe(1)
  })

  it('同列移到最后：位置在最后一张之后', () => {
    const next = applyMoveToSnapshot(snapshot, 1, { columnId: 1, afterId: 2 })
    expect(pos(next, 1)).toBe(2048 + POSITION_STEP)
  })

  it('跨列插到两张卡片之间：位置取中点，列名和完成状态同步', () => {
    const snap = makeSnapshot({
      cards: [
        makeCard({ id: 1, column_id: 1 }),
        makeCard({ id: 5, column_id: 3, position: 1000, done: true, column: '完成' }),
        makeCard({ id: 6, column_id: 3, position: 2000, done: true, column: '完成' }),
      ],
    })
    const next = applyMoveToSnapshot(snap, 1, { columnId: 3, afterId: 5 })
    const moved = next.cards.find((c) => c.id === 1)!
    expect(moved.position).toBe(1500)
    expect(moved.column_id).toBe(3)
    expect(moved.column).toBe('完成')
    expect(moved.done).toBe(true)
  })

  it('换列清掉领取，同列移动保留领取', () => {
    const moved = applyMoveToSnapshot(snapshot, 3, { columnId: 1 }).cards.find((c) => c.id === 3)!
    expect(moved).toMatchObject({ column_id: 1, claimed_by: '', claim_active: false, claim_expires_at: null })
    const stay = applyMoveToSnapshot(snapshot, 3, { columnId: 2 }).cards.find((c) => c.id === 3)!
    expect(stay).toMatchObject({ claimed_by: 'codex', claim_active: true })
  })

  it('拖出完成列后不再是完成状态', () => {
    const moved = applyMoveToSnapshot(snapshot, 4, { columnId: 1, beforeId: 1 }).cards.find((c) => c.id === 4)!
    expect(moved.done).toBe(false)
    expect(moved.column).toBe('待办')
  })

  it('拖进空列：位置是第一个间距', () => {
    const next = applyMoveToSnapshot(makeSnapshot({ cards: [makeCard({ id: 1, column_id: 1 })] }), 1, { columnId: 3 })
    expect(pos(next, 1)).toBe(POSITION_STEP)
  })

  it('邻居不在目标列里时放到末尾，不会崩溃', () => {
    const next = applyMoveToSnapshot(snapshot, 1, { columnId: 1, beforeId: 999 })
    expect(pos(next, 1)).toBe(2048 + POSITION_STEP)
  })

  it('找不到卡片或列时原样返回；不修改原快照', () => {
    expect(applyMoveToSnapshot(snapshot, 99, { columnId: 1 })).toBe(snapshot)
    expect(applyMoveToSnapshot(snapshot, 1, { columnId: 99 })).toBe(snapshot)
    applyMoveToSnapshot(snapshot, 1, { columnId: 2 })
    expect(snapshot.cards.find((c) => c.id === 1)?.column_id).toBe(1)
  })
})
