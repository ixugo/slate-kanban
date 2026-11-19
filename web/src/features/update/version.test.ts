import { afterEach, describe, expect, it, vi } from 'vitest'
import { IGNORED_KEY, isNewer, parseVersion, readIgnored, saveIgnored } from './version'

afterEach(() => {
  localStorage.clear()
  vi.restoreAllMocks()
})

describe('parseVersion', () => {
  it('解析带或不带 v 的三段版本号，忽略预发布后缀', () => {
    expect(parseVersion('v1.2.3')).toEqual([1, 2, 3])
    expect(parseVersion('1.10.0')).toEqual([1, 10, 0])
    expect(parseVersion('v1.2.3-rc1')).toEqual([1, 2, 3])
  })

  it.each(['', 'dev', 'v1.2', 'v1.2.x', 'v1.2.3.4', 'v-1.2.3'])('格式不对返回 null：%s', (text) => {
    expect(parseVersion(text)).toBeNull()
  })
})

describe('isNewer', () => {
  it('逐段比较数字而不是字符串', () => {
    expect(isNewer('v1.10.0', 'v1.9.0')).toBe(true)
    expect(isNewer('v2.0.0', 'v1.99.99')).toBe(true)
    expect(isNewer('v1.2.3', 'v1.2.3')).toBe(false)
    expect(isNewer('v1.2.3', 'v1.2.4')).toBe(false)
  })

  it('解析不了就不算更新', () => {
    expect(isNewer('latest', 'v1.0.0')).toBe(false)
    expect(isNewer('v1.0.0', '')).toBe(false)
  })
})

describe('忽略记录', () => {
  it('写入后能读出来，没有记录时是空串', () => {
    expect(readIgnored()).toBe('')
    saveIgnored('v1.2.3')
    expect(localStorage.getItem(IGNORED_KEY)).toBe('v1.2.3')
    expect(readIgnored()).toBe('v1.2.3')
  })

  it('存储不可用时读写都不抛错', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('denied')
    })
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('denied')
    })
    expect(readIgnored()).toBe('')
    expect(() => saveIgnored('v1.0.0')).not.toThrow()
  })
})
