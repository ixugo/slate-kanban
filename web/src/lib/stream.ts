// 看板实时流的客户端：用 fetch 读 SSE，断线自动带着最后收到的事件序号重连。
// 协议见 .cursor/scopes/api.rest.go.md 的"实时流"：ready、board、reset、error 四种事件，加每 20 秒一次的 `: ping` 心跳。
// 令牌只放在 Authorization 请求头里，不进 URL。

import { API_PREFIX, authHeaders } from './api/client'
import type { BoardEvent } from './api/types'
import { SseParser, type SseMessage } from './sse'

/** 连接状态：连接中、已连上、等待重连、已停止 */
export type StreamState = 'connecting' | 'live' | 'backoff' | 'stopped'

/** 不会再自动重连的失败原因 */
export type StreamFailure = 'unauthorized' | 'not_found' | 'server_error'

/** 多久没有收到任何数据就认为连接半开：服务端每 20 秒发一次心跳，留出两倍多的余量 */
export const IDLE_TIMEOUT_MS = 45_000
/** 重连的退避起点和上限 */
export const BACKOFF_BASE_MS = 1_000
export const BACKOFF_MAX_MS = 15_000
/** 连上之后至少撑过这么久才算"健康"：健康的连接结束后马上重连并重新计退避；撑不到的按失败处理，避免服务端刚连上就关闭时疯狂重连 */
const MIN_LIVE_MS = 1_000

export interface StreamOptions {
  boardId: number
  /** 起点：只接收序号大于它的事件，一般是快照里的 seq */
  after: number
  /** 每次（重）连时读取令牌，登录状态变了能马上生效 */
  getToken: () => string
  /** 收到一条看板事件 */
  onEvent: (event: BoardEvent) => void
  onState: (state: StreamState) => void
  /** 服务端说续传的起点已经不可用：调用方要重新取快照，再用新序号重新开始。收到后连接已经停止 */
  onReset: (latest: number) => void
  /** 不会再重连的失败；收到后连接已经停止 */
  onFatal: (kind: StreamFailure, message: string) => void
  /** 下面三项只给测试注入，生产用默认值 */
  fetchImpl?: typeof fetch
  idleTimeoutMs?: number
  sleep?: (ms: number, signal: AbortSignal) => Promise<void>
}

/** 一次连接结束的结果，决定外层循环是重连、退避还是停止 */
type Outcome =
  | { kind: 'ended' }
  | { kind: 'failed' }
  | { kind: 'reset' }
  | { kind: 'fatal' }
  | { kind: 'aborted' }

/** 处理完一条消息的结果：null 表示继续读，'ready' 表示连接成功，其余是需要结束这次连接的结果 */
type Verdict = null | 'ready' | Outcome

// 等待指定毫秒；收到取消信号就提前结束，不抛错。
function defaultSleep(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    const timer = setTimeout(resolve, ms)
    signal.addEventListener(
      'abort',
      () => {
        clearTimeout(timer)
        resolve()
      },
      { once: true },
    )
  })
}

// 从 reset 事件的数据里取出服务端最新的序号，格式不对返回 0（调用方反正要重新取快照）。
function parseLatest(data: string): number {
  try {
    const parsed = JSON.parse(data) as { latest?: unknown }
    return typeof parsed.latest === 'number' ? parsed.latest : 0
  } catch {
    return 0
  }
}

// 从 error 事件的数据里取出服务端给的提示文字，取不到就用兜底文案。
function parseErrorMessage(data: string): string {
  try {
    const parsed = JSON.parse(data) as { msg?: unknown; reason?: unknown }
    if (typeof parsed.reason === 'string') return parsed.reason
    if (typeof parsed.msg === 'string') return parsed.msg
  } catch {
    // 数据不是 JSON，用下面的兜底文案
  }
  return 'stream error'
}

// 把 board 事件的数据解析成事件对象；不是 JSON、不是对象、没有数字序号都返回 null。
function parseBoardEvent(data: string): BoardEvent | null {
  try {
    const parsed = JSON.parse(data) as { id?: unknown } | null
    return typeof parsed?.id === 'number' ? (parsed as BoardEvent) : null
  } catch {
    return null
  }
}

/**
 * 一条实时流的会话：记着续传的位置和退避的次数，负责"连接、读取、遇到问题重连"的整个循环。
 * 用类是因为这些状态要在连接的多个步骤之间共享，拆成一堆带很多参数的函数反而难读。
 */
class StreamSession {
  private cursor: number
  private attempt = 0
  /** 当前这次连接收到 ready 的时刻，0 表示这次还没连上 */
  private liveSince = 0
  private readonly opts: StreamOptions
  private readonly signal: AbortSignal
  private readonly fetchFn: typeof fetch
  private readonly idleMs: number
  private readonly sleep: (ms: number, signal: AbortSignal) => Promise<void>

  /** 记下配置，续传位置从 opts.after 开始 */
  constructor(opts: StreamOptions, signal: AbortSignal) {
    this.opts = opts
    this.signal = signal
    this.cursor = opts.after
    this.fetchFn = opts.fetchImpl ?? ((input, init) => fetch(input, init))
    this.idleMs = opts.idleTimeoutMs ?? IDLE_TIMEOUT_MS
    this.sleep = opts.sleep ?? defaultSleep
  }

