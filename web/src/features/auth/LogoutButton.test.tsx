import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { en } from '@/i18n/locales/en'
import { installFakeApi } from '@/test/fakeApi'
import { renderApp } from '@/test/renderApp'
import { queryKeys } from '@/lib/queryKeys'
import { AuthGate } from './AuthGate'
import { LogoutButton } from './LogoutButton'

// 在 jsdom 里装一个假的 Service Worker 控制器，返回的 postMessage 记录页面发给它的消息；用例结束后在 afterEach 里还原
function installFakeServiceWorker() {
  const postMessage = vi.fn()
  Object.defineProperty(navigator, 'serviceWorker', { value: { controller: { postMessage } }, configurable: true })
  return postMessage
}

afterEach(() => {
  Reflect.deleteProperty(navigator, 'serviceWorker')
})

// 渲染门加退出按钮，门里面是一句应用内容
function renderWithGate() {
  return renderApp(
    <AuthGate>
      <p>应用内容</p>
      <LogoutButton />
    </AuthGate>,
  )
}

describe('LogoutButton', () => {
  it('没有启用口令时不出现', async () => {
    installFakeApi({ 'GET /api/v1/auth/status': { required: false, authenticated: true } })
    renderWithGate()
    await screen.findByText('应用内容')
    expect(screen.queryByRole('button', { name: en.auth.logout })).toBeNull()
  })

  it('点击后通知服务端退出、清掉离线缓存，并回到登录页', async () => {
    const postMessage = installFakeServiceWorker()
    const api = installFakeApi({
      'GET /api/v1/auth/status': { required: true, authenticated: true },
      'POST /api/v1/auth/logout': {},
    })
    const user = userEvent.setup()
    renderWithGate()
    await user.click(await screen.findByRole('button', { name: en.auth.logout }))
    expect(await screen.findByRole('heading', { name: en.auth.title })).toBeInTheDocument()
    expect(api.count('POST /api/v1/auth/logout')).toBe(1)
    expect(postMessage).toHaveBeenCalledWith({ type: 'clear-caches' })
    expect(screen.queryByText('应用内容')).toBeNull()
  })

  it('退出后内存里的看板数据被清掉，只留下「未登录」的状态', async () => {
    installFakeServiceWorker()
    installFakeApi({
      'GET /api/v1/auth/status': { required: true, authenticated: true },
      'POST /api/v1/auth/logout': {},
    })
    const user = userEvent.setup()
    const { queryClient } = renderWithGate()
    queryClient.setQueryData(queryKeys.boards, [{ id: 1 }])
    await user.click(await screen.findByRole('button', { name: en.auth.logout }))
    await screen.findByRole('heading', { name: en.auth.title })
    expect(queryClient.getQueryData(queryKeys.boards)).toBeUndefined()
    expect(queryClient.getQueryData(queryKeys.auth)).toEqual({ required: true, authenticated: false })
  })

  it('退出失败时仍留在应用里，提示失败原因，也不清缓存', async () => {
    const postMessage = installFakeServiceWorker()
    installFakeApi({
      'GET /api/v1/auth/status': { required: true, authenticated: true },
      'POST /api/v1/auth/logout': () => ({ status: 500, body: { reason: 'ErrInternal', msg: '出错' } }),
    })
    const user = userEvent.setup()
    renderWithGate()
    await user.click(await screen.findByRole('button', { name: en.auth.logout }))
    expect(await screen.findByText(en.errors.ErrInternal)).toBeInTheDocument()
    expect(screen.getByText('应用内容')).toBeInTheDocument()
    await waitFor(() => expect(screen.getByRole('button', { name: en.auth.logout })).toBeEnabled())
    expect(postMessage).not.toHaveBeenCalled()
  })
})
