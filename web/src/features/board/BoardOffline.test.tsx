import { act, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { Route, Routes, useLocation } from 'react-router-dom'
import { en } from '@/i18n/locales/en'
import { installFakeApi, type FakeCall } from '@/test/fakeApi'
import { makeCard, makeSnapshot } from '@/test/factories'
import { renderApp } from '@/test/renderApp'
import { installFakeEventSource } from '@/test/fakeEventSource'
import BoardPage from './BoardPage'
import { boardViewStorageKey } from './useBoardView'

const snapshot = makeSnapshot({
  name: '离线看板',
  cards: [makeCard({ id: 1, title: 'Alpha', column_id: 1 }), makeCard({ id: 2, title: 'Beta', column_id: 2 })],
})

/** 把当前地址写到页面上，测试靠它判断有没有打开卡片 */
function LocationProbe() {
  const { pathname, search } = useLocation()
  return <output data-testid="location">{pathname + search}</output>
}

/** 假装浏览器离线（useOnlineStatus 的初值取自 navigator.onLine） */
function goOffline() {
  vi.spyOn(window.navigator, 'onLine', 'get').mockReturnValue(false)
}

/** 渲染看板页；弹窗要读卡片详情，所以一并给假接口 */
function renderBoard() {
  installFakeApi({
    'GET /api/v1/boards/:id/snapshot': snapshot,
    'GET /api/v1/cards/:id': (call: FakeCall) => ({
      body: { ...makeCard({ id: Number(call.path.split('/').pop()), title: 'Alpha' }), dependencies: [], comments: [] },
    }),
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
    { route: '/b/1' },
  )
}

describe('离线时看板只读', () => {
  beforeEach(() => installFakeEventSource())
  afterEach(() => {
    vi.restoreAllMocks()
    localStorage.clear()
  })

  it('在线时每列底部有「添加卡片」', async () => {
    renderBoard()
    await screen.findByText('Alpha')
    expect(screen.getAllByRole('button', { name: en.board.addCard }).length).toBeGreaterThan(0)
  })

  it('离线时不提供「添加卡片」，卡片仍能看，也不出现搜索拖动提示', async () => {
    goOffline()
    renderBoard()
    expect(await screen.findByText('Alpha')).toBeInTheDocument()
    expect(screen.getByText('Beta')).toBeInTheDocument()
    expect(screen.queryAllByRole('button', { name: en.board.addCard })).toHaveLength(0)
    expect(screen.queryByText(en.board.dragDisabledSearch)).toBeNull()
  })

  it('离线时拖不动磁贴：空格不会进入拖拽，回车照常打开卡片', async () => {
    goOffline()
    const user = userEvent.setup()
    renderBoard()
    const link = await screen.findByRole('link', { name: 'SLATE-1 Alpha' })
    link.focus()
    await user.keyboard(' ')
    await user.keyboard('{Enter}')
    expect(screen.getByTestId('location')).toHaveTextContent('/b/1?card=1')
  })

  it('离线时列表视图也没有「新建卡片」', async () => {
    localStorage.setItem(boardViewStorageKey(1), 'list')
    goOffline()
    renderBoard()
    await screen.findByRole('table')
    expect(screen.queryAllByRole('button', { name: en.board.newCard })).toHaveLength(0)
  })

  it('页面打开后断网，「添加卡片」随之消失；恢复联网又出现', async () => {
    renderBoard()
    await screen.findByText('Alpha')
    expect(screen.getAllByRole('button', { name: en.board.addCard }).length).toBeGreaterThan(0)
    act(() => {
      window.dispatchEvent(new Event('offline'))
    })
    expect(screen.queryAllByRole('button', { name: en.board.addCard })).toHaveLength(0)
    act(() => {
      window.dispatchEvent(new Event('online'))
    })
    expect(screen.getAllByRole('button', { name: en.board.addCard }).length).toBeGreaterThan(0)
  })
})
