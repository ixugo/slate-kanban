import { screen } from '@testing-library/react'
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

  it('取版本失败时显示「未知」，其余内容照常显示', async () => {
    installFakeApi({ 'GET /api/v1/version': () => ({ status: 500, body: { reason: 'ErrInternal', msg: 'boom' } }) })
    renderApp(<AppSettingsButton />)
    await userEvent.click(screen.getByRole('button', { name: en.nav.settings }))
    await userEvent.click(await screen.findByRole('tab', { name: en.nav.about }))
    expect(await screen.findByText(en.about.versionUnknown)).toBeInTheDocument()
    expect(screen.getByText(en.about.intro)).toBeInTheDocument()
  })
})
