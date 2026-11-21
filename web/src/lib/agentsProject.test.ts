import { describe, expect, it } from 'vitest'
import { agentsSearchFromBoardSlug, parseAgentsProjectParam } from './agentsProject'

describe('agentsProject', () => {
  it('空串或缺省都视为未指定 project', () => {
    expect(parseAgentsProjectParam(null)).toBe(null)
    expect(parseAgentsProjectParam('')).toBe(null)
  })

  it('有值时转为大写 slug', () => {
    expect(parseAgentsProjectParam('slate')).toBe('SLATE')
  })

  it('没有 slug 时不生成查询串', () => {
    expect(agentsSearchFromBoardSlug(undefined)).toBe('')
  })

  it('有 slug 时生成小写 project 参数', () => {
    expect(agentsSearchFromBoardSlug('SLATE')).toBe('?project=slate')
  })
})