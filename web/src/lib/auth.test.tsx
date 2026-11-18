import { QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '@/lib/api/client'
import { createQueryClient } from '@/lib/queryClient'
import { clearToken, getToken, setToken } from '@/lib/token'
import { AuthProvider } from './auth'
import { REASON_NOT_HUMAN, useAuth } from './auth-context'

// 模拟 /me 接口：按收到的令牌决定返回什么。
function stubMe(handler: (token: string) => { status: number; body: unknown }) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (_url: string, init: RequestInit) => {
      const auth = new Headers(init.headers).get('Authorization') ?? ''
      const { status, body } = handler(auth.replace('Bearer ', ''))
      return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
    }),
  )
}

function setup() {
  const queryClient = createQueryClient()
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <AuthProvider>{children}</AuthProvider>
    </QueryClientProvider>
  )
  return { queryClient, ...renderHook(() => useAuth(), { wrapper }) }
}

// 调用 login 并返回它抛出的错误；成功时返回 null。
async function loginError(login: (token: string) => Promise<void>, token: string): Promise<unknown> {
  try {
    await act(async () => {
      await login(token)
    })
  } catch (err) {
    return err
  }
  return null
}

describe('AuthProvider', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    clearToken()
  })

  it('管理令牌登录成功：保存令牌（去掉首尾空白）', async () => {
    stubMe((t) => (t === 'good' ? { status: 200, body: { name: 'admin', kind: 'human' } } : { status: 401, body: {} }))
    const { result } = setup()
    expect(result.current.token).toBe('')
    await act(async () => {
      await result.current.login('  good  ')
    })
    expect(result.current.token).toBe('good')
    expect(getToken()).toBe('good')
  })

  it('agent 令牌被拒绝，并且不会保存', async () => {
    stubMe(() => ({ status: 200, body: { name: 'codex', kind: 'agent' } }))
    const { result } = setup()
    const err = await loginError(result.current.login, 'agent-token')
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).reason).toBe(REASON_NOT_HUMAN)
    expect(getToken()).toBe('')
  })

  it('令牌无效（401）抛出错误，旧令牌不受影响', async () => {
    stubMe(() => ({ status: 401, body: { reason: 'ErrUnauthorized', msg: '未授权' } }))
    setToken('old')
    const { result } = setup()
    const err = await loginError(result.current.login, 'bad')
    expect((err as ApiError).status).toBe(401)
    expect(getToken()).toBe('old')
  })

  it('登录前清空缓存，上一个身份的数据不会留下', async () => {
    stubMe(() => ({ status: 200, body: { name: 'admin', kind: 'human' } }))
    const { result, queryClient } = setup()
    queryClient.setQueryData(['boards'], [{ id: 1 }])
    await act(async () => {
      await result.current.login('good')
    })
    expect(queryClient.getQueryData(['boards'])).toBeUndefined()
  })

  it('退出清掉令牌和缓存', async () => {
    setToken('good')
    const { result, queryClient } = setup()
    queryClient.setQueryData(['boards'], [{ id: 1 }])
    act(() => result.current.logout())
    expect(result.current.token).toBe('')
    expect(getToken()).toBe('')
    expect(queryClient.getQueryData(['boards'])).toBeUndefined()
  })

  it('令牌在别处被清掉（比如接口返回 401）时，登录状态跟着变化', () => {
    setToken('good')
    const { result } = setup()
    expect(result.current.token).toBe('good')
    act(() => clearToken())
    expect(result.current.token).toBe('')
  })

  it('不在 AuthProvider 里使用会明确报错', () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    expect(() => renderHook(() => useAuth())).toThrow('AuthProvider')
    spy.mockRestore()
  })
})