  /** 主循环：连接，结束后按结果重连、退避或停止；调用方取消时安静地退出 */
  async run(): Promise<void> {
    while (!this.signal.aborted) {
      this.opts.onState('connecting')
      this.liveSince = 0
      const outcome = await this.connectOnce()
      if (outcome.kind === 'aborted') return
      if (outcome.kind === 'reset' || outcome.kind === 'fatal') {
        this.opts.onState('stopped')
        return
      }
      const healthy = this.liveSince > 0 && Date.now() - this.liveSince >= MIN_LIVE_MS
      if (healthy) this.attempt = 0
      await this.waitBeforeRetry(outcome, healthy)
    }
  }

  // 重连前的等待：撑过一段时间后被服务端正常结束的连接马上重连；其余按 1、2、4…秒（最长 15 秒）退避。
  private async waitBeforeRetry(outcome: Outcome, healthy: boolean): Promise<void> {
    if (outcome.kind === 'ended' && healthy) return
    const delay = Math.min(BACKOFF_MAX_MS, BACKOFF_BASE_MS * 2 ** this.attempt)
    this.attempt += 1
    this.opts.onState('backoff')
    await this.sleep(delay, this.signal)
  }

  // 建立一次连接并读到结束。无论怎么结束，都会关闭这次连接并清掉定时器。
  private async connectOnce(): Promise<Outcome> {
    const conn = new AbortController()
    const forwardAbort = () => conn.abort()
    this.signal.addEventListener('abort', forwardAbort, { once: true })
    let timer: ReturnType<typeof setTimeout> | undefined
    const resetIdle = () => {
      clearTimeout(timer)
      timer = setTimeout(forwardAbort, this.idleMs)
    }
    try {
      resetIdle()
      const res = await this.fetchFn(this.url(), { headers: this.headers(), signal: conn.signal })
      if (!res.ok || !res.body) return this.failureOf(res)
      return await this.readStream(res.body, resetIdle)
    } catch {
      return this.signal.aborted ? { kind: 'aborted' } : { kind: 'failed' }
    } finally {
      clearTimeout(timer)
      this.signal.removeEventListener('abort', forwardAbort)
      conn.abort()
    }
  }

  // 流的地址：从当前续传位置之后开始。
  private url(): string {
    return `${API_PREFIX}/boards/${this.opts.boardId}/stream?after=${this.cursor}`
  }

  // 请求头：令牌放在 Authorization 里，声明要的是事件流。
  private headers(): Record<string, string> {
    return { Accept: 'text/event-stream', ...authHeaders(this.opts.getToken()) }
  }

  // 非 200 的响应：401、404 和其他 4xx 重试也没用，直接报告；其余（5xx、429、没有响应体）等一会儿再试。
  private failureOf(res: Response): Outcome {
    const retryable = res.status >= 500 || res.status === 429 || res.status === 408 || (res.ok && !res.body)
    if (retryable) return { kind: 'failed' }
    const kind: StreamFailure =
      res.status === 401 ? 'unauthorized' : res.status === 404 ? 'not_found' : 'server_error'
    this.opts.onFatal(kind, `HTTP ${res.status}`)
    return { kind: 'fatal' }
  }

  // 一直读到服务端结束或出现需要结束的消息。每读到一块数据（包括心跳）就重置半开检测的计时。
  private async readStream(body: ReadableStream<Uint8Array>, resetIdle: () => void): Promise<Outcome> {
    const reader = body.getReader()
    const decoder = new TextDecoder()
    const parser = new SseParser()
    for (;;) {
      const { done, value } = await reader.read()
      // 调用方已经停止：即使读到了数据也不再回调
      if (this.signal.aborted) return { kind: 'aborted' }
      resetIdle()
      if (done) return { kind: 'ended' }
      for (const msg of parser.feed(decoder.decode(value, { stream: true }))) {
        const verdict = this.handle(msg)
        if (verdict === 'ready') {
          this.liveSince = Date.now()
          this.opts.onState('live')
        } else if (verdict) {
          return verdict
        }
      }
    }
  }

  // 处理一条消息：board 交给调用方，reset 和 error 结束这次连接，其余（含未知事件）忽略。
  private handle(msg: SseMessage): Verdict {
    switch (msg.event) {
      case 'ready':
        return 'ready'
      case 'board':
        return this.deliver(msg)
      case 'reset':
        this.opts.onReset(parseLatest(msg.data))
        return { kind: 'reset' }
      case 'error':
        this.opts.onFatal('server_error', parseErrorMessage(msg.data))
        return { kind: 'fatal' }
      default:
        return null
    }
  }

  // 把一条 board 事件交给调用方并前移续传位置；序号不比当前位置新的当作重复丢掉；内容不是合法的事件就停止。
  private deliver(msg: SseMessage): Verdict {
    const event = parseBoardEvent(msg.data)
    if (!event) {
      this.opts.onFatal('server_error', 'malformed event')
      return { kind: 'fatal' }
    }
    if (event.id <= this.cursor) return null
    this.cursor = event.id
    try {
      this.opts.onEvent(event)
    } catch (err) {
      // 调用方的处理函数出错不应该让整条流断掉，也不能悄悄吞掉：记到控制台，继续收后面的事件
      console.error('slate: board event handler failed', err)
    }
    return null
  }
}

/**
 * 开始接收一个看板的实时事件，返回停止函数。
 * 停止后不会再回调；断线、服务端到点结束都会自动重连，reset 和不可恢复的错误会通知调用方并停止。
 */
export function connectBoardStream(opts: StreamOptions): () => void {
  const ctrl = new AbortController()
  new StreamSession(opts, ctrl.signal).run().catch((err: unknown) => {
    opts.onFatal('server_error', err instanceof Error ? err.message : String(err))
  })
  return () => ctrl.abort()
}
