import { renderHook, act } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { makeColumn } from '@/test/factories'
import {
  columnCollapseKey,
  defaultCollapsedForColumn,
  useCollapsedColumns,
} from './useCollapsedColumns'

const doneCol = makeColumn({ id: 3, name: '完成', is_done: true })
const todoCol = makeColumn({ id: 1, name: '待办' })
const cols = [todoCol, makeColumn({ id: 2, name: '进行中' }), doneCol]

beforeEach(() => localStorage.clear())
afterEach(() => vi.restoreAllMocks())

describe('defaultCollapsedForColumn', () => {
  it('完成列默认收起', () => {
    expect(defaultCollapsedForColumn(doneCol)).toBe(true)
    expect(defaultCollapsedForColumn(todoCol)).toBe(false)
  })
})

describe('useCollapsedColumns', () => {
  it('无偏好时完成列收起、其他列展开', () => {
    const { result } = renderHook(() => useCollapsedColumns(1, cols))
    expect(result.current.isCollapsed(1)).toBe(false)
    expect(result.current.isCollapsed(3)).toBe(true)
  })

  it('读取本地已存偏好', () => {
    localStorage.setItem(columnCollapseKey(1, 1), '1')
    localStorage.setItem(columnCollapseKey(1, 3), '0')
    const { result } = renderHook(() => useCollapsedColumns(1, cols))
    expect(result.current.isCollapsed(1)).toBe(true)
    expect(result.current.isCollapsed(3)).toBe(false)
  })

  it('切换后写入 localStorage', () => {
    const { result } = renderHook(() => useCollapsedColumns(1, cols))
    act(() => result.current.collapseColumn(1))
    expect(localStorage.getItem(columnCollapseKey(1, 1))).toBe('1')
    act(() => result.current.expandColumn(3))
    expect(localStorage.getItem(columnCollapseKey(1, 3))).toBe('0')
  })

  it('不同看板互不影响', () => {
    const { result: a } = renderHook(() => useCollapsedColumns(1, cols))
    act(() => a.current.collapseColumn(1))
    const { result: b } = renderHook(() => useCollapsedColumns(2, cols))
    expect(b.current.isCollapsed(1)).toBe(false)
    expect(localStorage.getItem(columnCollapseKey(2, 1))).toBeNull()
  })

  it('localStorage 抛错时退回内存仍能切换', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('denied')
    })
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('quota')
    })
    const { result } = renderHook(() => useCollapsedColumns(1, cols))
    expect(result.current.isCollapsed(1)).toBe(false)
    act(() => result.current.collapseColumn(1))
    expect(result.current.isCollapsed(1)).toBe(true)
    expect(warn).toHaveBeenCalled()
  })
})