import { describe, expect, it } from 'vitest'
import { en } from '@/i18n/locales/en'
import { makeColumn } from '@/test/factories'
import { describeFilters } from './filterChips'
import { EMPTY_FILTERS, type BoardFilters } from './filters'

/** 按英文文案翻译，占位符 {{x}} 换成参数 */
const t = ((key: string, params?: Record<string, string | number>) => {
  const text = key.split('.').reduce<unknown>((node, part) => (node as Record<string, unknown>)[part], en) as string
  return text.replace(/\{\{(\w+)\}\}/g, (_, name: string) => String(params?.[name]))
}) as Parameters<typeof describeFilters>[2]

const columns = [makeColumn({ id: 1, name: 'Todo' }), makeColumn({ id: 2, name: 'Doing' })]
const filters = (patch: Partial<BoardFilters>): BoardFilters => ({ ...EMPTY_FILTERS, ...patch })

describe('describeFilters', () => {
  it('什么都没选时没有小标签', () => {
    expect(describeFilters(EMPTY_FILTERS, columns, t)).toEqual([])
  })

  it('每个选中的取值一个小标签，列 id 换成列名，顺序和弹层里的分组一致', () => {
    const chips = describeFilters(
      filters({ tags: ['bug'], columns: [2], states: ['free'], priorities: ['high'], claimers: ['codex'], due: 'today' }),
      columns,
      t,
    )
    expect(chips.map((c) => c.label)).toEqual([
      'Tags: bug',
      'Column: Doing',
      'Status: Unclaimed',
      'Priority: High',
      'Claimed by: codex',
      'Due: Due today',
    ])
  })

  it('标签选了多个并且是「全部」时，标签组名后面注明', () => {
    const chips = describeFilters(filters({ tags: ['a', 'b'], tagMode: 'all' }), columns, t)
    expect(chips.map((c) => c.label)).toEqual(['Tags (all): a', 'Tags (all): b'])
    expect(describeFilters(filters({ tags: ['a'], tagMode: 'all' }), columns, t)[0].label).toBe('Tags: a')
  })

  it('取消一个小标签只去掉它自己，其余条件保持不变', () => {
    const start = filters({ tags: ['a', 'b'], columns: [1], due: 'overdue' })
    const byKey = Object.fromEntries(describeFilters(start, columns, t).map((c) => [c.key, c.next]))
    expect(byKey['tag:a']).toMatchObject({ tags: ['b'], columns: [1], due: 'overdue' })
    expect(byKey['col:1']).toMatchObject({ tags: ['a', 'b'], columns: [], due: 'overdue' })
    expect(byKey['due']).toMatchObject({ tags: ['a', 'b'], columns: [1], due: 'any' })
  })

  it('「没有标签」有自己的小标签；列已经不存在时显示 #id', () => {
    expect(describeFilters(filters({ noTags: true }), columns, t)[0]).toMatchObject({
      label: 'Tags: No tags',
      next: EMPTY_FILTERS,
    })
    expect(describeFilters(filters({ columns: [99] }), columns, t)[0].label).toBe('Column: #99')
  })
})
