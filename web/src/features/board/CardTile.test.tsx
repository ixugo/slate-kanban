import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { Route, Routes, useLocation } from 'react-router-dom'
import { en } from '@/i18n/locales/en'
import { installFakeApi, type FakeCall, type FakeHandler } from '@/test/fakeApi'
import { makeCard, makeSnapshot } from '@/test/factories'
import { renderApp } from '@/test/renderApp'
import { installFakeEventSource } from '@/test/fakeEventSource'
import BoardPage from './BoardPage'

const alpha = makeCard({ id: 1, title: 'Alpha', column_id: 1 })
const snapshot = makeSnapshot({
  name: '测试看板',
  cards: [alpha, makeCard({ id: 2, title: 'Beta', column_id: 2 })],
})

/** 把当前地址写到页面上，测试靠它判断有没有在应用内跳转 */
function LocationProbe() {
  const { pathname, search } = useLocation()
  return <output data-testid="location">{pathname + search}</output>
}

/** 弹窗要读卡片详情，给一份最小的 */
const cardDetail = (cardId: number) => ({ ...makeCard({ id: cardId, title: cardId === 1 ? 'Alpha' : 'Beta' }), dependencies: [], comments: [] })

// 渲染看板和地址探针；弹窗打开时要读卡片详情，所以一并给假接口
const renderBoard = (route = '/b/1', extraRoutes: Record<string, FakeHandler> = {}) => {
  installFakeApi({
    'GET /api/v1/boards/:id/snapshot': snapshot,
    'GET /api/v1/cards/:id': (call: FakeCall) => ({ body: cardDetail(Number(call.path.split('/').pop())) }),
    ...extraRoutes,
  })
  return renderApp(
    <Routes>
      <Route
        path="/b/:boardId"
        element={
          <>
            <BoardPage />
            <LocationProbe />
          </>
        }
      />
    </Routes>,
    { route },
  )
}

/** 记下每一次点击到达 document 时是否已被页面拦截（拦截了才不会触发浏览器的整页跳转），并把默认行为一并取消，避免 jsdom 报“未实现导航” */
function recordClickDefaults() {
  const seen: boolean[] = []
  const onClick = (e: MouseEvent) => {
    seen.push(e.defaultPrevented)
    e.preventDefault()
  }
  document.addEventListener('click', onClick)
  return { seen, stop: () => document.removeEventListener('click', onClick) }
}

