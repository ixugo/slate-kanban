import { describe, expect, it } from 'vitest'
import { formatDate, formatDateTime, formatRelative, fromLocalInput, isPast, parseTime, toLocalInput } from './format'

const NOW = Date.parse('2026-11-18T08:00:00Z')

describe('parseTime', () => {
  it('合法的 RFC3339 返回毫秒，空值和乱写返回 null', () => {
    expect(parseTime('2026-11-18T08:00:00Z')).toBe(NOW)
    expect(parseTime('')).toBeNull()
    expect(parseTime(null)).toBeNull()
    expect(parseTime(undefined)).toBeNull()
    expect(parseTime('not a date')).toBeNull()
  })
})

describe('formatRelative', () => {
  const ago = (seconds: number) => new Date(NOW - seconds * 1000).toISOString()

  it('英文：分钟、小时、天、月、年', () => {
    expect(formatRelative(ago(150), 'en', NOW)).toBe('2 minutes ago')
    expect(formatRelative(ago(3 * 3600), 'en', NOW)).toBe('3 hours ago')
    expect(formatRelative(ago(2 * 86400), 'en', NOW)).toBe('2 days ago')
    expect(formatRelative(ago(60 * 86400), 'en', NOW)).toBe('2 months ago')
    expect(formatRelative(ago(800 * 86400), 'en', NOW)).toBe('2 years ago')
  })

  it('未来的时间说"之后"，并且和过去对称地向下取整', () => {
    expect(formatRelative(ago(-2 * 86400), 'en', NOW)).toBe('in 2 days')
    expect(formatRelative(ago(90), 'en', NOW)).toBe('1 minute ago')
    expect(formatRelative(ago(-90), 'en', NOW)).toBe('in 1 minute')
  })

  it('5 秒以内显示"现在"，不会出现"0 秒前"', () => {
    expect(formatRelative(ago(2), 'en', NOW)).toBe('now')
    expect(formatRelative(ago(-3), 'en', NOW)).toBe('now')
    expect(formatRelative(ago(5), 'en', NOW)).toBe('5 seconds ago')
  })

  it('档位边界：59 秒还是秒，60 秒变成分钟', () => {
    expect(formatRelative(ago(59), 'en', NOW)).toBe('59 seconds ago')
    expect(formatRelative(ago(60), 'en', NOW)).toBe('1 minute ago')
    expect(formatRelative(ago(3599), 'en', NOW)).toBe('59 minutes ago')
    expect(formatRelative(ago(3600), 'en', NOW)).toBe('1 hour ago')
  })

  it('跟随语言：简体中文、日文', () => {
    expect(formatRelative(ago(3 * 3600), 'zh-CN', NOW)).toBe('3小时前')
    expect(formatRelative(ago(3 * 3600), 'ja', NOW)).toBe('3 時間前')
  })

  it('解析不了返回空串', () => {
    expect(formatRelative('oops', 'en', NOW)).toBe('')
    expect(formatRelative(null, 'en', NOW)).toBe('')
  })
})

describe('formatDate / formatDateTime', () => {
  it('按语言格式化，解析不了返回空串', () => {
    expect(formatDate('2026-11-18T08:00:00Z', 'en')).toContain('2026')
    expect(formatDateTime('2026-11-18T08:00:00Z', 'en')).toMatch(/\d{1,2}:\d{2}/)
    expect(formatDate('', 'en')).toBe('')
    expect(formatDateTime('bad', 'en')).toBe('')
  })
})

describe('isPast', () => {
  it('已过去为真，未来、空值、乱写为假', () => {
    expect(isPast('2026-11-17T08:00:00Z', NOW)).toBe(true)
    expect(isPast('2026-11-19T08:00:00Z', NOW)).toBe(false)
    expect(isPast(null, NOW)).toBe(false)
    expect(isPast('bad', NOW)).toBe(false)
  })
})

describe('本地时间输入框的互转', () => {
  it('转出去再转回来是同一个时刻（精确到分钟）', () => {
    const iso = '2026-11-18T08:30:00.000Z'
    const local = toLocalInput(iso)
    expect(local).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/)
    expect(fromLocalInput(local)).toBe(iso)
  })

  it('空串表示清除，格式不对返回 null', () => {
    expect(toLocalInput(null)).toBe('')
    expect(toLocalInput('bad')).toBe('')
    expect(fromLocalInput('')).toBe('')
    expect(fromLocalInput('garbage')).toBeNull()
  })
})
