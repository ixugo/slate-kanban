import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { Route, Routes } from 'react-router-dom'
import { en } from '@/i18n/locales/en'
import { installFakeApi } from '@/test/fakeApi'
import { makeCard, makeSnapshot } from '@/test/factories'
import { renderApp } from '@/test/renderApp'
import { FakeEventSource, installFakeEventSource } from '@/test/fakeEventSource'
import BoardPage from './BoardPage'
import { columnCollapseKey } from './useCollapsedColumns'

afterEach(() => {
  vi.unstubAllGlobals()
  localStorage.clear()
})

const snapshot = makeSnapshot({
  name: '测试看板',
  cards: [
    makeCard({ id: 1, title: 'Alpha', column_id: 1 }),
    makeCard({ id: 2, title: 'Beta', column_id: 2 }),
  ],
})

const renderBoard = (route = '/b/1') =>
  renderApp(
    <Routes>
      <Route path="/b/:boardId" element={<BoardPage />} />
    </Routes>,
    { route },
  )

function observeScroll(list: HTMLElement) {
  let scrollTop = 0
  Object.defineProperty(list, 'scrollHeight', { value: 777, configurable: true })
  Object.defineProperty(list, 'scrollTop', { get: () => scrollTop, set: (v: number) => { scrollTop = v }, configurable: true })
  return () => scrollTop
}

function notifyCardCreated() {
  act(() => FakeEventSource.instances.at(-1)?.emit('board', { type: 'card.created' }))
}

