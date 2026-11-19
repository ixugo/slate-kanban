import { act, renderHook } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { EMPTY_FILTERS, toggleTag } from './filters'
import { useBoardFilters } from './useBoardFilters'

describe('useBoardFilters', () => {
  it('设置后能读到，清空后回到什么都没选', () => {
    const { result } = renderHook(() => useBoardFilters(1))
    expect(result.current.filters).toBe(EMPTY_FILTERS)

    act(() => result.current.setFilters(toggleTag(EMPTY_FILTERS, 'bug')))
    expect(result.current.filters.tags).toEqual(['bug'])

    act(() => result.current.clear())
    expect(result.current.filters).toEqual(EMPTY_FILTERS)
  })

  it('换到另一块看板时是什么都没选，不把上一块看板的列和标签带过去；在那块看板上设置后，上一块的条件被替换', () => {
    const { result, rerender } = renderHook(({ id }) => useBoardFilters(id), { initialProps: { id: 1 } })
    act(() => result.current.setFilters(toggleTag(EMPTY_FILTERS, 'bug')))

    rerender({ id: 2 })
    expect(result.current.filters).toEqual(EMPTY_FILTERS)

    act(() => result.current.setFilters(toggleTag(EMPTY_FILTERS, 'ui')))
    expect(result.current.filters.tags).toEqual(['ui'])

    rerender({ id: 1 })
    expect(result.current.filters).toEqual(EMPTY_FILTERS)
  })
})
