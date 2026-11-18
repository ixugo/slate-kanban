import { describe, expect, it } from 'vitest'
import { makeCard, makeColumn } from '@/test/factories'
import {
  DEFAULT_SORT,
  compareDue,
  comparePriority,
  sortCards,
  tieBreakCards,
  type SortState,
} from './sortCards'

const columns = [
  makeColumn({ id: 1, name: '待办', sort: 10 }),
  makeColumn({ id: 2, name: '进行中', sort: 20 }),
]

/** 方便构造带 position 的卡片 */
function card(
  id: number,
  extra: Parameters<typeof makeCard>[0] extends infer P ? Partial<Omit<P, 'id'>> : never = {},
) {
  return makeCard({ id, position: id * 1024, ...extra })
}

describe('sortCards', () => {
  it('空列表不报错', () => {
    expect(sortCards([], columns, DEFAULT_SORT)).toEqual([])
  })

  it('默认按列 sort 与同列 position', () => {
    const cards = [
      card(3, { column_id: 2, position: 2048 }),
      card(1, { column_id: 1, position: 1024 }),
      card(2, { column_id: 1, position: 3072 }),
    ]
    const sorted = sortCards(cards, columns, DEFAULT_SORT)
    expect(sorted.map((c) => c.id)).toEqual([1, 2, 3])
  })

  it('按优先级升序：高→低，0 永远最后', () => {
    const cards = [
      card(1, { priority: 0 }),
      card(2, { priority: 3 }),
      card(3, { priority: 1 }),
      card(4, { priority: 2 }),
    ]
    const state: SortState = { key: 'priority', dir: 'asc' }
    expect(sortCards(cards, columns, state).map((c) => c.id)).toEqual([2, 4, 3, 1])
  })

  it('按优先级降序：低→高，0 仍最后', () => {
    const cards = [
      card(1, { priority: 0 }),
      card(2, { priority: 3 }),
      card(3, { priority: 1 }),
    ]
    const state: SortState = { key: 'priority', dir: 'desc' }
    expect(sortCards(cards, columns, state).map((c) => c.id)).toEqual([3, 2, 1])
  })

  it('按截止升序：早→晚，无截止最后', () => {
    const cards = [
      card(1, { due_at: null }),
      card(2, { due_at: '2026-12-01T00:00:00Z' }),
      card(3, { due_at: '2026-11-01T00:00:00Z' }),
    ]
    const state: SortState = { key: 'due', dir: 'asc' }
    expect(sortCards(cards, columns, state).map((c) => c.id)).toEqual([3, 2, 1])
  })

  it('按截止降序：晚→早，无截止仍最后', () => {
    const cards = [
      card(1, { due_at: null }),
      card(2, { due_at: '2026-11-01T00:00:00Z' }),
      card(3, { due_at: '2026-12-01T00:00:00Z' }),
    ]
    const state: SortState = { key: 'due', dir: 'desc' }
    expect(sortCards(cards, columns, state).map((c) => c.id)).toEqual([3, 2, 1])
  })

  it('稳定性：主键相等时保持原顺序', () => {
    const cards = [card(5, { priority: 2 }), card(6, { priority: 2 }), card(7, { priority: 2 })]
    const state: SortState = { key: 'priority', dir: 'asc' }
    const again = sortCards(cards, columns, state)
    expect(again.map((c) => c.id)).toEqual([5, 6, 7])
  })

  it('平局回退到列顺序与 position', () => {
    const a = card(10, { title: 'Zulu', column_id: 1, position: 5000 })
    const b = card(11, { title: 'Alpha', column_id: 1, position: 1000 })
    expect(tieBreakCards(a, b, columns)).toBeGreaterThan(0)
    expect(sortCards([a, b], columns, { key: 'title', dir: 'asc' }).map((c) => c.id)).toEqual([11, 10])
  })
})

describe('其余排序字段', () => {
  const cards = [
    card(1, { number: 12, title: 'Banana', updated_at: '2026-11-02T00:00:00Z', column_id: 2 }),
    card(2, { number: 3, title: 'apple', updated_at: '2026-11-03T00:00:00Z', column_id: 1 }),
    card(3, { number: 7, title: 'Cherry', updated_at: '2026-11-01T00:00:00Z', column_id: 1 }),
  ]
  const ids = (state: SortState) => sortCards(cards, columns, state).map((c) => c.id)

  it('按编号按数字大小而不是字符串排', () => {
    expect(ids({ key: 'key', dir: 'asc' })).toEqual([2, 3, 1])
    expect(ids({ key: 'key', dir: 'desc' })).toEqual([1, 3, 2])
  })

  it('按标题不区分大小写', () => {
    expect(ids({ key: 'title', dir: 'asc' })).toEqual([2, 1, 3])
    expect(ids({ key: 'title', dir: 'desc' })).toEqual([3, 1, 2])
  })

  it('按更新时间', () => {
    expect(ids({ key: 'updated', dir: 'asc' })).toEqual([3, 1, 2])
    expect(ids({ key: 'updated', dir: 'desc' })).toEqual([2, 1, 3])
  })

  it('按列降序时后面的列排前面', () => {
    expect(ids({ key: 'column', dir: 'desc' })).toEqual([1, 3, 2])
  })
})

describe('comparePriority / compareDue 边界', () => {
  it('两个都无优先级相等', () => {
    const a = card(1, { priority: 0 })
    const b = card(2, { priority: 0 })
    expect(comparePriority(a, b, 'asc')).toBe(0)
    expect(comparePriority(a, b, 'desc')).toBe(0)
  })

  it('两个都无截止相等', () => {
    const a = card(1, { due_at: null })
    const b = card(2, { due_at: null })
    expect(compareDue(a, b, 'asc')).toBe(0)
  })
})