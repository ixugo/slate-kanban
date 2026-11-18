import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes, useLocation } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { en } from '@/i18n/locales/en'
import SettingsPage from '@/features/settings/SettingsPage'
import { installFakeApi } from '@/test/fakeApi'
import { makeSnapshot } from '@/test/factories'
import { renderApp } from '@/test/renderApp'

afterEach(() => vi.unstubAllGlobals())

function LocationProbe() {
  return <output data-testid="location">{useLocation().pathname}</output>
}

const renderSettings = (id = '1') =>
  renderApp(
    <>
      <Routes>
        <Route path="/" element={<p data-testid="home">home</p>} />
        <Route path="/b/:boardId/settings" element={<SettingsPage />} />
      </Routes>
      <LocationProbe />
    </>,
    { route: `/b/${id}/settings` },
  )

describe('SettingsPage', () => {
  it('加载快照并显示看板名', async () => {
    installFakeApi({
      'GET /api/v1/boards/:id/snapshot': makeSnapshot({ name: 'Slate 项目' }),
    })
    renderSettings()
    expect(await screen.findByRole('heading', { name: 'Slate 项目' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: en.settings.tabs.general })).toBeInTheDocument()
  })

  it('「导出」页签给出 JSON 和 Markdown 两个下载链接，指向当前项目的导出接口', async () => {
    installFakeApi({ 'GET /api/v1/boards/:id/snapshot': makeSnapshot({ id: 7 }) })
    renderSettings('7')
    await screen.findByRole('heading', { name: '示例看板' })
    await userEvent.click(screen.getByRole('tab', { name: en.settings.tabs.export }))

    const links = await screen.findAllByRole('link', { name: en.settings.export.download })
    expect(links.map((a) => a.getAttribute('href'))).toEqual([
      '/api/v1/boards/7/export?format=json',
      '/api/v1/boards/7/export?format=md',
    ])
    for (const link of links) expect(link).toHaveAttribute('download')
    expect(screen.getByText(en.settings.export.jsonHint)).toBeInTheDocument()
    expect(screen.getByText(en.settings.export.mdHint)).toBeInTheDocument()
  })

  it('删除项目必须输入项目名称：slug、半截名称、大小写不符都不放行，首尾空白忽略', async () => {
    const api = installFakeApi({
      'GET /api/v1/boards/:id/snapshot': makeSnapshot({ name: 'Slate Kanban', slug: 'SLATE' }),
      'DELETE /api/v1/boards/:id': {},
    })
    renderSettings()
    await screen.findByRole('heading', { name: 'Slate Kanban' })
    await userEvent.click(screen.getByRole('button', { name: en.settings.general.deleteBoard }))
    const dialog = await screen.findByRole('alertdialog')
    const del = within(dialog).getByRole('button', { name: en.common.delete })
    const input = within(dialog).getByLabelText(en.settings.general.confirmName)
    expect(within(dialog).getByText('Slate Kanban')).toBeInTheDocument()
    expect(del).toBeDisabled()

    for (const wrong of ['SLATE', 'Slate', 'slate kanban', 'Slate Kanban!']) {
      await userEvent.clear(input)
      await userEvent.type(input, wrong)
      expect(del, wrong).toBeDisabled()
    }
    await userEvent.clear(input)
    await userEvent.type(input, '  Slate Kanban ')
    expect(del).toBeEnabled()
    await userEvent.click(del)
    await waitFor(() => expect(api.count('DELETE /api/v1/boards/:id')).toBe(1))
    expect(await screen.findByTestId('home')).toBeInTheDocument()
  })

  it('在输入框里回车：名称不对不删、对了才删；取消后重新打开输入框是空的', async () => {
    const api = installFakeApi({
      'GET /api/v1/boards/:id/snapshot': makeSnapshot(),
      'DELETE /api/v1/boards/:id': {},
    })
    renderSettings()
    await screen.findByRole('heading', { name: '示例看板' })
    const openDialog = () => userEvent.click(screen.getByRole('button', { name: en.settings.general.deleteBoard }))

    await openDialog()
    let dialog = await screen.findByRole('alertdialog')
    await userEvent.type(within(dialog).getByLabelText(en.settings.general.confirmName), '示例{Enter}')
    expect(api.count('DELETE /api/v1/boards/:id')).toBe(0)
    await userEvent.click(within(dialog).getByRole('button', { name: en.common.cancel }))
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())

    await openDialog()
    dialog = await screen.findByRole('alertdialog')
    const input = within(dialog).getByLabelText(en.settings.general.confirmName)
    expect(input).toHaveValue('')
    await userEvent.type(input, '示例看板{Enter}')
    await waitFor(() => expect(api.count('DELETE /api/v1/boards/:id')).toBe(1))
  })

  it('删除列有卡片时必须选 move_to', async () => {
    const full = makeSnapshot({
      cards: [
        {
          id: 10,
          board_id: 1,
          column_id: 1,
          number: 10,
          key: 'SLATE-10',
          title: '卡',
          description: '',
          position: 1024,
          priority: 0,
          tags: [],
          human_only: false,
          waits_on: [],
          due_at: null,
          started_at: null,
          finished_at: null,
          claimed_by: '',
          claim_expires_at: null,
          branch: '',
          url: '',
          version: 1,
          created_by: 'human',
          updated_by: 'human',
          created_at: '2026-11-18T08:00:00Z',
          updated_at: '2026-11-18T08:00:00Z',
          column: '待办',
          done: false,
          waiting: false,
          claim_active: false,
        },
      ],
    })
    const api = installFakeApi({
      'GET /api/v1/boards/:id/snapshot': full,
      'DELETE /api/v1/boards/:id/columns/:columnId': {},
    })
    renderSettings()
    await screen.findByRole('heading', { name: '示例看板' })
    await userEvent.click(screen.getByRole('tab', { name: en.settings.tabs.columns }))
    const deleteButtons = await screen.findAllByRole('button', { name: en.common.delete })
    await userEvent.click(deleteButtons[0])
    const dialog = await screen.findByRole('alertdialog')
    expect(dialog).toHaveTextContent(en.settings.columns.deleteBodyMove.replace('{{count}}', '1'))
    await userEvent.click(within(dialog).getByRole('button', { name: en.common.delete }))
    await waitFor(() => expect(api.count('DELETE /api/v1/boards/:id/columns/:columnId')).toBe(1))
    api.expectNoUnmatched()
  })
})