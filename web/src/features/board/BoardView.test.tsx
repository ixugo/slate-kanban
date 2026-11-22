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
import { LIST_PREFETCH_DELAY_MS, prefetchListView } from './prefetchListView'
import { boardViewStorageKey } from './useBoardView'

// 预取只是为了让 Service Worker 缓存列表视图的代码，测试里换成可观察的假函数
vi.mock('./prefetchListView', () => ({ LIST_PREFETCH_DELAY_MS: 300, prefetchListView: vi.fn() }))

afterEach(() => {
  vi.unstubAllGlobals()
  localStorage.clear()
  vi.mocked(prefetchListView).mockClear()
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

  it('打开看板后延迟预取列表视图的代码，不阻塞首屏', async () => {
    installFakeApi({ 'GET /api/v1/boards/:id/snapshot': snapshot })
    // 记下预取被调用的时刻，用“距离打开看板过了多久”判断是否延迟；
    // 不靠“渲染完那一刻还没被调用”，因为机器繁忙时渲染可能比延迟还慢，会误判
    let calledAt = 0
    vi.mocked(prefetchListView).mockImplementation(() => {
      calledAt = performance.now()
    })
    const openedAt = performance.now()
    renderBoard()
    await screen.findByText('可见卡')
    expect(LIST_PREFETCH_DELAY_MS).toBeGreaterThan(0)
    await waitFor(() => expect(prefetchListView).toHaveBeenCalledTimes(1))
    // 计时器偶尔会比约定早触发 1 毫秒左右，留 5 毫秒余量
    expect(calledAt - openedAt).toBeGreaterThanOrEqual(LIST_PREFETCH_DELAY_MS - 5)
  })
})
