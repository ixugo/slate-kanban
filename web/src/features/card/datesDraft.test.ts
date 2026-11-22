import { describe, expect, it } from 'vitest'
import { datesPatch, draftOf, draftProblem, todayLocal, withDefaults } from './datesDraft'

const local = (s: string) => new Date(s).toISOString()
const none = { started_at: null, due_at: null }

describe('draftOf', () => {
  it('没有时间时都不勾选，截止时刻用默认值', () => {
    expect(draftOf(none)).toEqual({ start: false, startDate: '', due: false, dueDate: '', dueTime: '18:00' })
  })
  it('有时间时勾选并拆成日期和时刻', () => {
    const d = draftOf({ started_at: local('2026-11-12T08:30'), due_at: local('2026-11-27T01:09') })
    expect(d).toEqual({ start: true, startDate: '2026-11-12', due: true, dueDate: '2026-11-27', dueTime: '01:09' })
  })
})

describe('withDefaults', () => {
  it('勾选后日期为空就补今天，已有的不动', () => {
    const d = withDefaults({ start: true, startDate: '', due: true, dueDate: '2026-12-01', dueTime: '' }, '2026-11-22')
    expect(d).toMatchObject({ startDate: '2026-11-22', dueDate: '2026-12-01', dueTime: '18:00' })
  })
  it('没勾选的不补日期', () => {
    expect(withDefaults({ start: false, startDate: '', due: false, dueDate: '', dueTime: '18:00' }, '2026-11-22').startDate).toBe('')
  })
})

describe('todayLocal', () => {
  it('补零成 YYYY-MM-DD', () => expect(todayLocal(new Date(2026, 0, 5))).toBe('2026-01-05'))
})

describe('draftProblem', () => {
  const base = { start: true, startDate: '2026-11-12', due: true, dueDate: '2026-11-27', dueTime: '18:00' }
  it('正常', () => expect(draftProblem(base)).toBeNull())
  it('勾选了但日期为空', () => {
    expect(draftProblem({ ...base, startDate: '' })).toBe('empty')
    expect(draftProblem({ ...base, dueDate: '' })).toBe('empty')
  })
  it('截止早于开始', () => expect(draftProblem({ ...base, dueDate: '2026-11-01' })).toBe('order'))
  it('同一天可以', () => expect(draftProblem({ ...base, dueDate: '2026-11-12' })).toBeNull())
  it('只勾一个时不比较顺序', () => expect(draftProblem({ ...base, start: false, dueDate: '2020-01-01' })).toBeNull())
})

describe('datesPatch', () => {
  it('没改动返回空对象', () => {
    const card = { started_at: local('2026-11-12T08:30'), due_at: local('2026-11-27T01:09') }
    expect(datesPatch(card, draftOf(card))).toEqual({})
  })
  it('新设开始日期取当天 00:00，新设截止带时刻', () => {
    const patch = datesPatch(none, { start: true, startDate: '2026-11-12', due: true, dueDate: '2026-11-27', dueTime: '09:30' })
    expect(patch).toEqual({ started_at: '2026-11-12T00:00', due_at: '2026-11-27T09:30' })
  })
  it('开始日期没动时不改写自动化记下的具体时刻', () => {
    const card = { started_at: local('2026-11-12T08:30'), due_at: null }
    const draft = { ...draftOf(card), due: true, dueDate: '2026-11-30', dueTime: '10:00' }
    expect(datesPatch(card, draft)).toEqual({ due_at: '2026-11-30T10:00' })
  })
  it('改了开始日期就按新日期 00:00', () => {
    const card = { started_at: local('2026-11-12T08:30'), due_at: null }
    expect(datesPatch(card, { ...draftOf(card), startDate: '2026-11-13' })).toEqual({ started_at: '2026-11-13T00:00' })
  })
  it('取消勾选就清除，原本没有则不提交', () => {
    const card = { started_at: local('2026-11-12T08:30'), due_at: null }
    expect(datesPatch(card, { ...draftOf(card), start: false })).toEqual({ started_at: '' })
    expect(datesPatch(none, { start: false, startDate: '', due: false, dueDate: '', dueTime: '18:00' })).toEqual({})
  })
})
