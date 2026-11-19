import { describe, expect, it } from 'vitest'
import { displayKey } from './cardKey'

describe('displayKey', () => {
  it('把「前缀-数字」显示成 #数字', () => {
    expect(displayKey('SLATE-40')).toBe('#40')
    expect(displayKey('A1B2-7')).toBe('#7')
    expect(displayKey('TEAM-A-12')).toBe('#12')
  })

  it('格式不对的原样返回，空串也不出错', () => {
    expect(displayKey('')).toBe('')
    expect(displayKey('SLATE')).toBe('SLATE')
    expect(displayKey('SLATE-x')).toBe('SLATE-x')
    expect(displayKey('-5')).toBe('-5')
  })
})
