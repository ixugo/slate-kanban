import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { Route, Routes } from 'react-router-dom'
import { en } from '@/i18n/locales/en'
import { installFakeApi, type FakeApi } from '@/test/fakeApi'
import { makeCard, makeColumn, makeSnapshot } from '@/test/factories'
import { renderApp } from '@/test/renderApp'
import { installFakeEventSource } from '@/test/fakeEventSource'
import type { CardView } from '@/lib/api/types'
import BoardPage from './BoardPage'

afterEach(() => {
  vi.unstubAllGlobals()
  localStorage.clear()
})

const alpha = makeCard({ id: 1, title: 'Alpha', column_id: 1, tags: ['bug'] })
const beta = makeCard({ id: 2, title: 'Beta', column_id: 1, tags: ['ui', 'bug'] })
const gamma = makeCard({ id: 3, title: 'Gamma', column_id: 2, claimed_by: 'codex', claim_active: true })
// 列名故意不用起步模板的文案，免得被换成当前语言的说法后和「状态」组里的同名按钮撞上
const columns = [makeColumn({ id: 1, name: 'Backlog' }), makeColumn({ id: 2, name: 'Doing' }), makeColumn({ id: 3, name: 'Shipped', is_done: true })]
const snapshot = makeSnapshot({ name: '测试看板', columns, cards: [alpha, beta, gamma] })

/** 假服务端：任务接口返回 hits 里的任务，调用方可以随时换 hits */
function installBoard(hits: CardView[]): { api: FakeApi; setHits: (next: CardView[]) => void } {
  let current = hits
  const api = installFakeApi({
    'GET /api/v1/boards/:id/snapshot': snapshot,
    'GET /api/v1/boards/:id/cards': () => ({ body: { cards: current, total: current.length, truncated: false } }),
  })
  return { api, setHits: (next) => void (current = next) }
}

/** 发给任务接口的最近一次查询 */
const lastQuery = (api: FakeApi) =>
  api.calls.filter((c) => c.path.endsWith('/cards') && c.method === 'GET').at(-1)?.query.get('query')

const renderBoard = () =>
  renderApp(
    <Routes>
      <Route path="/b/:boardId" element={<BoardPage />} />
    </Routes>,
    { route: '/b/1' },
  )

/** 打开「筛选」弹层 */
async function openFilter(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('button', { name: new RegExp(`^${en.filter.button}`) }))
  return screen.findByText(en.filter.title)
}

