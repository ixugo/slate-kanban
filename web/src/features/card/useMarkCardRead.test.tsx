import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, renderHook, waitFor } from '@testing-library/react'
import { QueryClientProvider } from '@tanstack/react-query'
import type { Snapshot } from '@/lib/api/types'
import { createQueryClient } from '@/lib/queryClient'
import { queryKeys } from '@/lib/queryKeys'
import { makeSnapshot, makeCard } from '@/test/factories'
import { installFakeApi } from '@/test/fakeApi'
import { startInflightQuery } from '@/test/inflightQuery'
import { useMarkCardRead } from './useMarkCardRead'

const toastError = vi.fn()
vi.mock('@/lib/useErrorToast', () => ({ useErrorToast: () => toastError }))

const READ_ROUTE = 'POST /api/v1/cards/:id/read'

function wrapper(client: ReturnType<typeof createQueryClient>) {
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  )
}

describe('useMarkCardRead', () => {
  afterEach(() => vi.clearAllMocks())

  it('乐观清零并调用已读接口', async () => {
    const api = installFakeApi({
      'POST /api/v1/cards/:id/read': {},
    })
    const client = createQueryClient()
    const snap = makeSnapshot({
      cards: [makeCard({ id: 9, unread_count: 2 })],
    })
    client.setQueryData(queryKeys.board(1), snap)
    const { result } = renderHook(() => useMarkCardRead(1), { wrapper: wrapper(client) })
    result.current.tryMark(9)
    await waitFor(() => expect(api.calls.some((c) => c.path.endsWith('/cards/9/read'))).toBe(true))
    const cached = client.getQueryData<typeof snap>(queryKeys.board(1))
    expect(cached?.cards[0].unread_count).toBe(0)
  })

  // 打开有未读的任务时，详情里的活动流和已读请求几乎同时发出；
  // 乐观清零只该中止整板快照，不能把同一看板键下刚发出的活动流请求一起中止，
  // 否则时间线只剩评论，领取、换列这些事件悄悄缺失，而且没有任何东西会重新读取。
  it('乐观清零不会中止正在加载的任务活动流', async () => {
    const api = installFakeApi({ [READ_ROUTE]: {} })
    const client = createQueryClient()
    client.setQueryData(queryKeys.board(1), makeSnapshot({ cards: [makeCard({ id: 9, unread_count: 2 })] }))
    const activity = startInflightQuery(client, queryKeys.cardActivity(1, 9))

    const { result } = renderHook(() => useMarkCardRead(1), { wrapper: wrapper(client) })
    result.current.tryMark(9)
    await waitFor(() => expect(api.calls.some((c) => c.path.endsWith('/cards/9/read'))).toBe(true))

    expect(activity.aborted()).toBe(false)
    await expect(activity.result).resolves.toEqual({ ok: true })
  })

  // 详情弹窗每次重新渲染都会再检查一遍未读；乐观清零是异步的，
  // 已读请求还在路上时本地快照仍显示未读，不去重就会对同一张任务连发好几条一样的请求。
  it('同一张任务的已读请求还在途中时，重复检查不会再发请求', async () => {
    const api = installFakeApi({ [READ_ROUTE]: {} })
    const client = createQueryClient()
    client.setQueryData(queryKeys.board(1), makeSnapshot({ cards: [makeCard({ id: 9, unread_count: 2 })] }))
    const { result } = renderHook(() => useMarkCardRead(1), { wrapper: wrapper(client) })

    act(() => {
      result.current.tryMark(9)
      result.current.tryMark(9)
      result.current.tryMark(9)
    })
    await waitFor(() => expect(client.isMutating()).toBe(0))

    expect(api.count(READ_ROUTE)).toBe(1)
  })

  // 去重只覆盖请求在途的这段时间；落地之后详情还开着、又来了新评论，要能再标记一次。
  it('已读请求落地后又出现新的未读，会再标记一次', async () => {
    const api = installFakeApi({ [READ_ROUTE]: {} })
    const client = createQueryClient()
    client.setQueryData(queryKeys.board(1), makeSnapshot({ cards: [makeCard({ id: 9, unread_count: 2 })] }))
    const { result } = renderHook(() => useMarkCardRead(1), { wrapper: wrapper(client) })

    act(() => result.current.tryMark(9))
    await waitFor(() => expect(client.isMutating()).toBe(0))
    expect(api.count(READ_ROUTE)).toBe(1)

    client.setQueryData(queryKeys.board(1), makeSnapshot({ cards: [makeCard({ id: 9, unread_count: 1 })] }))
    act(() => result.current.tryMark(9))
    await waitFor(() => expect(api.count(READ_ROUTE)).toBe(2))
  })

  // 请求失败时快照会恢复成未读，弹窗随即重新渲染再检查一次；
  // 如果每次都重发，服务端持续出错时就会没完没了地重试并反复弹错误，重新打开详情才应当再试。
  it('已读请求失败后，重新渲染的检查不会无限重试', async () => {
    const api = installFakeApi({ [READ_ROUTE]: () => ({ status: 500, body: { reason: 'boom', msg: 'boom' } }) })
    const client = createQueryClient()
    client.setQueryData(queryKeys.board(1), makeSnapshot({ cards: [makeCard({ id: 9, unread_count: 2 })] }))
    const { result } = renderHook(() => useMarkCardRead(1), { wrapper: wrapper(client) })

    act(() => result.current.tryMark(9))
    await waitFor(() => expect(client.isMutating()).toBe(0))
    expect(toastError).toHaveBeenCalledTimes(1)
    expect(client.getQueryData<Snapshot>(queryKeys.board(1))?.cards[0].unread_count).toBe(2)

    act(() => result.current.tryMark(9))
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0))
    })
    expect(api.count(READ_ROUTE)).toBe(1)
  })
})
