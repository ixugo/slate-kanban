import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { Route, Routes } from 'react-router-dom'
import { en } from '@/i18n/locales/en'
import { installFakeApi } from '@/test/fakeApi'
import { makeCard, makeSnapshot } from '@/test/factories'
import { renderApp } from '@/test/renderApp'
import { installFakeEventSource } from '@/test/fakeEventSource'
import BoardPage from './BoardPage'
import { boardViewStorageKey } from './useBoardView'

afterEach(() => {
  vi.unstubAllGlobals()
  localStorage.clear()
})

const snapshot = makeSnapshot({
  name: '视图测试',
  cards: [
    makeCard({ id: 1, title: '可见卡', column_id: 1 }),
    makeCard({ id: 2, title: '另一张', column_id: 1 }),
  ],
})

const renderBoard = (route = '/b/1') =>
  renderApp(
    <Routes>
      <Route path="/b/:boardId" element={<BoardPage />} />
    </Routes>,
    { route },
  )

describe('BoardPage 列表视图', () => {
  beforeEach(() => installFakeEventSource())

  it('切换到列表后出现表格', async () => {
    installFakeApi({ 'GET /api/v1/boards/:id/snapshot': snapshot })
    const user = userEvent.setup()
    renderBoard()
    await screen.findByText('可见卡')
    const listButton = screen.getByRole('button', { name: en.board.viewList })
    expect(listButton).toHaveAttribute('aria-pressed', 'false')
    await user.click(listButton)
    expect(await screen.findByRole('table')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: en.board.viewList })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: en.board.viewBoard })).toHaveAttribute('aria-pressed', 'false')
    expect(screen.queryByText(en.board.addCard)).toBeNull()
  })

  it('刷新偏好后仍是列表', async () => {
    localStorage.setItem(boardViewStorageKey(1), 'list')
    installFakeApi({ 'GET /api/v1/boards/:id/snapshot': snapshot })
    renderBoard()
    expect(await screen.findByRole('table')).toBeInTheDocument()
  })

  it('搜索时列表只显示命中任务', async () => {
    const api = installFakeApi({
      'GET /api/v1/boards/:id/snapshot': snapshot,
      'GET /api/v1/boards/:id/cards': { cards: [{ id: 1 }], total: 1, truncated: false },
    })
    localStorage.setItem(boardViewStorageKey(1), 'list')
    const user = userEvent.setup()
    renderBoard()
    await screen.findByRole('table')
    await user.type(screen.getByPlaceholderText(en.search.placeholder), 'tag:x')
    await waitFor(() => expect(api.count('GET /api/v1/boards/:id/cards')).toBeGreaterThan(0), { timeout: 2500 })
    await waitFor(() => expect(screen.getByText('可见卡')).toBeInTheDocument())
    expect(screen.queryByText('另一张')).toBeNull()
  })

  it('列表视图新建任务发 POST 到第一列', async () => {
    const api = installFakeApi({
      'GET /api/v1/boards/:id/snapshot': snapshot,
      'POST /api/v1/boards/:id/cards': makeCard({ id: 9, title: '列表新建' }),
    })
    localStorage.setItem(boardViewStorageKey(1), 'list')
    const user = userEvent.setup()
    renderBoard()
    await screen.findByRole('table')
    await user.click(screen.getByRole('button', { name: en.board.newCard }))
    await user.type(screen.getByLabelText(en.board.addCard), '列表新建{Enter}')
    await waitFor(() => expect(api.count('POST /api/v1/boards/:id/cards')).toBe(1))
    expect(api.calls.find((c) => c.method === 'POST')?.body).toMatchObject({ title: '列表新建', column_id: 1 })
  })
})
