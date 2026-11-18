import { afterEach, describe, expect, it, vi } from 'vitest'
import type { BoardEvent } from './api/types'
import { BACKOFF_MAX_MS, connectBoardStream, type StreamFailure, type StreamState } from './stream'

const encoder = new TextEncoder()

/** 一条可以由测试手动推送数据的响应流 */
interface FakeStream {
  response: Response
  push: (text: string) => void
  end: () => void
  /** 把 fetch 的取消信号接上：和真实浏览器一样，取消后读响应体会报错 */
  attach: (signal: AbortSignal | undefined) => void
}

// 造一条可控的 SSE 响应：测试决定什么时候推数据、什么时候结束。
// ignoreAbort 为真时取消信号不会让响应体报错，用来验证"连接没有真正断开时调用方也不再收到回调"。
function fakeStream(status = 200, ignoreAbort = false): FakeStream {
  let controller!: ReadableStreamDefaultController<Uint8Array>
  let closed = false
  const body = new ReadableStream<Uint8Array>({
    start(c) {
      controller = c
    },
  })
  return {
    response: new Response(status === 200 ? body : null, {
      status,
      headers: { 'Content-Type': 'text/event-stream' },
    }),
    push: (text) => {
      if (!closed) controller.enqueue(encoder.encode(text))
    },
    end: () => {
      closed = true
      controller.close()
    },
    attach: (signal) => {
      if (ignoreAbort) return
      signal?.addEventListener('abort', () => {
        if (closed) return
        closed = true
        controller.error(new DOMException('aborted', 'AbortError'))
      })
    },
  }
}

// 造一条 board 事件的 SSE 文本。
function boardMsg(id: number, type = 'card.moved'): string {
  const event: BoardEvent = {
    id,
    board_id: 1,
    card_id: 5,
    type,
    actor_kind: 'human',
    actor: 'admin',
    payload: {},
    created_at: '2026-11-18T00:00:00Z',
  }
  return `event: board\nid: ${id}\ndata: ${JSON.stringify(event)}\n\n`
}

const readyMsg = 'event: ready\ndata: {"latest":0}\n\n'

interface Harness {
  events: BoardEvent[]
  states: StreamState[]
  resets: number[]
  fatals: { kind: StreamFailure; message: string }[]
  delays: number[]
  urls: string[]
  headers: Record<string, string>[]
  signals: AbortSignal[]
  stop: () => void
}

// 启动一条流：fetchImpl 依次返回 responses 里的响应（或抛出其中的 Error），sleep 立刻返回并记录等待时间。
function start(
  responses: (FakeStream | Error)[],
  extra: { after?: number; token?: () => string; idleTimeoutMs?: number; realSleep?: boolean } = {},
): Harness {
  const h: Harness = {
    events: [],
    states: [],
    resets: [],
    fatals: [],
    delays: [],
    urls: [],
    headers: [],
    signals: [],
    stop: () => {},
  }
  let call = 0
  const fetchImpl = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    h.urls.push(String(input))
    h.headers.push((init?.headers ?? {}) as Record<string, string>)
    h.signals.push(init?.signal as AbortSignal)
    const next = responses[call++]
    if (next === undefined) return new Promise<Response>(() => {})
    if (next instanceof Error) return Promise.reject(next)
    next.attach(init?.signal as AbortSignal | undefined)
    return Promise.resolve(next.response)
  })
  h.stop = connectBoardStream({
    boardId: 1,
    after: extra.after ?? 0,
    getToken: extra.token ?? (() => 'tok'),
    onEvent: (e) => h.events.push(e),
    onState: (s) => h.states.push(s),
    onReset: (n) => h.resets.push(n),
    onFatal: (kind, message) => h.fatals.push({ kind, message }),
    fetchImpl: fetchImpl as unknown as typeof fetch,
    idleTimeoutMs: extra.idleTimeoutMs,
    sleep: extra.realSleep
      ? undefined
      : (ms) => {
          h.delays.push(ms)
          return Promise.resolve()
        },
  })
  return h
}

afterEach(() => {
  vi.useRealTimers()
})