describe('BoardPage', () => {
  beforeEach(() => installFakeEventSource())

  it('渲染列与卡片标题', async () => {
    installFakeApi({ 'GET /api/v1/boards/:id/snapshot': snapshot })
    renderBoard()
    expect(await screen.findByRole('heading', { name: '测试看板' })).toBeInTheDocument()
    expect(screen.getByText('Alpha')).toBeInTheDocument()
    expect(screen.getByText('Beta')).toBeInTheDocument()
  })

  it('顶栏没有导出入口，导出在项目设置里；设置入口仍在', async () => {
    installFakeApi({ 'GET /api/v1/boards/:id/snapshot': snapshot })
    renderBoard()
    await screen.findByText('Alpha')
    expect(screen.queryByRole('button', { name: en.settings.tabs.export })).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: en.settings.tabs.export })).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: en.board.settings })).toHaveAttribute('href', '/b/1/settings')
  })

  it('非法看板编号时提示返回列表', async () => {
    renderBoard('/b/abc')
    expect(await screen.findByText(en.notFound.body)).toBeInTheDocument()
  })

  it('搜索防抖后调用 cards 接口，语法错误显示提示', async () => {
    const api = installFakeApi({
      'GET /api/v1/boards/:id/snapshot': snapshot,
      'GET /api/v1/boards/:id/cards': () => ({
        status: 400,
        body: { reason: 'ErrInvalidInput', msg: '不认识的词 foo' },
      }),
    })
    const user = userEvent.setup()
    renderBoard()
    await screen.findByText('Alpha')
    const search = screen.getByPlaceholderText(en.search.placeholder)
    await user.type(search, 'tag:foo')
    await waitFor(() => expect(api.count('GET /api/v1/boards/:id/cards')).toBeGreaterThan(0), { timeout: 2000 })
    const alerts = await screen.findAllByRole('alert')
    expect(alerts.some((a) => a.textContent?.includes('不认识的词 foo'))).toBe(true)
  })

  it('快捷键 n 打开第一列添加卡片', async () => {
    installFakeApi({ 'GET /api/v1/boards/:id/snapshot': snapshot })
    const user = userEvent.setup()
    renderBoard()
    await screen.findByText('Alpha')
    await user.keyboard('n')
    const col = screen.getByText('To do').closest('section') as HTMLElement
    expect(within(col).getByLabelText(en.board.addCard)).toBeInTheDocument()
  })

  it('新建卡片发送 POST', async () => {
    const api = installFakeApi({
      'GET /api/v1/boards/:id/snapshot': snapshot,
      'POST /api/v1/boards/:id/cards': makeCard({ id: 9, title: '新建' }),
    })
    const user = userEvent.setup()
    renderBoard()
    await screen.findByText('Alpha')
    await user.keyboard('n')
    const input = screen.getByLabelText(en.board.addCard)
    await user.type(input, '新建{Enter}')
    await waitFor(() => expect(api.count('POST /api/v1/boards/:id/cards')).toBe(1))
    expect(api.calls.find((c) => c.method === 'POST')?.body).toMatchObject({ title: '新建', column_id: 1 })
  })

  it('在本页面添加卡片后，该列的卡片列表滚到底，让新卡片看得见', async () => {
    let created = false
    installFakeApi({
      'GET /api/v1/boards/:id/snapshot': () => ({
        body: created ? { ...snapshot, cards: [...snapshot.cards, makeCard({ id: 9, title: '新建', column_id: 1, position: 9999 })] } : snapshot,
      }),
      'POST /api/v1/boards/:id/cards': () => {
        created = true
        return { body: makeCard({ id: 9, title: '新建', column_id: 1, position: 9999 }) }
      },
    })
    const user = userEvent.setup()
    renderBoard()
    const alpha = await screen.findByText('Alpha')
    const list = alpha.closest('.overflow-y-auto') as HTMLElement
    const scrollTop = observeScroll(list)
    await user.keyboard('n')
    await user.type(screen.getByLabelText(en.board.addCard), '新建{Enter}')
    expect(await screen.findByText('新建')).toBeInTheDocument()
    await waitFor(() => expect(scrollTop()).toBe(777))
  })

  it('本地 POST 失败后，远端新卡不会滚动本列', async () => {
    let cards = snapshot.cards
    const api = installFakeApi({
      'GET /api/v1/boards/:id/snapshot': () => ({ body: { ...snapshot, cards } }),
      'POST /api/v1/boards/:id/cards': () => ({ status: 500, body: { reason: 'ErrStore' } }),
    })
    const user = userEvent.setup()
    renderBoard()
    const alpha = await screen.findByText('Alpha')
    const scrollTop = observeScroll(alpha.closest('.overflow-y-auto') as HTMLElement)
    await user.keyboard('n')
    await user.type(screen.getByLabelText(en.board.addCard), 'Failed locally{Enter}')
    expect(await screen.findByText(en.errors.ErrStore)).toBeInTheDocument()
    expect(api.count('POST /api/v1/boards/:id/cards')).toBe(1)

    cards = [...cards, makeCard({ id: 10, column_id: 1, title: 'Remote after failure', position: 10000 })]
    notifyCardCreated()
    expect(await screen.findByText('Remote after failure')).toBeInTheDocument()
    expect(scrollTop()).toBe(0)
  })

  it('本地 POST 等待期间先收到远端新增，只为本地成功的 ID 滚动对应列', async () => {
    let cards = snapshot.cards
    const local = makeCard({ id: 11, column_id: 1, title: 'Local delayed', position: 11000 })
    const api = installFakeApi({
      'GET /api/v1/boards/:id/snapshot': () => ({ body: { ...snapshot, cards } }),
      'POST /api/v1/boards/:id/cards': () => {
        cards = [...cards, local]
        return { body: local }
      },
    })
    const fetch = globalThis.fetch
    let finishPost!: () => void
    const postReady = new Promise<void>((resolve) => { finishPost = resolve })
    vi.stubGlobal('fetch', async (input: RequestInfo | URL, init?: RequestInit) => {
      if (init?.method === 'POST') await postReady
      return fetch(input, init)
    })

    const user = userEvent.setup()
    renderBoard()
    const alpha = await screen.findByText('Alpha')
    const beta = screen.getByText('Beta')
    const scrollTop = observeScroll(alpha.closest('.overflow-y-auto') as HTMLElement)
    const adjacentTop = observeScroll(beta.closest('.overflow-y-auto') as HTMLElement)
    await user.keyboard('n')
    const field = screen.getByLabelText(en.board.addCard)
    await user.type(field, 'Local delayed{Enter}')
    expect(field).toHaveAttribute('readonly')

    cards = [...cards, makeCard({ id: 12, column_id: 1, title: 'Remote first', position: 10000 })]
    notifyCardCreated()
    expect(await screen.findByText('Remote first')).toBeInTheDocument()
    try {
      expect(scrollTop()).toBe(0)
    } finally {
      finishPost()
    }
    expect(await screen.findByText('Local delayed')).toBeInTheDocument()
    await waitFor(() => expect(scrollTop()).toBe(777))
    expect(adjacentTop()).toBe(0)
    expect(api.count('POST /api/v1/boards/:id/cards')).toBe(1)
  })

  it('快照 404 时显示看板不存在', async () => {
    installFakeApi({
      'GET /api/v1/boards/:id/snapshot': () => ({ status: 404, body: { reason: 'ErrBoardNotFound' } }),
    })
    renderBoard()
    expect(await screen.findByText(en.board.gone.title)).toBeInTheDocument()
  })

  it('完成列默认显示为窄边，点击展开', async () => {
    installFakeApi({ 'GET /api/v1/boards/:id/snapshot': snapshot })
    const user = userEvent.setup()
    renderBoard()
    await screen.findByText('Alpha')
    const expandDone = screen.getByRole('button', {
      name: en.board.expandColumn.replace('{{name}}', 'Done').replace('{{count}}', '0'),
    })
    expect(expandDone).toHaveAttribute('aria-expanded', 'false')
    await user.click(expandDone)
    expect(screen.getByRole('heading', { name: 'Done' })).toBeInTheDocument()
    expect(screen.getByLabelText(en.board.collapseColumn.replace('{{name}}', 'Done'))).toBeInTheDocument()
  })

  it('收起按钮可再次收起列', async () => {
    localStorage.setItem(columnCollapseKey(1, 3), '0')
    installFakeApi({ 'GET /api/v1/boards/:id/snapshot': snapshot })
    const user = userEvent.setup()
    renderBoard()
    await screen.findByText('Alpha')
    const collapse = screen.getByLabelText(en.board.collapseColumn.replace('{{name}}', 'Done'))
    await user.click(collapse)
    expect(
      screen.getByRole('button', {
        name: en.board.expandColumn.replace('{{name}}', 'Done').replace('{{count}}', '0'),
      }),
    ).toBeInTheDocument()
  })

  it('刷新后保持列收起偏好', async () => {
    localStorage.setItem(columnCollapseKey(1, 1), '1')
    installFakeApi({ 'GET /api/v1/boards/:id/snapshot': snapshot })
    renderBoard()
    await screen.findByText('Beta')
    expect(
      screen.getByRole('button', {
        name: en.board.expandColumn.replace('{{name}}', 'To do').replace('{{count}}', '1'),
      }),
    ).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'To do' })).toBeNull()
  })

  it('键盘 Enter 展开窄边列', async () => {
    installFakeApi({ 'GET /api/v1/boards/:id/snapshot': snapshot })
    const user = userEvent.setup()
    renderBoard()
    await screen.findByText('Alpha')
    const expandDone = screen.getByRole('button', {
      name: en.board.expandColumn.replace('{{name}}', 'Done').replace('{{count}}', '0'),
    })
    expandDone.focus()
    await user.keyboard('{Enter}')
    expect(screen.getByRole('heading', { name: 'Done' })).toBeInTheDocument()
  })
})