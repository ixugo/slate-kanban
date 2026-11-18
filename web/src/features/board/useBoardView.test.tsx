import { renderHook, act } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  boardViewStorageKey,
  persistBoardViewMode,
  readBoardViewMode,
  useBoardViewMode,
} from './useBoardView'

afterEach(() => {
  localStorage.clear()
  vi.unstubAllGlobals()
})

describe('useBoardViewMode', () => {
  it('默认是看板视图', () => {
    const { result } = renderHook(() => useBoardViewMode(1))
    expect(result.current.mode).toBe('board')
  })

  it('可写入并读出 localStorage', () => {
    const { result } = renderHook(() => useBoardViewMode(1))
    act(() => result.current.setMode('list'))
    expect(localStorage.getItem(boardViewStorageKey(1))).toBe('list')
    expect(readBoardViewMode(1)).toBe('list')
  })

  it('不同看板互不影响', () => {
    persistBoardViewMode(1, 'list')
    persistBoardViewMode(2, 'board')
    expect(readBoardViewMode(1)).toBe('list')
    expect(readBoardViewMode(2)).toBe('board')
  })

  it('切换到另一个看板时按那个看板的偏好显示', () => {
    persistBoardViewMode(1, 'board')
    persistBoardViewMode(2, 'list')
    const { result, rerender } = renderHook(({ id }) => useBoardViewMode(id), { initialProps: { id: 1 } })
    expect(result.current.mode).toBe('board')
    rerender({ id: 2 })
    expect(result.current.mode).toBe('list')
    rerender({ id: 1 })
    expect(result.current.mode).toBe('board')
  })

  it('在新看板里切换只改新看板的偏好', () => {
    const { result, rerender } = renderHook(({ id }) => useBoardViewMode(id), { initialProps: { id: 1 } })
    rerender({ id: 2 })
    act(() => result.current.setMode('list'))
    expect(readBoardViewMode(2)).toBe('list')
    expect(readBoardViewMode(1)).toBe('board')
  })

  it('存储失败时仍能在本页记住选择', async () => {
    const store: Record<string, string> = {}
    vi.stubGlobal('localStorage', {
      getItem: (k: string) => store[k] ?? null,
      setItem: () => {
        throw new Error('quota')
      },
      removeItem: vi.fn(),
      clear: vi.fn(),
      length: 0,
      key: () => null,
    })
    const fresh = await import('./useBoardView')
    const { result } = renderHook(() => fresh.useBoardViewMode(7))
    act(() => result.current.setMode('list'))
    expect(result.current.mode).toBe('list')
    expect(fresh.readBoardViewMode(7)).toBe('list')
  })
})