describe('connectBoardStream', () => {
  it('带着令牌和起点连接，收到 ready 后进入 live，board 事件交给调用方', async () => {
    const s = fakeStream()
    const h = start([s], { after: 42 })
    await vi.waitFor(() => expect(h.urls).toHaveLength(1))
    expect(h.urls[0]).toBe('/api/v1/boards/1/stream?after=42')
    expect(h.headers[0].Authorization).toBe('Bearer tok')
    expect(h.headers[0].Accept).toBe('text/event-stream')

    s.push(readyMsg)
    await vi.waitFor(() => expect(h.states).toContain('live'))
    s.push(boardMsg(43))
    await vi.waitFor(() => expect(h.events.map((e) => e.id)).toEqual([43]))
    h.stop()
  })

  it('令牌不会出现在地址里', async () => {
    const h = start([fakeStream()], { token: () => 'secret-token' })
    await vi.waitFor(() => expect(h.urls).toHaveLength(1))
    expect(h.urls[0]).not.toContain('secret-token')
    h.stop()
  })

  it('一条消息被切成几块到达也能正确解析', async () => {
    const s = fakeStream()
    const h = start([s])
    await vi.waitFor(() => expect(h.urls).toHaveLength(1))
    const text = readyMsg + boardMsg(1)
    s.push(text.slice(0, 10))
    s.push(text.slice(10, 70))
    s.push(text.slice(70))
    await vi.waitFor(() => expect(h.events).toHaveLength(1))
    h.stop()
  })

  it('序号不比已有位置新的事件当作重复丢掉', async () => {
    const s = fakeStream()
    const h = start([s], { after: 10 })
    await vi.waitFor(() => expect(h.urls).toHaveLength(1))
    s.push(readyMsg + boardMsg(10) + boardMsg(11) + boardMsg(11) + boardMsg(9) + boardMsg(12))
    await vi.waitFor(() => expect(h.events.map((e) => e.id)).toEqual([11, 12]))
    h.stop()
  })

  it('心跳和不认识的事件被忽略', async () => {
    const s = fakeStream()
    const h = start([s])
    await vi.waitFor(() => expect(h.urls).toHaveLength(1))
    s.push(readyMsg + ': ping\n\nevent: weird\ndata: {}\n\n' + boardMsg(1))
    await vi.waitFor(() => expect(h.events).toHaveLength(1))
    expect(h.fatals).toEqual([])
    h.stop()
  })

  it('服务端到点结束后马上重连，起点是最后收到的序号，不退避', async () => {
    const first = fakeStream()
    const second = fakeStream()
    vi.useFakeTimers({ toFake: ['Date'] })
    const h = start([first, second])
    await vi.waitFor(() => expect(h.urls).toHaveLength(1))
    first.push(readyMsg + boardMsg(7))
    await vi.waitFor(() => expect(h.events).toHaveLength(1))
    vi.setSystemTime(Date.now() + 5_000)
    first.end()
    await vi.waitFor(() => expect(h.urls).toHaveLength(2))
    expect(h.urls[1]).toBe('/api/v1/boards/1/stream?after=7')
    expect(h.delays).toEqual([])
    h.stop()
  })

  it('刚连上就被关闭按失败处理，会退避', async () => {
    const first = fakeStream()
    const h = start([first, fakeStream()])
    await vi.waitFor(() => expect(h.urls).toHaveLength(1))
    first.push(readyMsg)
    first.end()
    await vi.waitFor(() => expect(h.urls).toHaveLength(2))
    expect(h.delays).toEqual([1000])
    h.stop()
  })

  it('网络错误按 1、2、4…秒退避，上限 15 秒', async () => {
    const errors = Array.from({ length: 7 }, () => new Error('boom'))
    const h = start([...errors, fakeStream()])
    await vi.waitFor(() => expect(h.urls).toHaveLength(8))
    expect(h.delays).toEqual([1000, 2000, 4000, 8000, BACKOFF_MAX_MS, BACKOFF_MAX_MS, BACKOFF_MAX_MS])
    h.stop()
  })

  it('连上并稳定一段时间之后，退避从头开始计', async () => {
    const errors = Array.from({ length: 3 }, () => new Error('boom'))
    const ok = fakeStream()
    vi.useFakeTimers({ toFake: ['Date'] })
    const h = start([...errors, ok, new Error('again'), fakeStream()])
    await vi.waitFor(() => expect(h.urls).toHaveLength(4))
    expect(h.delays).toEqual([1000, 2000, 4000])
    ok.push(readyMsg)
    await vi.waitFor(() => expect(h.states).toContain('live'))
    vi.setSystemTime(Date.now() + 5_000)
    ok.end()
    // 稳定过的连接被服务端结束：马上重连（没有等待），之后再失败又从 1 秒开始
    await vi.waitFor(() => expect(h.urls).toHaveLength(6))
    expect(h.delays).toEqual([1000, 2000, 4000, 1000])
    h.stop()
  })

  it('刚连上又断开的服务端不会让重连频率恢复到 1 秒，而是继续往上退避', async () => {
    const flaky = [fakeStream(), fakeStream(), fakeStream()]
    const h = start([...flaky, fakeStream()])
    for (const s of flaky) {
      await vi.waitFor(() => expect(h.states.at(-1)).toBe('connecting'))
      s.push(readyMsg)
      s.end()
    }
    await vi.waitFor(() => expect(h.urls).toHaveLength(4))
    expect(h.delays).toEqual([1000, 2000, 4000])
    h.stop()
  })

  it('401 不重连，报告 unauthorized 并停止', async () => {
    const h = start([fakeStream(401), fakeStream()])
    await vi.waitFor(() => expect(h.fatals).toHaveLength(1))
    expect(h.fatals[0].kind).toBe('unauthorized')
    expect(h.states.at(-1)).toBe('stopped')
    await new Promise((r) => setTimeout(r, 20))
    expect(h.urls).toHaveLength(1)
  })

  it('404 报告 not_found 并停止', async () => {
    const h = start([fakeStream(404), fakeStream()])
    await vi.waitFor(() => expect(h.fatals).toHaveLength(1))
    expect(h.fatals[0].kind).toBe('not_found')
    expect(h.urls).toHaveLength(1)
  })

  it('500 属于临时故障，退避后重试', async () => {
    const h = start([fakeStream(500), fakeStream()])
    await vi.waitFor(() => expect(h.urls).toHaveLength(2))
    expect(h.fatals).toEqual([])
    expect(h.delays).toEqual([1000])
    h.stop()
  })

  it('reset 通知调用方并停止，不再重连', async () => {
    const s = fakeStream()
    const h = start([s, fakeStream()])
    await vi.waitFor(() => expect(h.urls).toHaveLength(1))
    s.push(readyMsg + 'event: reset\ndata: {"latest":99}\n\n')
    await vi.waitFor(() => expect(h.resets).toEqual([99]))
    expect(h.states.at(-1)).toBe('stopped')
    await new Promise((r) => setTimeout(r, 20))
    expect(h.urls).toHaveLength(1)
  })

  it('error 事件报告服务端的原因并停止', async () => {
    const s = fakeStream()
    const h = start([s, fakeStream()])
    await vi.waitFor(() => expect(h.urls).toHaveLength(1))
    s.push(readyMsg + 'event: error\ndata: {"reason":"ErrBoardNotFound","msg":"看板不存在"}\n\n')
    await vi.waitFor(() => expect(h.fatals).toHaveLength(1))
    expect(h.fatals[0]).toEqual({ kind: 'server_error', message: 'ErrBoardNotFound' })
    await new Promise((r) => setTimeout(r, 20))
    expect(h.urls).toHaveLength(1)
  })

  it('事件内容不是合法 JSON 时停止并报告，而不是悄悄跳过', async () => {
    const s = fakeStream()
    const h = start([s])
    await vi.waitFor(() => expect(h.urls).toHaveLength(1))
    s.push(readyMsg + 'event: board\nid: 1\ndata: not-json\n\n')
    await vi.waitFor(() => expect(h.fatals).toHaveLength(1))
    expect(h.fatals[0].message).toBe('malformed event')
  })

  it.each(['null', '[]', '{}', '{"id":"7"}', '"text"'])('事件内容 %s 不是带数字序号的对象，停止并报告', async (data) => {
    const s = fakeStream()
    const h = start([s])
    await vi.waitFor(() => expect(h.urls).toHaveLength(1))
    s.push(readyMsg + `event: board\ndata: ${data}\n\n`)
    await vi.waitFor(() => expect(h.fatals).toHaveLength(1))
    expect(h.fatals[0].message).toBe('malformed event')
    expect(h.events).toEqual([])
  })

  it('长时间收不到任何数据（半开连接）就断开重连', async () => {
    const first = fakeStream()
    const second = fakeStream()
    const h = start([first, second], { idleTimeoutMs: 40, realSleep: false })
    await vi.waitFor(() => expect(h.urls).toHaveLength(1))
    first.push(readyMsg)
    await vi.waitFor(() => expect(h.states).toContain('live'))
    await vi.waitFor(() => expect(h.urls).toHaveLength(2), { timeout: 1000 })
    expect(h.signals[0].aborted).toBe(true)
    h.stop()
  })

  it('心跳能让半开检测重新计时', async () => {
    const s = fakeStream()
    const h = start([s, fakeStream()], { idleTimeoutMs: 80 })
    await vi.waitFor(() => expect(h.urls).toHaveLength(1))
    s.push(readyMsg)
    for (let i = 0; i < 4; i++) {
      await new Promise((r) => setTimeout(r, 40))
      s.push(': ping\n\n')
    }
    expect(h.urls).toHaveLength(1)
    h.stop()
  })

  it('stop 之后取消连接，不再回调也不再重连', async () => {
    const s = fakeStream()
    const h = start([s, fakeStream()])
    await vi.waitFor(() => expect(h.urls).toHaveLength(1))
    s.push(readyMsg)
    await vi.waitFor(() => expect(h.states).toContain('live'))
    h.stop()
    expect(h.signals[0].aborted).toBe(true)
    const before = h.events.length
    s.push(boardMsg(1))
    await new Promise((r) => setTimeout(r, 20))
    expect(h.events.length).toBe(before)
    expect(h.urls).toHaveLength(1)
  })

  it('连接没有真正断开时，stop 之后读到的数据也不会回调', async () => {
    const s = fakeStream(200, true)
    const h = start([s])
    await vi.waitFor(() => expect(h.urls).toHaveLength(1))
    s.push(readyMsg)
    await vi.waitFor(() => expect(h.states).toContain('live'))
    h.stop()
    s.push(boardMsg(1))
    await new Promise((r) => setTimeout(r, 20))
    expect(h.events).toEqual([])
  })

  it('每次重连都重新读取令牌', async () => {
    const tokens = ['first', 'second']
    const first = fakeStream()
    const h = start([first, fakeStream()], { token: () => tokens.shift() ?? 'last' })
    await vi.waitFor(() => expect(h.urls).toHaveLength(1))
    first.push(readyMsg)
    first.end()
    await vi.waitFor(() => expect(h.urls).toHaveLength(2))
    expect(h.headers.map((x) => x.Authorization)).toEqual(['Bearer first', 'Bearer second'])
    h.stop()
  })

  it('调用方的事件处理函数抛错不会断流，后面的事件照常送达', async () => {
    const s = fakeStream()
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
    const events: number[] = []
    const stop = connectBoardStream({
      boardId: 1,
      after: 0,
      getToken: () => 't',
      onEvent: (e) => {
        events.push(e.id)
        if (e.id === 1) throw new Error('handler bug')
      },
      onState: () => {},
      onReset: () => {},
      onFatal: () => {},
      fetchImpl: (() => Promise.resolve(s.response)) as unknown as typeof fetch,
    })
    s.push(readyMsg + boardMsg(1) + boardMsg(2))
    await vi.waitFor(() => expect(events).toEqual([1, 2]))
    expect(errorSpy).toHaveBeenCalledTimes(1)
    stop()
    errorSpy.mockRestore()
  })
})
