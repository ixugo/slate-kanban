import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { Route, Routes } from 'react-router-dom'
import { en } from '@/i18n/locales/en'
import { installFakeApi } from '@/test/fakeApi'
import { makeCard, makeSnapshot } from '@/test/factories'
import { renderApp } from '@/test/renderApp'
import { installFakeEventSource } from '@/test/fakeEventSource'
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
    const col = screen.getByText('待办').closest('section') as HTMLElement
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
      name: en.board.expandColumn.replace('{{name}}', '完成').replace('{{count}}', '0'),
    })
    expect(expandDone).toHaveAttribute('aria-expanded', 'false')
    await user.click(expandDone)
    expect(screen.getByRole('heading', { name: '完成' })).toBeInTheDocument()
    expect(screen.getByLabelText(en.board.collapseColumn.replace('{{name}}', '完成'))).toBeInTheDocument()
  })

  it('收起按钮可再次收起列', async () => {
    localStorage.setItem(columnCollapseKey(1, 3), '0')
    installFakeApi({ 'GET /api/v1/boards/:id/snapshot': snapshot })
    const user = userEvent.setup()
    renderBoard()
    await screen.findByText('Alpha')
    const collapse = screen.getByLabelText(en.board.collapseColumn.replace('{{name}}', '完成'))
    await user.click(collapse)
    expect(
      screen.getByRole('button', {
        name: en.board.expandColumn.replace('{{name}}', '完成').replace('{{count}}', '0'),
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
        name: en.board.expandColumn.replace('{{name}}', '待办').replace('{{count}}', '1'),
      }),
    ).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: '待办' })).toBeNull()
  })

  it('键盘 Enter 展开窄边列', async () => {
    installFakeApi({ 'GET /api/v1/boards/:id/snapshot': snapshot })
    const user = userEvent.setup()
    renderBoard()
    await screen.findByText('Alpha')
    const expandDone = screen.getByRole('button', {
      name: en.board.expandColumn.replace('{{name}}', '完成').replace('{{count}}', '0'),
    })
    expandDone.focus()
    await user.keyboard('{Enter}')
    expect(screen.getByRole('heading', { name: '完成' })).toBeInTheDocument()
  })
})