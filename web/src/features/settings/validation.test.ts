import { describe, expect, it } from 'vitest'
import { makeColumn } from '@/test/factories'
import { countCardsInColumn, moveColumnDown, moveColumnUp, validateDraft } from './validation'

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

describe('工程师校验', () => {
  const all = [{ id: 1, name: 'Dev A', kind: 'dev' as const, prompt: 'p', avatar: '' }]
  const draft = { id: 0, name: 'Dev B', kind: 'dev' as const, prompt: 'p', avatar: '' }

  it('名称、提示词为空（含全空白）被拒绝', () => {
    expect(validateDraft({ ...draft, name: '  ' }, all)).toBe('nameRequired')
    expect(validateDraft({ ...draft, prompt: ' \n', avatar: '' }, all)).toBe('promptRequired')
  })

  it('重名不分大小写；改自己的名字不算重名', () => {
    expect(validateDraft({ ...draft, name: ' dev a ' }, all)).toBe('nameTaken')
    expect(validateDraft({ ...all[0], name: 'DEV A' }, all)).toBeNull()
  })

  it('名册满 20 位后不能再招募，但能编辑已有的', () => {
    const full = Array.from({ length: 20 }, (_, i) => ({ id: i + 1, name: `E${i}`, kind: 'dev' as const, prompt: 'p', avatar: '' }))
    expect(validateDraft(draft, full)).toBe('limitReached')
    expect(validateDraft(full[0], full)).toBeNull()
  })
})
