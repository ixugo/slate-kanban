import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { getSnapshot } from '@/lib/api/endpoints'
import type { BoardEvent } from '@/lib/api/types'
import { createQueryClient } from '@/lib/queryClient'
import { queryKeys } from '@/lib/queryKeys'
import { connectBoardStream, type StreamOptions } from '@/lib/stream'
import { getToken, setToken } from '@/lib/token'
import { makeSnapshot } from '@/test/factories'
import { REFRESH_COALESCE_MS, useBoardLive } from './useBoardLive'

vi.mock('@/lib/stream', () => ({ connectBoardStream: vi.fn() }))
vi.mock('@/lib/api/endpoints', () => ({ getSnapshot: vi.fn() }))

const connect = vi.mocked(connectBoardStream)
const snapshotApi = vi.mocked(getSnapshot)

// 每次连接的参数和对应的停止函数，按连接顺序记录
let connections: { opts: StreamOptions; stop: ReturnType<typeof vi.fn> }[] = []

const event = (type: string, id = 1): BoardEvent => ({
  id,
  board_id: 1,
  card_id: 0,
  type,
  actor_kind: 'human',
  actor: 'admin',
  payload: {},
  created_at: '2026-11-18T08:00:00Z',
})

// 快照序号传 null 表示快照还没取到
function setup(initialSeq: number | null = 5) {
  const queryClient: QueryClient = createQueryClient()
  const invalidate = vi.spyOn(queryClient, 'invalidateQueries')
  const onGone = vi.fn()
  const onStopped = vi.fn()
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
  const hook = renderHook(
    ({ seq }: { seq: number | undefined }) => useBoardLive({ boardId: 1, snapshotSeq: seq, onGone, onStopped }),
    { wrapper, initialProps: { seq: initialSeq ?? undefined } },
  )
  return { ...hook, queryClient, invalidate, onGone, onStopped }
}

const invalidatedKeys = (spy: { mock: { calls: unknown[][] } }) =>
  spy.mock.calls.map((call) => (call[0] as { queryKey: unknown[] }).queryKey)

