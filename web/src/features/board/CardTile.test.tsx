import { screen, waitFor, within } from '@testing-library/react'
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

/** 弹窗要读任务详情，给一份最小的 */
const cardDetail = (cardId: number) => ({ ...makeCard({ id: cardId, title: cardId === 1 ? 'Alpha' : 'Beta' }), dependencies: [], comments: [] })

// 渲染看板和地址探针；弹窗打开时要读任务详情，所以一并给假接口
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

describe('CardTile 是指向任务的链接', () => {
  beforeEach(() => installFakeEventSource())
  afterEach(() => localStorage.clear())

  it('每张磁贴是 ?card= 链接：沿用地址里的其他参数，且不会被浏览器原生拖走', async () => {
    renderBoard('/b/1?x=1')
    const link = await screen.findByRole('link', { name: '#1 Alpha' })
    expect(link).toHaveAttribute('href', '/b/1?x=1&card=1')
    expect(link).toHaveAttribute('draggable', 'false')
    expect(screen.getByRole('link', { name: '#2 Beta' })).toHaveAttribute('href', '/b/1?x=1&card=2')
  })

  it('编号和领取者同列靠左，未读角标放在该行最右', async () => {
    renderBoard('/b/1', {
      'GET /api/v1/boards/:id/snapshot': makeSnapshot({
        cards: [makeCard({ id: 1, title: 'Alpha', column_id: 1, claim_active: true, claimed_by: 'Orion', unread_count: 4 })],
      }),
    })

    const unread = await screen.findByLabelText('4 unread updates')
    const number = screen.getByText('#1')
    const holder = screen.getByText('Held by Orion')
    expect(number.parentElement).toContainElement(holder)
    expect(number.parentElement?.parentElement?.lastElementChild).toBe(unread)
  })

  it('本地分支在磁贴上展示只读合并状态及提交', async () => {
    renderBoard('/b/1', {
      'GET /api/v1/boards/:id/snapshot': makeSnapshot({
        cards: [
          { ...alpha, key: 'SLATE-1', branch: 'slate/slate-1-alpha', merged_commits: ['abc123', 'def456'] },
          makeCard({ id: 2, title: 'Beta', key: 'SLATE-2', branch: 'slate/slate-2-beta' }),
          makeCard({ id: 3, title: 'Gamma', key: 'SLATE-3', branch: 'fix/gamma' }),
        ],
      }),
    })
    expect(await screen.findByText(en.card.field.merged)).toHaveAttribute('title', 'Merged · abc123, def456')
    expect(screen.getAllByText(en.card.field.mergePending)).toHaveLength(1)
  })

  it('地址里已有 card 时只替换它', async () => {
    renderBoard('/b/1?card=2')
    const link = await screen.findByRole('link', { name: '#1 Alpha' })
    expect(link).toHaveAttribute('href', '/b/1?card=1')
  })

  it('每张磁贴只有一个 Tab 停靠点，里面没有嵌套的可聚焦元素', async () => {
    renderBoard()
    const link = await screen.findByRole('link', { name: '#1 Alpha' })
    expect(link).toHaveAttribute('tabindex', '0')
    expect(link.querySelectorAll('a, button, input, select, textarea, [tabindex]')).toHaveLength(0)
  })

  it('左键点击在应用内打开弹窗，并拦截浏览器的整页跳转', async () => {
    const clicks = recordClickDefaults()
    const user = userEvent.setup()
    renderBoard()
    await user.click(await screen.findByRole('link', { name: '#1 Alpha' }))
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
      const link = await screen.findByRole('link', { name: '#1 Alpha' })
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

  it('回车打开任务', async () => {
    const user = userEvent.setup()
    renderBoard()
    const link = await screen.findByRole('link', { name: '#1 Alpha' })
    link.focus()
    await user.keyboard('{Enter}')
    expect(await screen.findByRole('dialog')).toBeInTheDocument()
    expect(screen.getByTestId('location')).toHaveTextContent('/b/1?card=1')
  })

  it('空格拾起磁贴做键盘拖拽，拖拽中按回车也不会打开任务', async () => {
    const user = userEvent.setup()
    renderBoard()
    const link = await screen.findByRole('link', { name: '#1 Alpha' })
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
    const link = screen.getByRole('link', { name: '#1 Alpha' })
    expect(link).not.toHaveAttribute('aria-disabled')
    await user.click(link)
    expect(await screen.findByRole('dialog')).toBeInTheDocument()
    expect(screen.getByTestId('location')).toHaveTextContent('/b/1?card=1')
  })
})

describe('看板搜索高亮', () => {
  afterEach(() => localStorage.clear())

  it('只标记匹配的正向文字，清空搜索后恢复', async () => {
    const hit = makeCard({ id: 126, title: '搜索 Alpha 与 A.B 预览 open A0B' })
    const user = userEvent.setup()
    renderBoard('/b/1', {
      'GET /api/v1/boards/:id/snapshot': makeSnapshot({ cards: [hit, makeCard({ id: 2, title: 'Beta' })] }),
      'GET /api/v1/boards/:id/cards': { cards: [hit], total: 1, truncated: false },
    })
    const tile = await screen.findByRole('link', { name: `#126 ${hit.title}` })
    const input = screen.getByPlaceholderText(en.search.placeholder)
    expect(tile.querySelector('mark')).toBeNull()

    await user.type(input, '"搜索 alpha" a.b title:预览 is:open -hidden')
    await waitFor(() => expect([...tile.querySelectorAll('mark')].map((item) => item.textContent)).toEqual([
      '搜索 Alpha', 'A.B', '预览',
    ]))
    expect(screen.queryByRole('link', { name: '#2 Beta' })).toBeNull()

    await user.clear(input)
    expect(tile.querySelector('mark')).toBeNull()
    expect(await screen.findByRole('link', { name: '#2 Beta' })).toBeInTheDocument()
  })

  it('按字段标记编号与标签，切换列表视图时沿用同一结果', async () => {
    const tag = { id: 9, text: '浏览器测试', color: 'green-3' }
    const hit = makeCard({ id: 126, title: '检查浏览器测试', tag_ids: [9], tags: [tag.text] })
    const user = userEvent.setup()
    renderBoard('/b/1', {
      'GET /api/v1/boards/:id/snapshot': makeSnapshot({ cards: [hit], tags: [tag] }),
      'GET /api/v1/boards/:id/cards': { cards: [hit], total: 1, truncated: false },
    })
    const tile = await screen.findByRole('link', { name: `#126 ${hit.title}` })
    await user.type(screen.getByPlaceholderText(en.search.placeholder), 'key:slate-126 tag:浏览器测试')
    await waitFor(() => expect([...tile.querySelectorAll('mark')].map((item) => item.textContent)).toEqual(['#126', tag.text]))

    await user.click(screen.getByRole('button', { name: en.board.viewList }))
    const table = await screen.findByRole('table', { name: en.list.caption })
    expect([...table.querySelectorAll('mark')].map((item) => item.textContent)).toEqual(['#126', tag.text])
    expect(within(table).getByRole('link', { name: hit.title })).toBeInTheDocument()
  })
})

describe('CardTile 封面', () => {
  const FIRST = `/api/v1/attachments/${'c'.repeat(32)}`
  const SECOND = `/api/v1/attachments/${'d'.repeat(32)}`

  function renderBoard(description: string) {
    installFakeApi({
      'GET /api/v1/boards/:id/snapshot': makeSnapshot({
        cards: [makeCard({ id: 1, title: 'Alpha', column_id: 1, description })],
      }),
    })
    return renderApp(
      <Routes>
        <Route path="/b/:boardId" element={<BoardPage />} />
      </Routes>,
      { route: '/b/1' },
    )
  }

  it('描述里的第一张图片作为封面放在磁贴顶部，位于标题之前', async () => {
    renderBoard(`说明\n\n![图](${FIRST})\n\n![图二](${SECOND})`)
    const tile = await screen.findByRole('link', { name: /Alpha/ })
    const images = tile.querySelectorAll('img')
    expect(images).toHaveLength(1)
    expect(images[0]).toHaveAttribute('src', FIRST)
    expect(images[0].compareDocumentPosition(screen.getByText('Alpha')) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('没有图片的描述不显示封面', async () => {
    renderBoard('只有文字')
    const tile = await screen.findByRole('link', { name: /Alpha/ })
    expect(tile.querySelector('img')).toBeNull()
  })
})

describe('CardTile 领取提示', () => {
  it('「xx 处理中」紧跟在编号后面，同一行，不在标签那一行；领取过期不显示', async () => {
    installFakeApi({
      'GET /api/v1/boards/:id/snapshot': makeSnapshot({
        cards: [
          makeCard({ id: 1, key: 'TST-7', title: 'Alpha', column_id: 1, claimed_by: 'cursor', claim_active: true, human_only: true }),
          makeCard({ id: 2, key: 'TST-8', title: 'Beta', column_id: 1, claimed_by: 'cursor', claim_active: false }),
        ],
      }),
    })
    renderApp(
      <Routes>
        <Route path="/b/:boardId" element={<BoardPage />} />
      </Routes>,
      { route: '/b/1' },
    )
    const tile = await screen.findByRole('link', { name: /Alpha/ })
    const hint = within(tile).getByText(en.board.claimed.replace('{{name}}', 'cursor'))
    const keyEl = within(tile).getByText('#7')
    expect(hint.closest('div')).toBe(keyEl.closest('div'))
    expect(keyEl.compareDocumentPosition(hint) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(screen.getByRole('link', { name: /Beta/ })).not.toHaveTextContent('cursor')
  })
})