describe('CardTile 是指向卡片的链接', () => {
  beforeEach(() => installFakeEventSource())
  afterEach(() => localStorage.clear())

  it('每张磁贴是 ?card= 链接：沿用地址里的其他参数，且不会被浏览器原生拖走', async () => {
    renderBoard('/b/1?x=1')
    const link = await screen.findByRole('link', { name: 'SLATE-1 Alpha' })
    expect(link).toHaveAttribute('href', '/b/1?x=1&card=1')
    expect(link).toHaveAttribute('draggable', 'false')
    expect(screen.getByRole('link', { name: 'SLATE-2 Beta' })).toHaveAttribute('href', '/b/1?x=1&card=2')
  })

  it('地址里已有 card 时只替换它', async () => {
    renderBoard('/b/1?card=2')
    const link = await screen.findByRole('link', { name: 'SLATE-1 Alpha' })
    expect(link).toHaveAttribute('href', '/b/1?card=1')
  })

  it('每张磁贴只有一个 Tab 停靠点，里面没有嵌套的可聚焦元素', async () => {
    renderBoard()
    const link = await screen.findByRole('link', { name: 'SLATE-1 Alpha' })
    expect(link).toHaveAttribute('tabindex', '0')
    expect(link.querySelectorAll('a, button, input, select, textarea, [tabindex]')).toHaveLength(0)
  })

  it('左键点击在应用内打开弹窗，并拦截浏览器的整页跳转', async () => {
    const clicks = recordClickDefaults()
    const user = userEvent.setup()
    renderBoard()
    await user.click(await screen.findByRole('link', { name: 'SLATE-1 Alpha' }))
    clicks.stop()
    expect(await screen.findByRole('dialog')).toBeInTheDocument()
    expect(screen.getByTestId('location')).toHaveTextContent('/b/1?card=1')
    expect(clicks.seen).toEqual([true])
  })

  it('Ctrl 或 Cmd 加点击不拦截，交给浏览器在新标签页打开', async () => {
    for (const key of ['Control', 'Meta']) {
      const clicks = recordClickDefaults()
      const user = userEvent.setup()
      const { unmount } = renderBoard()
      const link = await screen.findByRole('link', { name: 'SLATE-1 Alpha' })
      await user.keyboard(`{${key}>}`)
      await user.click(link)
      await user.keyboard(`{/${key}}`)
      clicks.stop()
      expect(clicks.seen, key).toEqual([false])
      expect(screen.getByTestId('location')).toHaveTextContent(/^\/b\/1$/)
      expect(screen.queryByRole('dialog')).toBeNull()
      unmount()
    }
  })

  it('回车打开卡片', async () => {
    const user = userEvent.setup()
    renderBoard()
    const link = await screen.findByRole('link', { name: 'SLATE-1 Alpha' })
    link.focus()
    await user.keyboard('{Enter}')
    expect(await screen.findByRole('dialog')).toBeInTheDocument()
    expect(screen.getByTestId('location')).toHaveTextContent('/b/1?card=1')
  })

  it('空格拾起磁贴做键盘拖拽，拖拽中按回车也不会打开卡片', async () => {
    const user = userEvent.setup()
    renderBoard()
    const link = await screen.findByRole('link', { name: 'SLATE-1 Alpha' })
    link.focus()
    await user.keyboard(' ')
    expect(screen.getByTestId('location')).toHaveTextContent(/^\/b\/1$/)
    await user.keyboard('{Enter}')
    expect(screen.getByTestId('location')).toHaveTextContent(/^\/b\/1$/)
    expect(screen.queryByRole('dialog')).toBeNull()
    await user.keyboard('{Escape}')
  })

  it('搜索中不能拖，但磁贴仍然是能点开的链接，也不对读屏标成不可用', async () => {
    const user = userEvent.setup()
    renderBoard('/b/1', {
      'GET /api/v1/boards/:id/cards': { cards: [alpha], total: 1, truncated: false },
    })
    await screen.findByText('Beta')
    await user.type(screen.getByPlaceholderText(en.search.placeholder), 'alpha')
    await waitFor(() => expect(screen.queryByText('Beta')).toBeNull(), { timeout: 3000 })
    expect(screen.getByText(en.board.dragDisabledSearch)).toBeInTheDocument()
    const link = screen.getByRole('link', { name: 'SLATE-1 Alpha' })
    expect(link).not.toHaveAttribute('aria-disabled')
    await user.click(link)
    expect(await screen.findByRole('dialog')).toBeInTheDocument()
    expect(screen.getByTestId('location')).toHaveTextContent('/b/1?card=1')
  })
})

describe('CardTile 封面', () => {
  const COVER = '/api/v1/attachments/' + 'c'.repeat(32)

  it('描述里的第一张图片附件显示在磁贴顶部，没有图片的卡片不显示', async () => {
    installFakeApi({
      'GET /api/v1/boards/:id/snapshot': makeSnapshot({
        cards: [
          makeCard({ id: 1, title: 'Alpha', column_id: 1, description: `说明\n\n![图](${COVER})` }),
          makeCard({ id: 2, title: 'Beta', column_id: 1, description: '没有图' }),
        ],
      }),
    })
    const { container } = renderApp(
      <Routes>
        <Route path="/b/:boardId" element={<BoardPage />} />
      </Routes>,
      { route: '/b/1' },
    )
    const tile = await screen.findByRole('link', { name: /Alpha/ })
    expect(tile.querySelector('img')).toHaveAttribute('src', COVER)
    expect(screen.getByRole('link', { name: /Beta/ }).querySelector('img')).toBeNull()
    expect(container.querySelectorAll('a img')).toHaveLength(1)
  })
})
