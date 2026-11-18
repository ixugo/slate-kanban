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

  it('删除看板需输入 slug', async () => {
    const api = installFakeApi({
      'GET /api/v1/boards/:id/snapshot': makeSnapshot({ slug: 'SLATE' }),
      'DELETE /api/v1/boards/:id': {},
    })
    renderSettings()
    await screen.findByRole('heading', { name: '示例看板' })
    await userEvent.click(screen.getByRole('button', { name: en.settings.general.deleteBoard }))
    const dialog = await screen.findByRole('alertdialog')
    const del = within(dialog).getByRole('button', { name: en.common.delete })
    expect(del).toBeDisabled()
    await userEvent.type(within(dialog).getByLabelText(/SLATE/), 'SLATE')
    await userEvent.click(del)
    await waitFor(() => expect(api.count('DELETE /api/v1/boards/:id')).toBe(1))
    expect(await screen.findByTestId('home')).toBeInTheDocument()
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