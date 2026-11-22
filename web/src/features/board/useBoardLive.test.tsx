import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '@/lib/api/client'
import * as endpoints from '@/lib/api/endpoints'
import { queryKeys } from '@/lib/queryKeys'
import { makeSnapshot } from '@/test/factories'
import { RECONNECT_DELAY_MS, REFRESH_COALESCE_MS, useBoardLive } from './useBoardLive'

// 假的 EventSource：记录创建和关闭，并能手动触发事件
class FakeEventSource {
  static instances: FakeEventSource[] = []
  static readonly CLOSED = 2
  readonly url: string
  readyState = 1
  closed = false
  private listeners = new Map<string, Array<(e: Event) => void>>()

  constructor(url: string) {
    this.url = url
    FakeEventSource.instances.push(this)
  }

  addEventListener(type: string, fn: (e: Event) => void) {
    this.listeners.set(type, [...(this.listeners.get(type) ?? []), fn])
  }

  close() {
    this.closed = true
    this.readyState = FakeEventSource.CLOSED
  }

  // 触发一个带数据的服务端事件
  emit(type: string, data?: unknown) {
    const event = new MessageEvent(type, { data: data === undefined ? undefined : JSON.stringify(data) })
    this.listeners.get(type)?.forEach((fn) => fn(event))
  }

  // 触发浏览器的连接错误；closed 为真表示浏览器放弃了重连
  fail(closed: boolean) {
    this.readyState = closed ? FakeEventSource.CLOSED : 0
    this.listeners.get('error')?.forEach((fn) => fn(new Event('error')))
  }
}

const last = () => FakeEventSource.instances[FakeEventSource.instances.length - 1]

function setup(boardId: number | null = 1) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  queryClient.setQueryData(queryKeys.board(1), makeSnapshot({ id: 1, seq: 5 }))
  const invalidate = vi.spyOn(queryClient, 'invalidateQueries')
  const onGone = vi.fn()
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
  const hook = renderHook(() => useBoardLive({ boardId, onGone }), { wrapper })
  return { ...hook, queryClient, invalidate, onGone }
}

// 被失效的查询键里有没有这个键
const invalidated = (spy: { mock: { calls: unknown[][] } }, key: readonly unknown[]) =>
  spy.mock.calls.some((c) => JSON.stringify((c[0] as { queryKey: unknown }).queryKey) === JSON.stringify(key))

