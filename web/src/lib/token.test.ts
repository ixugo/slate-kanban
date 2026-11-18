import { afterEach, describe, expect, it, vi } from 'vitest'
import { TOKEN_KEY, clearToken, getToken, setToken, subscribeToken } from './token'

afterEach(() => {
  clearToken()
  vi.restoreAllMocks()
})

describe('token', () => {
  it('保存后能读到，清除后是空串，并且落在 localStorage 里', () => {
    setToken('abc')
    expect(getToken()).toBe('abc')
    expect(localStorage.getItem(TOKEN_KEY)).toBe('abc')
    clearToken()
    expect(getToken()).toBe('')
    expect(localStorage.getItem(TOKEN_KEY)).toBeNull()
  })

  it('保存和清除都会通知订阅者，取消订阅后不再通知', () => {
    const listener = vi.fn()
    const unsubscribe = subscribeToken(listener)
    setToken('a')
    clearToken()
    expect(listener).toHaveBeenCalledTimes(2)
    unsubscribe()
    setToken('b')
    expect(listener).toHaveBeenCalledTimes(2)
  })

  it('别的标签页改了令牌（storage 事件）也会通知，改的是无关的键则不通知', () => {
    const listener = vi.fn()
    const unsubscribe = subscribeToken(listener)
    window.dispatchEvent(new StorageEvent('storage', { key: TOKEN_KEY }))
    window.dispatchEvent(new StorageEvent('storage', { key: null }))
    window.dispatchEvent(new StorageEvent('storage', { key: 'other' }))
    expect(listener).toHaveBeenCalledTimes(2)
    unsubscribe()
  })

  it('只有写入失败（读取正常）时也改用内存，不会读回存储里的旧令牌', async () => {
    vi.resetModules()
    vi.spyOn(Storage.prototype, 'getItem').mockReturnValue('old-token')
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('quota exceeded')
    })
    const fresh = await import('./token')
    expect(fresh.getToken()).toBe('old-token')
    fresh.setToken('new-token')
    expect(fresh.getToken()).toBe('new-token')
  })

  it('本地存储不可用时退回内存，当前页面里仍然能用', async () => {
    vi.resetModules()
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('denied')
    })
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('denied')
    })
    const fresh = await import('./token')
    fresh.setToken('mem')
    expect(fresh.getToken()).toBe('mem')
    fresh.clearToken()
    expect(fresh.getToken()).toBe('')
  })
})
