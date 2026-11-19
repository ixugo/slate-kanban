import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { en } from '@/i18n/locales/en'
import { installFakeApi } from '@/test/fakeApi'
import { renderApp } from '@/test/renderApp'
import { AboutButton } from './AboutButton'
import { PROJECT_URL } from './AboutDialog'

describe('AboutButton', () => {
  it('点开显示介绍、服务端版本号和项目地址链接', async () => {
    installFakeApi({ 'GET /api/v1/version': { version: 'v0.1.0', remark: 'master_abc_261119' } })
    renderApp(<AboutButton />)
    await userEvent.click(screen.getByRole('button', { name: en.nav.about }))
    expect(await screen.findByRole('heading', { name: en.about.title })).toBeInTheDocument()
    expect(screen.getByText(en.about.intro)).toBeInTheDocument()
    expect(await screen.findByText('v0.1.0')).toBeInTheDocument()
    const link = screen.getByRole('link', { name: new RegExp(PROJECT_URL) })
    expect(link).toHaveAttribute('href', 'https://github.com/ixugo/slate-kanban')
    expect(link).toHaveAttribute('rel', expect.stringContaining('noopener'))
  })

  it('取版本失败时显示「未知」，其余内容照常显示', async () => {
    installFakeApi({ 'GET /api/v1/version': () => ({ status: 500, body: { reason: 'ErrInternal', msg: 'boom' } }) })
    renderApp(<AboutButton />)
    await userEvent.click(screen.getByRole('button', { name: en.nav.about }))
    expect(await screen.findByText(en.about.versionUnknown)).toBeInTheDocument()
    expect(screen.getByText(en.about.intro)).toBeInTheDocument()
  })
})