describe('useBoardLive', () => {
  beforeEach(() => {
    connections = []
    connect.mockImplementation((opts) => {
      const stop = vi.fn()
      connections.push({ opts, stop })
      return stop
    })
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.clearAllMocks()
  })

  it('快照还没取到时不连接，取到之后用快照的序号连接', () => {
    const { rerender } = setup(null)
    expect(connections).toHaveLength(0)
    rerender({ seq: 7 })
    expect(connections).toHaveLength(1)
    expect(connections[0].opts).toMatchObject({ boardId: 1, after: 7 })
  })

  it('快照刷新（序号变大）不会重新连接', () => {
    const { rerender } = setup(5)
    rerender({ seq: 9 })
    rerender({ seq: 12 })
    expect(connections).toHaveLength(1)
  })

  it('连接时读取令牌的函数是实时的，登录状态变了立刻生效', () => {
    setup(5)
    setToken('t1')
    expect(connections[0].opts.getToken()).toBe('t1')
    setToken('t2')
    expect(connections[0].opts.getToken()).toBe('t2')
  })

  it('连接状态原样透传给页面', () => {
    const { result } = setup(5)
    expect(result.current.state).toBe('connecting')
    act(() => connections[0].opts.onState('live'))
    expect(result.current.state).toBe('live')
    act(() => connections[0].opts.onState('backoff'))
    expect(result.current.state).toBe('backoff')
  })

  it('一串事件合并成一次刷新：快照、卡片详情、活动流各失效一次', () => {
    vi.useFakeTimers()
    const { invalidate } = setup(5)
    act(() => {
      connections[0].opts.onEvent(event('card.moved', 6))
      connections[0].opts.onEvent(event('card.updated', 7))
      connections[0].opts.onEvent(event('comment.added', 8))
    })
    expect(invalidate).not.toHaveBeenCalled()
    act(() => vi.advanceTimersByTime(REFRESH_COALESCE_MS - 1))
    expect(invalidate).not.toHaveBeenCalled()
    act(() => vi.advanceTimersByTime(1))
    expect(invalidatedKeys(invalidate)).toEqual([queryKeys.board(1), queryKeys.card(), queryKeys.activity(1)])
    // 合并窗口结束后，下一个事件重新开一个窗口
    act(() => connections[0].opts.onEvent(event('card.moved', 9)))
    act(() => vi.advanceTimersByTime(REFRESH_COALESCE_MS))
    expect(invalidate).toHaveBeenCalledTimes(6)
  })

  it('看板信息类事件（board.*）马上让看板列表失效，其他事件不影响列表', () => {
    vi.useFakeTimers()
    const { invalidate } = setup(5)
    act(() => connections[0].opts.onEvent(event('card.created', 6)))
    expect(invalidatedKeys(invalidate)).not.toContainEqual(queryKeys.boards)
    act(() => connections[0].opts.onEvent(event('board.updated', 7)))
    expect(invalidatedKeys(invalidate)).toContainEqual(queryKeys.boards)
  })

  it('卸载时停止连接，并取消还没执行的刷新', () => {
    vi.useFakeTimers()
    const { unmount, invalidate } = setup(5)
    act(() => connections[0].opts.onEvent(event('card.moved', 6)))
    unmount()
    expect(connections[0].stop).toHaveBeenCalledTimes(1)
    act(() => vi.advanceTimersByTime(REFRESH_COALESCE_MS * 2))
    expect(invalidate).not.toHaveBeenCalled()
  })

  it('reset：重新取快照（即使缓存里的还很新），用新快照的序号重新连接，之后手动重试不再用它', async () => {
    snapshotApi.mockResolvedValue(makeSnapshot({ seq: 321 }))
    const { queryClient, result } = setup(5)
    queryClient.setQueryData(queryKeys.board(1), makeSnapshot({ seq: 5 }))
    act(() => connections[0].opts.onReset(300))
    await waitFor(() => expect(connections).toHaveLength(2))
    expect(snapshotApi).toHaveBeenCalledTimes(1)
    expect(connections[1].opts.after).toBe(321)
    expect(connections[0].stop).toHaveBeenCalledTimes(1)
    expect(queryClient.getQueryData<{ seq: number }>(queryKeys.board(1))?.seq).toBe(321)
    act(() => result.current.retry())
    expect(connections).toHaveLength(3)
    expect(connections[2].opts.after).toBe(5)
  })

  it('reset 之后重新取快照失败：通知页面停止，不会无限重连', async () => {
    snapshotApi.mockRejectedValue(new Error('boom'))
    const { onStopped } = setup(5)
    act(() => connections[0].opts.onReset(300))
    await waitFor(() => expect(onStopped).toHaveBeenCalledWith('boom'))
    expect(connections).toHaveLength(1)
  })

  it('令牌失效（unauthorized）：清掉令牌，页面会回到登录', () => {
    setToken('t1')
    const { onGone, onStopped } = setup(5)
    act(() => connections[0].opts.onFatal('unauthorized', 'HTTP 401'))
    expect(getToken()).toBe('')
    expect(onGone).not.toHaveBeenCalled()
    expect(onStopped).not.toHaveBeenCalled()
  })

  it('看板不存在（not_found）：通知页面离开，不清令牌', () => {
    setToken('t1')
    const { onGone, onStopped } = setup(5)
    act(() => connections[0].opts.onFatal('not_found', 'HTTP 404'))
    expect(onGone).toHaveBeenCalledTimes(1)
    expect(onStopped).not.toHaveBeenCalled()
    expect(getToken()).toBe('t1')
  })

  it('其他不可恢复的错误：通知页面停止并带上原因', () => {
    const { onGone, onStopped } = setup(5)
    act(() => connections[0].opts.onFatal('server_error', 'HTTP 400'))
    expect(onStopped).toHaveBeenCalledWith('HTTP 400')
    expect(onGone).not.toHaveBeenCalled()
  })

  it('手动重试：停掉旧连接并重新连接', () => {
    const { result } = setup(5)
    act(() => result.current.retry())
    expect(connections).toHaveLength(2)
    expect(connections[0].stop).toHaveBeenCalledTimes(1)
    expect(connections[1].opts.after).toBe(5)
  })
})
