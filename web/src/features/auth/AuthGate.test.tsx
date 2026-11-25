import { fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { en } from '@/i18n/locales/en'
import { installFakeApi, type FakeHandler } from '@/test/fakeApi'
import { renderApp } from '@/test/renderApp'
import { useBoards } from '@/features/boards/useBoards'
import { AuthGate } from './AuthGate'

afterEach(() => vi.unstubAllGlobals())

const APP_TEXT = '应用内容'
const PASSCODE = 'Real-Pass_2026'

// 渲染口令门，门里面是一句应用内容
function renderGate() {
  return renderApp(
    <AuthGate>
      <p>{APP_TEXT}</p>
    </AuthGate>,
  )
}

// 假服务端的口令状态接口
const status = (required: boolean, authenticated: boolean): FakeHandler => ({ required, authenticated })

// 在登录页上输入口令并提交
async function submitPasscode(value: string) {
  const user = userEvent.setup()
  await user.type(await screen.findByLabelText(en.auth.passcode), value)
  await user.click(screen.getByRole('button', { name: en.auth.submit }))
}

describe('AuthGate', () => {
  it('没有启用口令：直接显示应用，不出现登录表单', async () => {
    installFakeApi({ 'GET /api/v1/auth/status': status(false, true) })
    renderGate()
    expect(await screen.findByText(APP_TEXT)).toBeInTheDocument()
    expect(screen.queryByLabelText(en.auth.passcode)).toBeNull()
  })

  it('启用了口令并且已登录：显示应用', async () => {
    installFakeApi({ 'GET /api/v1/auth/status': status(true, true) })
    renderGate()
    expect(await screen.findByText(APP_TEXT)).toBeInTheDocument()
  })

  it('启用了口令但没登录：只显示登录页，应用内容不渲染', async () => {
    installFakeApi({ 'GET /api/v1/auth/status': status(true, false) })
    renderGate()
    expect(await screen.findByRole('heading', { name: en.auth.title })).toBeInTheDocument()
    expect(screen.getByLabelText(en.auth.passcode)).toHaveAttribute('type', 'password')
    expect(screen.queryByText(APP_TEXT)).toBeNull()
  })

  it('macOS 桌面端登录页可以从顶栏空白处拖动窗口', async () => {
    document.documentElement.setAttribute('data-slate-mac', '')
    const invoke = vi.fn()
    vi.stubGlobal('_wails', { invoke })
    try {
      installFakeApi({ 'GET /api/v1/auth/status': status(true, false) })
      renderGate()
      const header = await screen.findByRole('banner')
      fireEvent.mouseDown(header, { button: 0, buttons: 1 })
      fireEvent.mouseMove(header, { buttons: 1 })
      fireEvent.mouseUp(header, { button: 0 })
      fireEvent.mouseDown(screen.getByLabelText(en.auth.passcode), { button: 0, buttons: 1 })
      fireEvent.mouseMove(header, { buttons: 1 })
      fireEvent.mouseUp(header, { button: 0 })
      expect(invoke.mock.calls).toEqual([['wails:drag']])
    } finally {
      document.documentElement.removeAttribute('data-slate-mac')
    }
  })

  it('读口令状态失败（比如断网）时放行，不挡住离线只读', async () => {
    installFakeApi({
      'GET /api/v1/auth/status': () => {
        throw new TypeError('Failed to fetch')
      },
    })
    renderGate()
    expect(await screen.findByText(APP_TEXT)).toBeInTheDocument()
  })

  it('输入正确口令后进入应用，并且发给服务端的是去掉首尾空白的口令', async () => {
    const api = installFakeApi({
      'GET /api/v1/auth/status': status(true, false),
      'POST /api/v1/auth/login': { authenticated: true },
    })
    renderGate()
    await submitPasscode(`  ${PASSCODE}  `)
    expect(await screen.findByText(APP_TEXT)).toBeInTheDocument()
    expect(api.calls.find((c) => c.method === 'POST')?.body).toEqual({ passcode: PASSCODE })
    expect(screen.queryByLabelText(en.auth.passcode)).toBeNull()
  })

  it('口令不对（401）：提示口令不对，仍停在登录页，输入框标为无效', async () => {
    installFakeApi({
      'GET /api/v1/auth/status': status(true, false),
      'POST /api/v1/auth/login': () => ({ status: 401, body: { reason: 'ErrUnauthorized', msg: '需要访问口令' } }),
    })
    renderGate()
    await submitPasscode('wrong')
    expect(await screen.findByText(en.auth.wrong)).toBeInTheDocument()
    expect(screen.getByLabelText(en.auth.passcode)).toHaveAttribute('aria-invalid', 'true')
    expect(screen.queryByText(APP_TEXT)).toBeNull()
  })

  it('猜得太频繁被限流（429）：提示稍后再试，不说口令不对', async () => {
    installFakeApi({
      'GET /api/v1/auth/status': status(true, false),
      'POST /api/v1/auth/login': () => ({ status: 429, body: { reason: 'ErrTooManyAttempts', msg: '尝试次数过多，请稍后再试' } }),
    })
    renderGate()
    await submitPasscode(PASSCODE)
    expect(await screen.findByText(en.errors.ErrTooManyAttempts)).toBeInTheDocument()
    expect(screen.queryByText(en.auth.wrong)).toBeNull()
  })

  it('口令为空或只有空格时按钮不可点，不发登录请求', async () => {
    const api = installFakeApi({ 'GET /api/v1/auth/status': status(true, false) })
    const user = userEvent.setup()
    renderGate()
    const button = await screen.findByRole('button', { name: en.auth.submit })
    expect(button).toBeDisabled()
    await user.type(screen.getByLabelText(en.auth.passcode), '   ')
    expect(button).toBeDisabled()
    await user.keyboard('{Enter}')
    expect(api.count('POST /api/v1/auth/login')).toBe(0)
  })
})

// 门里面的一个会请求看板列表的组件，用来模拟「用着用着登录过期」
function BoardsProbe() {
  const { data } = useBoards()
  return <p>{data ? `看板 ${data.length} 个` : APP_TEXT}</p>
}

describe('AuthGate 遇到 401', () => {
  it('会话过期：接口返回 401，重新问服务端后切到登录页', async () => {
    let authenticated = true
    const api = installFakeApi({
      'GET /api/v1/auth/status': () => ({ body: { required: true, authenticated } }),
      'GET /api/v1/boards': () => {
        authenticated = false
        return { status: 401, body: { reason: 'ErrUnauthorized', msg: '需要访问口令' } }
      },
    })
    renderApp(
      <AuthGate>
        <BoardsProbe />
      </AuthGate>,
    )
    expect(await screen.findByRole('heading', { name: en.auth.title })).toBeInTheDocument()
    expect(api.count('GET /api/v1/auth/status')).toBe(2)
  })

  it('接口返回 401 但服务端并没有启用口令（比如前面有别的网关）：不跳登录页', async () => {
    const api = installFakeApi({
      'GET /api/v1/auth/status': status(false, true),
      'GET /api/v1/boards': () => ({ status: 401, body: { reason: 'ErrUnauthorized', msg: '网关拒绝' } }),
    })
    renderApp(
      <AuthGate>
        <BoardsProbe />
      </AuthGate>,
    )
    await waitFor(() => expect(api.count('GET /api/v1/boards')).toBe(1))
    await waitFor(() => expect(api.count('GET /api/v1/auth/status')).toBe(2))
    expect(screen.getByText(APP_TEXT)).toBeInTheDocument()
    expect(screen.queryByLabelText(en.auth.passcode)).toBeNull()
  })

  it('口令状态接口自己返回 401（前面有别的网关）：只请求一次，不无限重读，应用照常显示', async () => {
    const api = installFakeApi({
      'GET /api/v1/auth/status': () => ({ status: 401, body: { reason: 'ErrUnauthorized', msg: '网关拒绝' } }),
    })
    renderGate()
    expect(await screen.findByText(APP_TEXT)).toBeInTheDocument()
    await new Promise((resolve) => setTimeout(resolve, 100))
    expect(api.count('GET /api/v1/auth/status')).toBe(1)
  })

  it('其他错误（500）不会重新读口令状态', async () => {
    const api = installFakeApi({
      'GET /api/v1/auth/status': status(true, true),
      'GET /api/v1/boards': () => ({ status: 500, body: { reason: 'ErrInternal', msg: '出错' } }),
    })
    renderApp(
      <AuthGate>
        <BoardsProbe />
      </AuthGate>,
    )
    await waitFor(() => expect(api.count('GET /api/v1/boards')).toBe(1))
    expect(api.count('GET /api/v1/auth/status')).toBe(1)
  })
})
