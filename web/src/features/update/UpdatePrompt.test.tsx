import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it } from 'vitest'
import { en } from '@/i18n/locales/en'
import type { UpdateInfo } from '@/lib/api/types'
import { installFakeApi } from '@/test/fakeApi'
import { renderApp } from '@/test/renderApp'
import { UpdatePrompt } from './UpdatePrompt'
import { IGNORED_KEY } from './version'

afterEach(() => localStorage.clear())

const info: UpdateInfo = {
  current: 'v1.0.0',
  latest: 'v1.1.0',
  notes: '修复了若干问题',
  url: 'https://github.com/ixugo/slate-kanban/releases/tag/v1.1.0',
  has_update: true,
  can_apply: true,
}

describe('UpdatePrompt', () => {
  it('有新版本时弹窗显示版本号和更新说明', async () => {
    installFakeApi({ 'GET /api/v1/update': info })
    renderApp(<UpdatePrompt />)
    expect(await screen.findByText(en.update.title)).toBeInTheDocument()
    expect(screen.getByText('Current v1.0.0, latest v1.1.0')).toBeInTheDocument()
    expect(screen.getByText('修复了若干问题')).toBeInTheDocument()
  })

  it('没有更新、查询失败、已忽略的版本都不弹窗', async () => {
    const api = installFakeApi({ 'GET /api/v1/update': { ...info, has_update: false } })
    const { unmount } = renderApp(<UpdatePrompt />)
    await waitFor(() => expect(api.count('GET /api/v1/update')).toBe(1))
    expect(screen.queryByText(en.update.title)).not.toBeInTheDocument()
    unmount()

    const failing = installFakeApi({ 'GET /api/v1/update': () => ({ status: 500, body: { reason: 'ErrServer' } }) })
    const second = renderApp(<UpdatePrompt />)
    await waitFor(() => expect(failing.count('GET /api/v1/update')).toBe(1))
    expect(screen.queryByText(en.update.title)).not.toBeInTheDocument()
    second.unmount()

    localStorage.setItem(IGNORED_KEY, 'v1.1.0')
    const ignored = installFakeApi({ 'GET /api/v1/update': info })
    renderApp(<UpdatePrompt />)
    await waitFor(() => expect(ignored.count('GET /api/v1/update')).toBe(1))
    expect(screen.queryByText(en.update.title)).not.toBeInTheDocument()
  })

  it('忽略的是更旧的版本时，更新的版本仍然弹窗', async () => {
    localStorage.setItem(IGNORED_KEY, 'v1.0.5')
    installFakeApi({ 'GET /api/v1/update': info })
    renderApp(<UpdatePrompt />)
    expect(await screen.findByText(en.update.title)).toBeInTheDocument()
  })

  it('点「此版本忽略」记下版本号并关闭弹窗', async () => {
    installFakeApi({ 'GET /api/v1/update': info })
    renderApp(<UpdatePrompt />)
    await userEvent.click(await screen.findByRole('button', { name: en.update.ignore }))
    expect(localStorage.getItem(IGNORED_KEY)).toBe('v1.1.0')
    expect(screen.queryByText(en.update.title)).not.toBeInTheDocument()
  })

  it('点「立即更新」调用服务端并提示重启', async () => {
    const api = installFakeApi({ 'GET /api/v1/update': info, 'POST /api/v1/update': { version: 'v1.1.0' } })
    renderApp(<UpdatePrompt />)
    await userEvent.click(await screen.findByRole('button', { name: en.update.now }))
    expect(await screen.findByText('Updated to v1.1.0. Restart Slate, then refresh the page.')).toBeInTheDocument()
    expect(api.count('POST /api/v1/update')).toBe(1)
    expect(screen.queryByRole('button', { name: en.update.now })).not.toBeInTheDocument()
  })

  it('更新失败时显示错误并允许重试', async () => {
    const api = installFakeApi({
      'GET /api/v1/update': info,
      'POST /api/v1/update': () => ({ status: 500, body: { reason: 'ErrServer', msg: '更新失败：磁盘已满' } }),
    })
    renderApp(<UpdatePrompt />)
    await userEvent.click(await screen.findByRole('button', { name: en.update.now }))
    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: en.update.now })).toBeEnabled()
    expect(api.count('POST /api/v1/update')).toBe(1)
  })

  it('不能自动更新时引导去下载页面', async () => {
    installFakeApi({ 'GET /api/v1/update': { ...info, can_apply: false } })
    renderApp(<UpdatePrompt />)
    const link = await screen.findByRole('link', { name: en.update.download })
    expect(link).toHaveAttribute('href', info.url)
    expect(screen.queryByRole('button', { name: en.update.now })).not.toBeInTheDocument()
  })
})