beforeEach(() => {
  vi.useFakeTimers()
  FakeEventSource.instances = []
  vi.stubGlobal('EventSource', FakeEventSource)
})

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('useBoardLive', () => {
  it('连接到这块看板的实时流，收到 ready 后状态变成已连接', () => {
    const { result } = setup(1)
    expect(last().url).toBe('/api/v1/boards/1/stream')
    expect(result.current.state).toBe('connecting')
    act(() => last().emit('ready', { latest: 5 }))
    expect(result.current.state).toBe('live')
  })

  it('没有看板编号（地址不合法）时不连接', () => {
    setup(null)
    expect(FakeEventSource.instances).toHaveLength(0)
  })

  it('一次操作带来的多条事件合并成一次刷新，并同时刷新任务详情', () => {
    const { invalidate } = setup(1)
    act(() => {
      last().emit('board', { type: 'card.moved' })
      last().emit('board', { type: 'card.updated' })
      last().emit('board', { type: 'card.updated' })
    })
    expect(invalidate).not.toHaveBeenCalled()
    act(() => vi.advanceTimersByTime(REFRESH_COALESCE_MS))
    const snapshotCalls = invalidate.mock.calls.filter((c) => JSON.stringify(c[0]?.queryKey) === JSON.stringify(queryKeys.board(1)))
    expect(snapshotCalls).toHaveLength(1)
    expect(invalidated(invalidate, queryKeys.card())).toBe(true)
  })

  it('看板本身的事件（board.*）同时刷新看板列表', () => {
    const { invalidate } = setup(1)
    act(() => last().emit('board', { type: 'board.updated' }))
    expect(invalidated(invalidate, queryKeys.boards)).toBe(true)
  })

  it('任务事件不刷新看板列表', () => {
    const { invalidate } = setup(1)
    act(() => last().emit('board', { type: 'card.created' }))
    expect(invalidated(invalidate, queryKeys.boards)).toBe(false)
  })

  it('事件数据不是合法 JSON 时照常刷新，不丢更新', () => {
    const { invalidate } = setup(1)
    act(() => {
      const bad = new MessageEvent('board', { data: 'not json' })
      ;(last() as unknown as { listeners: Map<string, Array<(e: Event) => void>> }).listeners.get('board')?.forEach((fn) => fn(bad))
      vi.advanceTimersByTime(REFRESH_COALESCE_MS)
    })
    expect(invalidated(invalidate, queryKeys.board(1))).toBe(true)
  })

  it('ready 带的序号比缓存里的快照新，说明取快照到连上之间有更新，要补刷一次', () => {
    const { invalidate } = setup(1)
    act(() => {
      last().emit('ready', { latest: 9 })
      vi.advanceTimersByTime(REFRESH_COALESCE_MS)
    })
    expect(invalidated(invalidate, queryKeys.board(1))).toBe(true)
  })

  it('ready 的序号和快照一样新，不多刷新', () => {
    const { invalidate } = setup(1)
    act(() => {
      last().emit('ready', { latest: 5 })
      vi.advanceTimersByTime(REFRESH_COALESCE_MS * 2)
    })
    expect(invalidate).not.toHaveBeenCalled()
  })

  it('reset：关闭旧连接，刷新，并新建一个连接', () => {
    const { invalidate } = setup(1)
    const first = last()
    act(() => first.emit('reset', { latest: 3 }))
    expect(first.closed).toBe(true)
    expect(FakeEventSource.instances).toHaveLength(2)
    // 立刻刷新，不等合并计时，因为旧连接清理时会丢掉未触发的计时器
    expect(invalidated(invalidate, queryKeys.board(1))).toBe(true)
  })

  it('浏览器自己在重连时，状态显示为连接中，不新建连接', () => {
    const { result } = setup(1)
    act(() => last().emit('ready', { latest: 5 }))
    act(() => last().fail(false))
    expect(result.current.state).toBe('connecting')
    expect(FakeEventSource.instances).toHaveLength(1)
  })

  it('浏览器放弃重连、看板已被删（快照 404）：通知看板已不存在，不再重连', async () => {
    vi.spyOn(endpoints, 'getSnapshot').mockRejectedValue(new ApiError(404, 'ErrBoardNotFound', 'gone'))
    const { result, onGone } = setup(1)
    await act(async () => last().fail(true))
    expect(result.current.state).toBe('offline')
    expect(onGone).toHaveBeenCalledTimes(1)
    await act(async () => vi.advanceTimersByTime(RECONNECT_DELAY_MS * 2))
    expect(FakeEventSource.instances).toHaveLength(1)
  })

  it('浏览器放弃重连、看板还在（比如服务重启）：过一会儿自己重新连接', async () => {
    vi.spyOn(endpoints, 'getSnapshot').mockResolvedValue(makeSnapshot({ id: 1, seq: 8 }))
    const { onGone } = setup(1)
    await act(async () => last().fail(true))
    expect(onGone).not.toHaveBeenCalled()
    await act(async () => vi.advanceTimersByTime(RECONNECT_DELAY_MS))
    expect(FakeEventSource.instances).toHaveLength(2)
  })

  it('服务端发来 error 事件：关闭连接并确认看板是否还在', async () => {
    vi.spyOn(endpoints, 'getSnapshot').mockRejectedValue(new ApiError(404, 'ErrBoardNotFound', 'gone'))
    const { result, onGone } = setup(1)
    const first = last()
    await act(async () => first.emit('error', { reason: 'ErrBoardNotFound' }))
    expect(first.closed).toBe(true)
    expect(result.current.state).toBe('offline')
    expect(onGone).toHaveBeenCalledTimes(1)
  })

  it('卸载时关闭连接并取消待执行的刷新', () => {
    const { unmount, invalidate } = setup(1)
    act(() => last().emit('board', { type: 'card.updated' }))
    unmount()
    expect(last().closed).toBe(true)
    act(() => vi.advanceTimersByTime(REFRESH_COALESCE_MS * 2))
    expect(invalidate).not.toHaveBeenCalled()
  })

  it('手动重试会新建连接', () => {
    const { result } = setup(1)
    act(() => result.current.retry())
    expect(FakeEventSource.instances).toHaveLength(2)
    expect(FakeEventSource.instances[0].closed).toBe(true)
  })

  it('切换到另一块看板：旧连接关闭，新连接指向新看板', () => {
    const queryClient = new QueryClient()
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    )
    const { rerender } = renderHook(({ id }: { id: number }) => useBoardLive({ boardId: id, onGone: () => {} }), {
      wrapper,
      initialProps: { id: 1 },
    })
    const first = last()
    rerender({ id: 2 })
    expect(first.closed).toBe(true)
    expect(last().url).toBe('/api/v1/boards/2/stream')
  })
})
