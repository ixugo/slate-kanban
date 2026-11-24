import { act, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { en } from '@/i18n/locales/en'
import { installFakeApi } from '@/test/fakeApi'
import { renderApp } from '@/test/renderApp'
import { AppSettingsButton } from './AppSettingsButton'
import { PROJECT_URL } from './AboutSettings'

describe('AppSettingsDialog', () => {
  it('键盘打开设置、切换到关于，Escape 返回设置按钮', async () => {
    installFakeApi({ 'GET /api/v1/version': { version: 'v0.1.0' } })
    renderApp(<AppSettingsButton />)
    const trigger = screen.getByRole('button', { name: en.nav.settings })
    trigger.focus()
    await userEvent.keyboard('{Enter}')
    expect(await screen.findByRole('dialog', { name: en.nav.settings })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: en.preferences.general, selected: true })).toHaveFocus()
    await userEvent.keyboard('{ArrowDown}')
    expect(await screen.findByRole('tab', { name: en.preferences.templates, selected: true })).toHaveFocus()
    await userEvent.keyboard('{ArrowDown}')
    expect(await screen.findByRole('tab', { name: en.nav.about, selected: true })).toHaveFocus()
    await userEvent.keyboard('{Escape}')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(trigger).toHaveFocus()
  })

  it('点开显示介绍、服务端版本号和跳到项目仓库的 GitHub 图标链接', async () => {
    installFakeApi({ 'GET /api/v1/version': { version: 'v0.1.0', remark: 'master_abc_261119' } })
    renderApp(<AppSettingsButton />)
    await userEvent.click(screen.getByRole('button', { name: en.nav.settings }))
    await userEvent.click(await screen.findByRole('tab', { name: en.nav.about }))
    expect(await screen.findByRole('heading', { name: en.about.title })).toBeInTheDocument()
    expect(screen.getByText(en.about.intro)).toBeInTheDocument()
    expect(await screen.findByText('v0.1.0')).toBeInTheDocument()
    const link = screen.getByRole('link', { name: 'GitHub' })
    expect(link).toHaveAttribute('href', 'https://github.com/ixugo/slate-kanban')
    expect(link).toHaveAttribute('href', PROJECT_URL)
    expect(screen.queryByText(PROJECT_URL)).not.toBeInTheDocument()
    expect(link).toHaveAttribute('rel', expect.stringContaining('noopener'))
  })

  it('原生菜单请求可在设置加载前打开关于，弹窗开启时也能切回通用', async () => {
    installFakeApi({ 'GET /api/v1/version': { version: 'v0.1.0' } })
    const html = document.documentElement
    html.setAttribute('data-slate-settings-tab', 'about')
    try {
      renderApp(<AppSettingsButton />)
      expect(await screen.findByRole('tab', { name: en.nav.about, selected: true })).toBeInTheDocument()
      expect(screen.getByRole('link', { name: 'GitHub' })).toHaveAttribute('href', PROJECT_URL)
      expect(html).not.toHaveAttribute('data-slate-settings-tab')

      await act(async () => html.setAttribute('data-slate-settings-tab', 'general'))
      expect(screen.getByRole('tab', { name: en.preferences.general, selected: true })).toBeInTheDocument()
      expect(html).not.toHaveAttribute('data-slate-settings-tab')
    } finally {
      html.removeAttribute('data-slate-settings-tab')
    }
  })

  it('关于里的手动检查发现新版本时，复用更新说明和安装弹窗', async () => {
    const api = installFakeApi({
      'GET /api/v1/version': { version: 'v0.1.0' },
      'GET /api/v1/update': {
        current: 'v0.1.0',
        latest: 'v0.1.1',
        has_update: true,
        can_apply: true,
        check_state: 'ok',
        notes: 'A manual check result',
        url: 'https://example.com/v0.1.1',
      },
    })
    renderApp(<AppSettingsButton />)
    await userEvent.click(screen.getByRole('button', { name: en.nav.settings }))
    await userEvent.click(await screen.findByRole('tab', { name: en.nav.about }))
    expect(api.count('GET /api/v1/update')).toBe(0)
    await userEvent.click(screen.getByRole('button', { name: en.update.check }))
    expect(await screen.findByRole('dialog', { name: en.update.title })).toBeInTheDocument()
    expect(screen.getByText('A manual check result')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: en.update.now })).toBeEnabled()
    await userEvent.keyboard('{Escape}')
    expect(await screen.findByRole('tab', { name: en.nav.about, selected: true })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: en.update.check })).toHaveFocus()
  })

  it('能区分已是最新版本与暂时获取不到版本', async () => {
    const api = installFakeApi({
      'GET /api/v1/version': { version: 'v0.1.0' },
      'GET /api/v1/update': { current: 'v0.1.0', latest: 'v0.1.0', has_update: false, check_state: 'ok' },
    })
    renderApp(<AppSettingsButton />)
    await userEvent.click(screen.getByRole('button', { name: en.nav.settings }))
    await userEvent.click(await screen.findByRole('tab', { name: en.nav.about }))
    await userEvent.click(screen.getByRole('button', { name: en.update.check }))
    expect(await screen.findByText(en.update.current)).toBeInTheDocument()
    api.set('GET /api/v1/update', { current: 'v0.1.0', latest: '', has_update: false, check_state: 'unreachable' })
    await userEvent.click(screen.getByRole('button', { name: en.update.check }))
    expect(await screen.findByText(en.update.checkUnavailable)).toBeInTheDocument()
  })

  it('切换通用和关于时弹窗保持固定宽高', async () => {
    installFakeApi({ 'GET /api/v1/version': { version: 'v0.1.0' } })
    renderApp(<AppSettingsButton />)
    await userEvent.click(screen.getByRole('button', { name: en.nav.settings }))
    const dialog = await screen.findByRole('dialog', { name: en.nav.settings })
    expect(dialog).toHaveClass('h-[min(34rem,88dvh)]')
    expect(dialog).toHaveClass('w-[min(36rem,96vw)]')
    await userEvent.click(screen.getByRole('tab', { name: en.nav.about }))
    expect(dialog).toHaveClass('h-[min(34rem,88dvh)]')
    expect(dialog).toHaveClass('w-[min(36rem,96vw)]')
  })

  it('模板页签列出模板，重名导入时弹出覆盖确认', async () => {
    const api = installFakeApi({
      'GET /api/v1/templates': [
        { id: 1, name: 'Mine', updated_at: '2020-01-01T00:00:00Z', builtin: false },
        { id: 2, name: 'dev', updated_at: '2020-01-01T00:00:00Z', builtin: true },
      ],
    })
    renderApp(<AppSettingsButton />)
    await userEvent.click(screen.getByRole('button', { name: en.nav.settings }))
    await userEvent.click(await screen.findByRole('tab', { name: en.preferences.templates }))
    expect(await screen.findByText('Mine')).toBeInTheDocument()
    expect(screen.getByText(en.boards.create.templates.dev)).toBeInTheDocument()
    expect(screen.getByText(en.app.templates.builtin)).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: /delete template/i })).toHaveLength(1)
    const file = new File(
      [JSON.stringify({ format: 'slate-project-template', version: 1, name: 'Mine', columns: [] })],
      't.json',
      { type: 'application/json' },
    )
    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    await userEvent.upload(input, file)
    expect(await screen.findByRole('dialog', { name: en.app.templates.replaceTitle })).toBeInTheDocument()
    api.set('POST /api/v1/templates?replace=true', { id: 1, name: 'Mine', updated_at: '2020-01-02T00:00:00Z', tags: [], engineers: [], columns: [] })
    await userEvent.click(screen.getByRole('button', { name: en.app.templates.replaceConfirm }))
    expect(api.calls.some((c) => c.method === 'POST' && c.path === '/api/v1/templates' && c.query.get('replace') === 'true')).toBe(
      true,
    )
  })

  it('取版本失败时显示「未知」，其余内容照常显示', async () => {
    installFakeApi({ 'GET /api/v1/version': () => ({ status: 500, body: { reason: 'ErrInternal', msg: 'boom' } }) })
    renderApp(<AppSettingsButton />)
    await userEvent.click(screen.getByRole('button', { name: en.nav.settings }))
    await userEvent.click(await screen.findByRole('tab', { name: en.nav.about }))
    expect(await screen.findByText(en.about.versionUnknown)).toBeInTheDocument()
    expect(screen.getByText(en.about.intro)).toBeInTheDocument()
  })
})
