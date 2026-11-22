import { QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { moveCard } from '@/lib/api/endpoints'
import type { Snapshot } from '@/lib/api/types'
import { createQueryClient } from '@/lib/queryClient'
import { queryKeys } from '@/lib/queryKeys'
import { makeCard, makeSnapshot } from '@/test/factories'
import { useMoveCard } from './useMoveCard'

vi.mock('@/lib/api/endpoints', () => ({ moveCard: vi.fn() }))

const toastError = vi.fn()
vi.mock('@/lib/useErrorToast', () => ({ useErrorToast: () => toastError }))

const moveApi = vi.mocked(moveCard)

const snapshot = makeSnapshot({
  cards: [
    makeCard({ id: 1, column_id: 1, position: 1024 }),
    makeCard({ id: 2, column_id: 1, position: 2048 }),
    makeCard({ id: 3, column_id: 2, position: 1024 }),
  ],
})

function setup(withCache = true) {
  const queryClient = createQueryClient()
  if (withCache) queryClient.setQueryData(queryKeys.board(1), snapshot)
  const invalidate = vi.spyOn(queryClient, 'invalidateQueries')
  const cancel = vi.spyOn(queryClient, 'cancelQueries')
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
  const hook = renderHook(() => useMoveCard(1), { wrapper })
  const cached = () => queryClient.getQueryData<Snapshot>(queryKeys.board(1))
  const columnOf = (id: number) => cached()?.cards.find((c) => c.id === id)?.column_id
  return { ...hook, queryClient, invalidate, cancel, cached, columnOf }
}

describe('useMoveCard', () => {
  afterEach(() => vi.clearAllMocks())

  it('请求还没返回时，缓存里已经是移动后的样子；请求参数是目标列和相邻任务', async () => {
    let finish: (v: never) => void = () => {}
    moveApi.mockReturnValue(new Promise((resolve) => (finish = resolve as (v: never) => void)))
    const { result, columnOf } = setup()
    act(() => result.current.mutate({ cardId: 1, plan: { columnId: 2, beforeId: 3 } }))
    await waitFor(() => expect(columnOf(1)).toBe(2))
    expect(moveApi).toHaveBeenCalledWith(1, { column_id: 2, before_id: 3, after_id: undefined })
    await act(async () => finish({} as never))
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
  })

  it('成功后让快照和任务详情重新取一次，不弹错误', async () => {
    moveApi.mockResolvedValue(makeCard({ id: 1 }))
    const { result, invalidate } = setup()
    await act(async () => result.current.mutate({ cardId: 1, plan: { columnId: 2 } }))
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    const keys = invalidate.mock.calls.map((c) => (c[0] as { queryKey: unknown[] }).queryKey)
    expect(keys).toEqual([queryKeys.board(1), queryKeys.card()])
    expect(toastError).not.toHaveBeenCalled()
  })

  it('失败后恢复到拖动之前的快照，提示错误，并重新取一次确认服务端状态', async () => {
    const err = new Error('任务被领取')
    moveApi.mockRejectedValue(err)
    const { result, columnOf, invalidate, cached } = setup()
    await act(async () => result.current.mutate({ cardId: 1, plan: { columnId: 2 } }))
    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(cached()).toEqual(snapshot)
    expect(columnOf(1)).toBe(1)
    expect(toastError).toHaveBeenCalledWith(err)
    expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.board(1) })
  })

  it('先取消正在飞的快照请求，免得它回来盖掉乐观结果', async () => {
    moveApi.mockResolvedValue(makeCard({ id: 1 }))
    const { result, cancel } = setup()
    await act(async () => result.current.mutate({ cardId: 1, plan: { columnId: 2 } }))
    expect(cancel).toHaveBeenCalledWith({ queryKey: queryKeys.board(1) })
  })

  it('缓存里还没有快照时不会崩溃，照常发请求', async () => {
    moveApi.mockResolvedValue(makeCard({ id: 1 }))
    const { result, cached } = setup(false)
    await act(async () => result.current.mutate({ cardId: 1, plan: { columnId: 2 } }))
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(cached()).toBeUndefined()
    expect(moveApi).toHaveBeenCalledTimes(1)
  })
})
