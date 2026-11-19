import { act, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useOnlineStatus } from './useOnlineStatus'

describe('useOnlineStatus', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('初值跟随 navigator.onLine', () => {
    vi.stubGlobal('navigator', { onLine: false })
    const { result } = renderHook(() => useOnlineStatus())
    expect(result.current).toBe(false)
  })

  it('响应 offline 与 online 事件', () => {
    vi.stubGlobal('navigator', { onLine: true })
    const { result } = renderHook(() => useOnlineStatus())
    act(() => {
      window.dispatchEvent(new Event('offline'))
    })
    expect(result.current).toBe(false)
    act(() => {
      window.dispatchEvent(new Event('online'))
    })
    expect(result.current).toBe(true)
  })

  it('卸载后不再更新', () => {
    vi.stubGlobal('navigator', { onLine: true })
    const addSpy = vi.spyOn(window, 'addEventListener')
    const removeSpy = vi.spyOn(window, 'removeEventListener')
    const { unmount } = renderHook(() => useOnlineStatus())
    expect(addSpy).toHaveBeenCalledWith('online', expect.any(Function))
    expect(addSpy).toHaveBeenCalledWith('offline', expect.any(Function))
    unmount()
    expect(removeSpy).toHaveBeenCalledWith('online', expect.any(Function))
    expect(removeSpy).toHaveBeenCalledWith('offline', expect.any(Function))
  })
})