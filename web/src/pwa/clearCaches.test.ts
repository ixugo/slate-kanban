import { afterEach, describe, expect, it, vi } from 'vitest'
import { clearOfflineCaches } from './clearCaches'

afterEach(() => vi.unstubAllGlobals())

// 用假的 navigator 代替浏览器自带的，只提供 serviceWorker 这一项
function stubServiceWorker(serviceWorker: unknown) {
  vi.stubGlobal('navigator', serviceWorker === undefined ? {} : { serviceWorker })
}

describe('clearOfflineCaches', () => {
  it('有 Service Worker 在控制页面时，发 clear-caches 消息', () => {
    const postMessage = vi.fn()
    stubServiceWorker({ controller: { postMessage } })
    clearOfflineCaches()
    expect(postMessage).toHaveBeenCalledTimes(1)
    expect(postMessage).toHaveBeenCalledWith({ type: 'clear-caches' })
  })

  it('浏览器不支持 Service Worker 时不报错', () => {
    stubServiceWorker(undefined)
    expect(() => clearOfflineCaches()).not.toThrow()
  })

  it('Service Worker 还没有接管页面（controller 为空）时不报错', () => {
    stubServiceWorker({ controller: null })
    expect(() => clearOfflineCaches()).not.toThrow()
  })
})
