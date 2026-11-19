// 测试用的假服务端：按"方法 + 路径"匹配请求，返回预设的 JSON，并记录每一次调用，
// 让页面测试不需要真的起服务。没有匹配的请求会记下来，用 expectNoUnmatched 检查，防止页面悄悄多发了请求。

import { vi } from 'vitest'

/** 假服务端收到的一次请求 */
export interface FakeCall {
  method: string
  /** 路径，不含查询参数 */
  path: string
  query: URLSearchParams
  body: unknown
}

/** 路由处理函数：返回状态码和 JSON 体；也可以直接给一个静态的 JSON 体（状态码 200） */
export type FakeHandler = unknown | ((call: FakeCall) => { status?: number; body?: unknown })

export interface FakeApi {
  calls: FakeCall[]
  unmatched: string[]
  /** 换掉某条路由的处理 */
  set: (route: string, handler: FakeHandler) => void
  /** 某条路由被调用的次数 */
  count: (route: string) => number
  /** 断言没有未匹配的请求 */
  expectNoUnmatched: () => void
}

// 把请求体解析成 JSON；表单（上传文件）解析成 { 字段: 值 }；没有请求体返回 undefined。
function parseBody(body: BodyInit | null | undefined): unknown {
  if (body instanceof FormData) return Object.fromEntries(body.entries())
  return typeof body === 'string' && body !== '' ? JSON.parse(body) : undefined
}

/**
 * 安装假的 fetch。routes 的键写成 "GET /api/v1/boards"，路径里可以用 :id 匹配一段数字（如 "GET /api/v1/boards/:id/snapshot"）。
 */
export function installFakeApi(routes: Record<string, FakeHandler>): FakeApi {
  const table = new Map<string, FakeHandler>(Object.entries(routes))
  const calls: FakeCall[] = []
  const unmatched: string[] = []

  // 把 "GET /a/:id/b" 变成匹配方法和路径的正则
  const matcher = (route: string) => {
    const [method, pattern] = route.split(' ')
    return { method, regex: new RegExp(`^${pattern.replace(/:\w+/g, '[^/]+')}$`) }
  }

  const fakeFetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = new URL(String(input), 'http://localhost')
    const method = (init?.method ?? 'GET').toUpperCase()
    const call: FakeCall = {
      method,
      path: url.pathname,
      query: url.searchParams,
      body: parseBody(init?.body),
    }
    calls.push(call)
    const key = [...table.keys()].find((route) => {
      const m = matcher(route)
      return m.method === method && m.regex.test(url.pathname)
    })
    if (key === undefined) {
      unmatched.push(`${method} ${url.pathname}`)
      return new Response(JSON.stringify({ reason: 'ErrNotFound', msg: 'fake: unmatched route' }), { status: 404 })
    }
    const handler = table.get(key)
    const result = typeof handler === 'function' ? (handler as (c: FakeCall) => { status?: number; body?: unknown })(call) : { body: handler }
    return new Response(JSON.stringify(result.body ?? {}), {
      status: result.status ?? 200,
      headers: { 'Content-Type': 'application/json' },
    })
  })
  vi.stubGlobal('fetch', fakeFetch)

  return {
    calls,
    unmatched,
    set: (route, handler) => void table.set(route, handler),
    count: (route) => {
      const m = matcher(route)
      return calls.filter((c) => c.method === m.method && m.regex.test(c.path)).length
    },
    expectNoUnmatched: () => {
      if (unmatched.length > 0) throw new Error(`有未匹配的请求：${unmatched.join('，')}`)
    },
  }
}