describe('筛选弹层', () => {
  beforeEach(() => installFakeEventSource())

  it('点选标签后拼成一条查询发给服务端，只剩命中的任务，并出现「当前筛选」小标签', async () => {
    const { api } = installBoard([alpha, beta])
    const user = userEvent.setup()
    renderBoard()
    await screen.findByText('Gamma')

    await openFilter(user)
    await user.click(screen.getByRole('button', { name: /^bug/ }))
    await user.click(screen.getByRole('button', { name: /^ui/ }))

    await waitFor(() => expect(lastQuery(api)).toBe('tag:bug|ui'))
    await waitFor(() => expect(screen.queryByText('Gamma')).not.toBeInTheDocument())
    expect(screen.getByText('Alpha')).toBeInTheDocument()
    const bar = screen.getByRole('group', { name: en.filter.active })
    expect(within(bar).getByText('Tags: bug')).toBeInTheDocument()
    expect(within(bar).getByText('Tags: ui')).toBeInTheDocument()
  })

  it('选了两个以上标签才出现「任一/全部」，切到「全部」后每个标签各是一个词', async () => {
    const { api } = installBoard([beta])
    const user = userEvent.setup()
    renderBoard()
    await screen.findByText('Gamma')
    await openFilter(user)

    await user.click(screen.getByRole('button', { name: /^bug/ }))
    expect(screen.queryByRole('radio', { name: en.filter.tagMode.all })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: /^ui/ }))
    await user.click(screen.getByRole('radio', { name: en.filter.tagMode.all }))

    await waitFor(() => expect(lastQuery(api)).toBe('tag:bug tag:ui'))
  })

  it('列、状态、优先级、领取者、截止各组一起选：每组一个词，组之间都满足', async () => {
    const { api } = installBoard([gamma])
    const user = userEvent.setup()
    renderBoard()
    await screen.findByText('Gamma')
    await openFilter(user)

    await user.click(screen.getByRole('button', { name: 'Doing' }))
    await user.click(screen.getByRole('button', { name: 'Shipped' }))
    await user.click(screen.getByRole('button', { name: en.filter.states.claimed }))
    await user.click(screen.getByRole('button', { name: en.filter.priority.high }))
    await user.click(screen.getByRole('button', { name: 'codex' }))
    await user.click(screen.getByRole('radio', { name: en.filter.due.overdue }))

    await waitFor(() =>
      expect(lastQuery(api)).toBe('col:2|3 priority:high claimed:codex is:claimed is:overdue'),
    )
    // 工具栏按钮上的角标是选中的条件个数：两列加状态、优先级、领取者、截止各一个
    expect(screen.getByRole('button', { name: new RegExp(`^${en.filter.button}\\s*6$`) })).toBeInTheDocument()
  })

  it('点小标签上的叉取消这一个条件，「清除全部」回到什么都没选', async () => {
    const { api } = installBoard([alpha, beta])
    const user = userEvent.setup()
    renderBoard()
    await screen.findByText('Gamma')
    await openFilter(user)
    await user.click(screen.getByRole('button', { name: /^bug/ }))
    await user.click(screen.getByRole('button', { name: /^ui/ }))
    await waitFor(() => expect(lastQuery(api)).toBe('tag:bug|ui'))
    await user.keyboard('{Escape}')

    await user.click(screen.getByRole('button', { name: en.filter.remove.replace('{{name}}', 'Tags: ui') }))
    await waitFor(() => expect(lastQuery(api)).toBe('tag:bug'))

    await user.click(screen.getByRole('button', { name: en.filter.clearAll }))
    await waitFor(() => expect(screen.getByText('Gamma')).toBeInTheDocument())
    expect(screen.queryByRole('group', { name: en.filter.active })).not.toBeInTheDocument()
  })

  it('搜索框里的关键字和点选的条件合成一条查询：关键字在前', async () => {
    const { api } = installBoard([alpha])
    const user = userEvent.setup()
    renderBoard()
    await screen.findByText('Gamma')

    await user.type(screen.getByPlaceholderText(en.search.placeholder), 'Alpha')
    await openFilter(user)
    await user.click(screen.getByRole('button', { name: en.filter.states.open }))

    await waitFor(() => expect(lastQuery(api)).toBe('Alpha is:open'), { timeout: 2000 })
  })

  it('未完成和已完成互斥：选「已完成」会取消「未完成」', async () => {
    installBoard([])
    const user = userEvent.setup()
    renderBoard()
    await screen.findByText('Gamma')
    await openFilter(user)

    await user.click(screen.getByRole('button', { name: en.filter.states.open }))
    await user.click(screen.getByRole('button', { name: en.filter.states.done }))

    expect(screen.getByRole('button', { name: en.filter.states.open })).toHaveAttribute('aria-pressed', 'false')
    expect(screen.getByRole('button', { name: en.filter.states.done })).toHaveAttribute('aria-pressed', 'true')
  })

  it('条件拼出来超过长度上限时不发请求，直接提示，不会悄悄截断', async () => {
    const tags = Array.from({ length: 20 }, (_, i) => `label-${String(i).padStart(2, '0')}-${'x'.repeat(20)}`)
    const heavy = makeSnapshot({
      name: '测试看板',
      cards: [makeCard({ id: 1, title: 'Alpha', column_id: 1, tags: tags.slice(0, 10) }), makeCard({ id: 2, title: 'Beta', column_id: 1, tags: tags.slice(10) })],
    })
    const api = installFakeApi({
      'GET /api/v1/boards/:id/snapshot': heavy,
      'GET /api/v1/boards/:id/cards': { cards: [], total: 0, truncated: false },
    })
    const user = userEvent.setup()
    renderBoard()
    await screen.findByText('Alpha')
    await openFilter(user)
    for (const tag of tags) await user.click(screen.getByRole('button', { name: new RegExp(`^${tag}`) }))

    expect(await screen.findAllByText(en.filter.tooLong)).not.toHaveLength(0)
    // 还没超长的那些次点击会发请求，但没有任何一次请求带着超长的条件
    const sent = api.calls.filter((c) => c.path.endsWith('/cards')).map((c) => c.query.get('query') ?? '')
    expect(sent.length).toBeGreaterThan(0)
    expect(sent.every((q) => Array.from(q).length <= 500)).toBe(true)
  })
})
