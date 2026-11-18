import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clearToken, getToken, setToken } from '@/lib/token'
import { ApiError, REASON_BAD_RESPONSE, REASON_NETWORK, request } from './client'

type FetchMock = ReturnType<typeof vi.fn>

// 把全局 fetch 换成测试用的假实现，返回它方便检查调用参数。
function mockFetch(impl: (url: string, init: RequestInit) => Promise<Response>): FetchMock {
  const fn = vi.fn(impl)
  vi.stubGlobal('fetch', fn)
  return fn
}

// 造一个 JSON 响应。
function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

beforeEach(() => {
  clearToken()
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('request', () => {
  it('把已保存的令牌放进 Authorization 头，GET 不带 Content-Type', async () => {
    setToken('abc')
    const fetchMock = mockFetch(() => Promise.resolve(json({ ok: 1 })))
    await request('/boards')
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/boards')
    const headers = init.headers as Record<string, string>
    expect(headers.Authorization).toBe('Bearer abc')
    expect(headers['Content-Type']).toBeUndefined()
    expect(init.method).toBe('GET')
  })

  it('令牌不会出现在地址里', async () => {
    setToken('super-secret')
    const fetchMock = mockFetch(() => Promise.resolve(json({})))
    await request('/boards', { query: { a: 1 } })
    expect((fetchMock.mock.calls[0] as [string])[0]).not.toContain('super-secret')
  })

  it('没有令牌时不发送 Authorization 头', async () => {
    const fetchMock = mockFetch(() => Promise.resolve(json({})))
    await request('/me')
    const init = (fetchMock.mock.calls[0] as [string, RequestInit])[1]
    expect((init.headers as Record<string, string>).Authorization).toBeUndefined()
  })

  it('显式传入的令牌优先于已保存的', async () => {
    setToken('stored')
    const fetchMock = mockFetch(() => Promise.resolve(json({})))
    await request('/me', { token: 'candidate' })
    const init = (fetchMock.mock.calls[0] as [string, RequestInit])[1]
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer candidate')
  })

  it('查询参数会编码，undefined 的被跳过，false 和 0 保留', async () => {
    const fetchMock = mockFetch(() => Promise.resolve(json({})))
    await request('/x', { query: { q: 'col:"进行中" -is:free', limit: 0, flag: false, skip: undefined } })
    const url = (fetchMock.mock.calls[0] as [string])[0]
    const params = new URL(url, 'http://x').searchParams
    expect(params.get('q')).toBe('col:"进行中" -is:free')
    expect(params.get('limit')).toBe('0')
    expect(params.get('flag')).toBe('false')
    expect(params.has('skip')).toBe(false)
  })

  it('有请求体时序列化成 JSON 并声明类型', async () => {
    const fetchMock = mockFetch(() => Promise.resolve(json({})))
    await request('/cards', { method: 'POST', body: { title: '标题', tags: [] } })
    const init = (fetchMock.mock.calls[0] as [string, RequestInit])[1]
    expect((init.headers as Record<string, string>)['Content-Type']).toBe('application/json')
    expect(init.body).toBe('{"title":"标题","tags":[]}')
    expect(init.method).toBe('POST')
  })

  it('请求体是 null、0、false 这类假值时也要发出去', async () => {
    const fetchMock = mockFetch(() => Promise.resolve(json({})))
    await request('/x', { method: 'PATCH', body: false })
    expect((fetchMock.mock.calls[0] as [string, RequestInit])[1].body).toBe('false')
  })

  it('解析服务端的错误体', async () => {
    mockFetch(() =>
      Promise.resolve(json({ reason: 'ErrBoardNotFound', msg: '看板不存在', trace_id: 'tr-1' }, 404)),
    )
    const err = await request('/boards/9').catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({ status: 404, reason: 'ErrBoardNotFound', message: '看板不存在', traceId: 'tr-1' })
  })

  it('错误响应不是 JSON 时，reason 用状态码，文字截断到 200 个字符', async () => {
    mockFetch(() => Promise.resolve(new Response('x'.repeat(500), { status: 502 })))
    const err = (await request('/x').catch((e: unknown) => e)) as ApiError
    expect(err.reason).toBe('Http502')
    expect(err.message).toHaveLength(200)
  })

  it('错误响应没有内容时用状态文字兜底', async () => {
    mockFetch(() => Promise.resolve(new Response(null, { status: 503, statusText: 'Service Unavailable' })))
    const err = (await request('/x').catch((e: unknown) => e)) as ApiError
    expect(err).toMatchObject({ status: 503, reason: 'Http503', message: 'Service Unavailable' })
  })

  it('用已保存的令牌得到 401 时清掉令牌', async () => {
    setToken('stale')
    mockFetch(() => Promise.resolve(json({ reason: 'ErrUnauthorized', msg: 'x' }, 401)))
    await expect(request('/boards')).rejects.toMatchObject({ status: 401 })
    expect(getToken()).toBe('')
  })

  it('用显式传入的令牌得到 401 时不动已保存的令牌', async () => {
    setToken('keep')
    mockFetch(() => Promise.resolve(json({ reason: 'ErrUnauthorized', msg: 'x' }, 401)))
    await expect(request('/me', { token: 'wrong' })).rejects.toMatchObject({ status: 401 })
    expect(getToken()).toBe('keep')
  })

  it('其他错误状态不会清令牌', async () => {
    setToken('keep')
    mockFetch(() => Promise.resolve(json({ reason: 'ErrHumanOnly', msg: 'x' }, 403)))
    await expect(request('/boards')).rejects.toMatchObject({ status: 403 })
    expect(getToken()).toBe('keep')
  })

  it('网络不通时抛出 reason 为 ErrNetwork、状态码为 0 的错误', async () => {
    mockFetch(() => Promise.reject(new TypeError('Failed to fetch')))
    await expect(request('/boards')).rejects.toMatchObject({ status: 0, reason: REASON_NETWORK })
  })

  it('取消请求产生的 AbortError 原样抛出，不包装成网络错误', async () => {
    mockFetch(() => Promise.reject(new DOMException('aborted', 'AbortError')))
    const err = await request('/boards').catch((e: unknown) => e)
    expect(err).toBeInstanceOf(DOMException)
    expect((err as DOMException).name).toBe('AbortError')
  })

  it('成功响应不是合法 JSON 时抛出 ErrBadResponse', async () => {
    mockFetch(() => Promise.resolve(new Response('<html>', { status: 200 })))
    await expect(request('/boards')).rejects.toMatchObject({ reason: REASON_BAD_RESPONSE })
  })

  it('取消信号会传给 fetch', async () => {
    const ctrl = new AbortController()
    const fetchMock = mockFetch(() => Promise.resolve(json({})))
    await request('/boards', { signal: ctrl.signal })
    expect((fetchMock.mock.calls[0] as [string, RequestInit])[1].signal).toBe(ctrl.signal)
  })
})
