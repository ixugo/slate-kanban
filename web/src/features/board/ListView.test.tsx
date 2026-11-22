import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { Route, Routes } from 'react-router-dom'
import { en } from '@/i18n/locales/en'
import { makeCard, makeColumn } from '@/test/factories'
import { renderApp } from '@/test/renderApp'
import ListView from './ListView'

afterEach(() => {
  vi.setSystemTime(new Date('2026-11-18T12:00:00Z'))
})

const columns = [
  makeColumn({ id: 1, name: '待办', sort: 1 }),
  makeColumn({ id: 2, name: 'Done', sort: 2, is_done: true }),
]

const overdue = makeCard({
  id: 1,
  title: '逾期卡',
  column_id: 1,
  column: '待办',
  due_at: '2026-11-01T00:00:00Z',
  done: false,
})
const claimed = makeCard({
  id: 2,
  title: '领取中',
  column_id: 1,
  claim_active: true,
  claimed_by: 'bot-1',
  priority: 3,
})
const hidden = makeCard({ id: 3, title: '被筛掉', column_id: 2 })

/** 渲染列表视图 */
function renderList(
  props: Partial<React.ComponentProps<typeof ListView>> & { route?: string } = {},
) {
  const defaults = {
    columns,
    cards: [overdue, claimed, hidden],
    visibleIds: null as Set<number> | null,
    listAddOpen: false,
    onListAddOpen: vi.fn(),
    onListAddClose: vi.fn(),
    onAddCard: vi.fn(),
  }
  const { route = '/b/1', ...rest } = props
  return renderApp(
    <Routes>
      <Route path="/b/:boardId" element={<ListView {...defaults} {...rest} />} />
    </Routes>,
    { route },
  )
}

describe('ListView', () => {
  it('渲染表头与数据行', async () => {
    renderList()
    expect(screen.getByRole('columnheader', { name: en.list.colTitle })).toBeInTheDocument()
    expect(await screen.findByText('逾期卡')).toBeInTheDocument()
    expect(screen.getByText('领取中')).toBeInTheDocument()
  })

  it('标题链接含 card= 参数', async () => {
    renderList({ route: '/b/1?x=1' })
    const link = await screen.findByRole('link', { name: '逾期卡' })
    expect(link).toHaveAttribute('href', '/b/1?x=1&card=1')
  })

  it('visibleIds 过滤行', async () => {
    renderList({ visibleIds: new Set([1]) })
    expect(await screen.findByText('逾期卡')).toBeInTheDocument()
    expect(screen.queryByText('领取中')).toBeNull()
  })

  it('逾期截止日期红色样式', async () => {
    renderList()
    const row = (await screen.findByText('逾期卡')).closest('tr') as HTMLElement
    const dueCell = within(row).getAllByRole('cell')[5]
    expect(dueCell).toHaveClass('text-destructive')
  })

  it('领取者仅在 claim_active 时显示', async () => {
    renderList()
    const row = (await screen.findByText('领取中')).closest('tr') as HTMLElement
    expect(within(row).getByText('bot-1')).toBeInTheDocument()
  })

  it('点优先级列头切换 aria-sort 与顺序', async () => {
    renderList()
    const user = userEvent.setup()
    const priorityHeader = screen.getByRole('columnheader', { name: en.list.colPriority })
    const btn = within(priorityHeader).getByRole('button')
    expect(priorityHeader).toHaveAttribute('aria-sort', 'none')
    await user.click(btn)
    expect(priorityHeader).toHaveAttribute('aria-sort', 'ascending')
    const rows = screen.getAllByRole('row').slice(1)
    expect(within(rows[0]).getByText('领取中')).toBeInTheDocument()
    await user.click(btn)
    expect(priorityHeader).toHaveAttribute('aria-sort', 'descending')
  })

  it('无任务时显示空状态', () => {
    renderList({ cards: [] })
    expect(screen.getByText(en.list.empty)).toBeInTheDocument()
  })
})