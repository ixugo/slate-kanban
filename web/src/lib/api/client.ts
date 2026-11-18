// 调用服务端 REST 接口的底层封装：统一加令牌头、解析 JSON、把错误整理成 ApiError。
// 令牌只通过 Authorization 请求头发送，不会出现在 URL 里。

import { clearToken, getToken } from '@/lib/token'
import type { ErrorBody } from './types'

/** 所有接口的公共前缀 */
export const API_PREFIX = '/api/v1'

/** 网络不通、被浏览器拦截等没有拿到响应的情况使用的 reason */
export const REASON_NETWORK = 'ErrNetwork'

/** 响应体不是预期的 JSON 时使用的 reason */
export const REASON_BAD_RESPONSE = 'ErrBadResponse'

/** 服务端返回的错误。reason 是固定的错误名，网页据此显示各语言的提示；message 是服务端的兜底文案 */
export class ApiError extends Error {
  readonly status: number
  readonly reason: string
  readonly traceId: string

  /** 创建错误；status 为 0 表示根本没有拿到响应 */
  constructor(status: number, reason: string, message: string, traceId = '') {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.reason = reason
    this.traceId = traceId
  }
}

/** 查询参数的值，undefined 会被跳过 */
export type QueryValue = string | number | boolean | undefined

export interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  /** 请求体，会被序列化成 JSON */
  body?: unknown
  query?: Record<string, QueryValue>
  /** 取消请求，页面卸载或切换看板时用 */
  signal?: AbortSignal
  /** 显式指定令牌，登录时用来验证"还没保存"的令牌；指定后 401 不会清掉已保存的令牌 */
  token?: string
}

// 把查询参数拼成 "?a=1&b=2"，没有参数时返回空串。
function buildQuery(query?: Record<string, QueryValue>): string {
  if (!query) return ''
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) params.set(key, String(value))
  }
  const text = params.toString()
  return text ? `?${text}` : ''
}

// 读出响应体的文本并尽量解析成错误体；解析不了就当作没有内容。
async function readErrorBody(res: Response): Promise<ErrorBody> {
  const text = await res.text().catch(() => '')
  if (!text) return {}
  try {
    const parsed: unknown = JSON.parse(text)
    return typeof parsed === 'object' && parsed !== null ? (parsed as ErrorBody) : {}
  } catch {
    return { msg: text.slice(0, 200) }
  }
}

// 把失败的响应转成 ApiError；401 且用的是已保存的令牌时清掉令牌，页面会回到登录页。
async function toApiError(res: Response, usedStoredToken: boolean): Promise<ApiError> {
  if (res.status === 401 && usedStoredToken) clearToken()
  const body = await readErrorBody(res)
  const reason = body.reason || `Http${res.status}`
  return new ApiError(res.status, reason, body.msg || res.statusText, body.trace_id ?? '')
}

// 发出请求，网络层的错误统一成 reason 为 ErrNetwork 的 ApiError；取消请求的 AbortError 原样抛出。
async function send(url: string, init: RequestInit): Promise<Response> {
  try {
    return await fetch(url, init)
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') throw err
    throw new ApiError(0, REASON_NETWORK, err instanceof Error ? err.message : 'network error')
  }
}

/** 构造带令牌的请求头，实时流和普通请求共用 */
export function authHeaders(token: string): Record<string, string> {
  return token ? { Authorization: `Bearer ${token}` } : {}
}

/**
 * 调用一个接口并返回解析后的 JSON。
 * 成功时服务端直接返回资源本身，删除之类没有内容的接口返回 `{}`。
 */
export async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const token = opts.token ?? getToken()
  const headers: Record<string, string> = { Accept: 'application/json', ...authHeaders(token) }
  const init: RequestInit = { method: opts.method ?? 'GET', headers, signal: opts.signal }
  if (opts.body !== undefined) {
    headers['Content-Type'] = 'application/json'
    init.body = JSON.stringify(opts.body)
  }
  const res = await send(`${API_PREFIX}${path}${buildQuery(opts.query)}`, init)
  if (!res.ok) throw await toApiError(res, opts.token === undefined)
  try {
    return (await res.json()) as T
  } catch {
    throw new ApiError(res.status, REASON_BAD_RESPONSE, 'invalid response body')
  }
}
