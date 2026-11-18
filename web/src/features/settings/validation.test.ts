import { describe, expect, it } from 'vitest'
import { makeColumn } from '@/test/factories'
import { countCardsInColumn, moveColumnDown, moveColumnUp } from './validation'

describe('列校验与排序', () => {
  const columns = [makeColumn({ id: 1, sort: 1 }), makeColumn({ id: 2, sort: 2 }), makeColumn({ id: 3, sort: 3 })]

  it('统计列内卡片数', () => {
    expect(countCardsInColumn(1, [{ column_id: 1 }, { column_id: 1 }, { column_id: 2 }])).toBe(2)
  })

  it('上移与下移重排 id', () => {
    expect(moveColumnUp(columns, 2)).toEqual([2, 1, 3])
    expect(moveColumnDown(columns, 2)).toEqual([1, 3, 2])
    expect(moveColumnUp(columns, 1)).toBeNull()
  })
})