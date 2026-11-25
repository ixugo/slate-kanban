import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import BoardPage from '@/features/board/BoardPage'
import { en } from '@/i18n/locales/en'
import type { ActivityActor, ActorKind, BoardEvent } from '@/lib/api/types'
import { installFakeApi, type FakeCall } from '@/test/fakeApi'
import { FakeEventSource } from '@/test/fakeEventSource'
import { makeCard, makeSnapshot } from '@/test/factories'
import { renderApp } from '@/test/renderApp'
import { ACTIVITY_PAGE_SIZE } from './useActivity'

const ACTIVITY = 'GET /api/v1/boards/:id/activity'

const snapshot = makeSnapshot({
  name: '活动测试',
  cards: [makeCard({ id: 1, title: '任务一', column_id: 1 }), makeCard({ id: 2, title: '任务二', column_id: 2 })],
})

// 造一条事件
function ev(id: number, type: string, kind: ActorKind, actor: string, payload: Record<string, unknown> = {}, cardId = 0): BoardEvent {
  return { id, board_id: 1, card_id: cardId, type, actor_kind: kind, actor, payload, created_at: '2026-11-18T08:00:00Z' }
}

// 默认的四条事件：人建卡、规则触发、agent 评论、人移动任务（序号越大越新）
function defaultEvents(): BoardEvent[] {
  return [
    ev(1, 'card.created', 'human', 'human', { key: 'SLATE-1', title: '任务一', column: 'Todo' }, 1),
    ev(2, 'automation.fired', 'system', 'automation:开始计时', { key: 'SLATE-1', actions: ['set_started_at'], skipped: [] }, 1),
    ev(3, 'comment.added', 'agent', 'codex', { key: 'SLATE-1', snippet: '收到，开始做' }, 1),
    ev(4, 'card.moved', 'human', 'human', { key: 'SLATE-2', title: '任务二', from: 'Todo', to: 'Doing' }, 2),
  ]
}

// 假服务端的活动流接口：按和真服务端一致的规则筛选、从新到旧、用 before 翻页。events 可以在测试里随时增减。
function activityHandler(events: BoardEvent[]) {
  return (call: FakeCall) => {
    const types = call.query.get('types')?.split(',')
    const excluded = call.query.get('exclude_types')?.split(',') ?? []
    const kind = call.query.get('actor_kind')
    const actor = call.query.get('actor')
    const before = Number(call.query.get('before') ?? 0)
    const limit = Number(call.query.get('limit') ?? 50)
    const matched = [...events]
      .sort((a, b) => b.id - a.id)
      .filter((e) => !excluded.includes(e.type) && (!types || types.includes(e.type)) && (!kind || e.actor_kind === kind) && (!actor || e.actor === actor))
      .filter((e) => before === 0 || e.id < before)
    const actors: ActivityActor[] = []
    for (const e of events) {
      if (!actors.some((a) => a.kind === e.actor_kind && a.name === e.actor)) actors.push({ kind: e.actor_kind, name: e.actor })
    }
    return {
      body: {
        events: matched.slice(0, limit),
        has_more: matched.length > limit,
        latest: Math.max(0, ...events.map((e) => e.id)),
        actors,
      },
    }
  }
}

// 渲染看板页并安装假服务端；返回假服务端、事件列表和已配置好的用户操作对象
function setup(events: BoardEvent[] = defaultEvents(), extra: Record<string, unknown> = {}) {
  const api = installFakeApi({
    'GET /api/v1/boards/:id/snapshot': snapshot,
    [ACTIVITY]: activityHandler(events),
    ...extra,
  })
  const user = userEvent.setup()
  renderApp(
    <Routes>
      <Route path="/b/:boardId" element={<BoardPage />} />
    </Routes>,
    { route: '/b/1' },
  )
  return { api, events, user }
}

// 从工具栏的更多菜单打开「活动」，等抽屉出现，返回抽屉元素
async function openDrawer(user: ReturnType<typeof userEvent.setup>) {
  await screen.findByText('任务一')
  await user.click(screen.getByRole('button', { name: en.common.more }))
  await user.click(screen.getByRole('menuitem', { name: en.activity.open }))
  return screen.findByRole('dialog', { name: en.activity.title })
}

// 抽屉里当前显示的每一条事件的文字
const rowTexts = (dialog: HTMLElement) => within(dialog).queryAllByRole('listitem').map((li) => li.textContent ?? '')

// 最近一次读活动流的请求
const lastActivityCall = (api: ReturnType<typeof installFakeApi>) =>
  api.calls.filter((c) => c.path.endsWith('/activity')).at(-1) as FakeCall

describe('活动抽屉', () => {
  it('从更多菜单打开活动抽屉，从新到旧列出谁做了什么', async () => {
    const { user } = setup()
    const dialog = await openDrawer(user)
    await within(dialog).findByText(/moved the task/)
    const rows = rowTexts(dialog)
    expect(rows).toHaveLength(4)
    expect(rows[0]).toContain('Me')
    expect(rows[0]).toContain('moved the task from “Todo” to “Doing”')
    expect(rows[0]).toContain('#2 任务二')
    expect(rows[1]).toContain('codex')
    expect(rows[1]).toContain('commented')
    expect(rows[1]).toContain('收到，开始做')
    expect(rows[2]).toContain('Automation · 开始计时')
    expect(rows[2]).toContain('ran the rule')
    expect(rows[2]).toContain('Set started time')
    expect(rows[3]).toContain('created the task in “Todo”')
  })

  it('桌面领取显示当时的工程师，旧记录用名册回退，普通领取显示 agent', async () => {
    const desktop = 'desktop-5xfr3jfu5z2sitwvgckxdkj3n6'
    const noEngineer = 'desktop-aaaaaaaaaaaaaaaaaaaaaaaaaa'
    const { user } = setup([
      ev(1, 'card.claimed', 'agent', desktop, { engineer_id: 1, engineer_name: 'Atlas' }, 1),
      ev(2, 'card.claimed', 'agent', desktop, { engineer_id: 2 }, 2),
      ev(3, 'card.claimed', 'agent', 'codex', { engineer_id: 1 }, 1),
      ev(4, 'card.claimed', 'agent', desktop, { engineer_id: 2, engineer_name: '' }, 2),
      ev(5, 'card.claimed', 'agent', noEngineer, { engineer_id: 99 }, 1),
    ], {
      'GET /api/v1/boards/:id/snapshot': {
        ...snapshot,
        engineers: [
          { id: 1, name: 'Atlas Renamed', kind: 'dev', prompt: '', avatar: '' },
          { id: 2, name: 'Orion', kind: 'dev', prompt: '', avatar: '' },
        ],
      },
    })
    const dialog = await openDrawer(user)
    await within(dialog).findAllByText(en.activity.event.cardClaimed)
    expect(rowTexts(dialog).map((row) => row.split(' claimed the task')[0]))
      .toEqual([noEngineer, desktop, 'codex', 'Orion', 'Atlas'])
  })

  it('第一次请求不带筛选条件，每页读固定条数', async () => {
    const { api, user } = setup()
    await openDrawer(user)
    await screen.findByText(/moved the task/)
    const call = lastActivityCall(api)
    expect(call.query.get('limit')).toBe(String(ACTIVITY_PAGE_SIZE))
    expect(call.query.get('exclude_types')).toBe('card.tags_updated,board.labels_updated')
    for (const key of ['before', 'types', 'actor_kind', 'actor']) expect(call.query.has(key), key).toBe(false)
  })

  it('点事件类型的分组只看这些类型，再点一下取消，「清除筛选」一键复位', async () => {
    const { api, user } = setup()
    const dialog = await openDrawer(user)
    await within(dialog).findByText(/moved the task/)

    await user.click(within(dialog).getByRole('button', { name: en.activity.groups.move }))
    await waitFor(() => expect(lastActivityCall(api).query.get('types')).toBe('card.moved,card.reordered'))
    await waitFor(() => expect(rowTexts(dialog)).toHaveLength(1))
    expect(within(dialog).getByRole('button', { name: en.activity.groups.move })).toHaveAttribute('aria-pressed', 'true')

    await user.click(within(dialog).getByRole('button', { name: en.activity.groups.comment }))
    await waitFor(() =>
      expect(lastActivityCall(api).query.get('types')).toBe('card.moved,card.reordered,comment.added,comment.deleted'),
    )
    await waitFor(() => expect(rowTexts(dialog)).toHaveLength(2))

    await user.click(within(dialog).getByRole('button', { name: en.activity.groups.move }))
    await waitFor(() => expect(lastActivityCall(api).query.get('types')).toBe('comment.added,comment.deleted'))
    await waitFor(() => expect(rowTexts(dialog)).toHaveLength(1))

    await user.click(within(dialog).getByRole('button', { name: en.activity.clearFilters }))
    await waitFor(() => expect(lastActivityCall(api).query.has('types')).toBe(false))
    await waitFor(() => expect(rowTexts(dialog)).toHaveLength(4))
  })

  it('按操作者筛选：人、agent、规则各一个选项，选「所有人」复位', async () => {
    const { api, user } = setup()
    const dialog = await openDrawer(user)
    await within(dialog).findByText(/moved the task/)
    const group = within(dialog).getByRole('radiogroup', { name: en.activity.actorTitle })
    expect(within(group).getAllByRole('radio').map((r) => r.textContent)).toEqual([
      en.activity.anyone,
      'Me',
      'Automation · 开始计时',
      'codex',
    ])
    expect(within(group).getByRole('radio', { name: en.activity.anyone })).toBeChecked()

    await user.click(within(group).getByRole('radio', { name: 'codex' }))
    await waitFor(() => expect(lastActivityCall(api).query.get('actor_kind')).toBe('agent'))
    expect(lastActivityCall(api).query.get('actor')).toBe('codex')
    await waitFor(() => expect(rowTexts(dialog)).toHaveLength(1))
    expect(within(group).getByRole('radio', { name: 'codex' })).toBeChecked()

    await user.click(within(group).getByRole('radio', { name: 'Automation · 开始计时' }))
    await waitFor(() => expect(lastActivityCall(api).query.get('actor_kind')).toBe('system'))
    expect(lastActivityCall(api).query.get('actor')).toBe('automation:开始计时')

    await user.click(within(group).getByRole('radio', { name: en.activity.anyone }))
    await waitFor(() => expect(lastActivityCall(api).query.has('actor')).toBe(false))
    await waitFor(() => expect(rowTexts(dialog)).toHaveLength(4))
  })

  it('事件类型和操作者可以叠加，没有结果时提示并能清除', async () => {
    const { api, user } = setup()
    const dialog = await openDrawer(user)
    await within(dialog).findByText(/moved the task/)
    await user.click(within(dialog).getByRole('button', { name: en.activity.groups.move }))
    await user.click(within(dialog).getByRole('radio', { name: 'codex' }))
    await waitFor(() => expect(lastActivityCall(api).query.get('actor')).toBe('codex'))
    expect(lastActivityCall(api).query.get('types')).toBe('card.moved,card.reordered')
    expect(await within(dialog).findByText(en.activity.emptyFiltered)).toBeInTheDocument()
    expect(rowTexts(dialog)).toHaveLength(0)

    const clear = within(dialog).getAllByRole('button', { name: en.activity.clearFilters })
    await user.click(clear[clear.length - 1])
    await waitFor(() => expect(rowTexts(dialog)).toHaveLength(4))
  })

  it('一页放不下时出现「加载更早的活动」，用最旧一条的序号往前翻，翻完按钮消失', async () => {
    const many = Array.from({ length: ACTIVITY_PAGE_SIZE + 5 }, (_, i) => ev(i + 1, 'card.deleted', 'human', 'human', { key: `SLATE-${i + 1}`, title: `旧卡 ${i + 1}` }))
    const { api, user } = setup(many)
    const dialog = await openDrawer(user)
    await waitFor(() => expect(rowTexts(dialog)).toHaveLength(ACTIVITY_PAGE_SIZE))
    expect(rowTexts(dialog)[0]).toContain('旧卡 35')

    await user.click(within(dialog).getByRole('button', { name: en.activity.loadMore }))
    await waitFor(() => expect(rowTexts(dialog)).toHaveLength(ACTIVITY_PAGE_SIZE + 5))
    expect(lastActivityCall(api).query.get('before')).toBe('6')
    expect(rowTexts(dialog)[ACTIVITY_PAGE_SIZE + 4]).toContain('旧卡 1')
    expect(within(dialog).queryByRole('button', { name: en.activity.loadMore })).toBeNull()
  })

  it('抽屉开着时有新事件，列表实时出现新的一条', async () => {
    const { api, events, user } = setup()
    const dialog = await openDrawer(user)
    await within(dialog).findByText(/moved the task/)
    const before = api.count(ACTIVITY)

    events.push(ev(5, 'comment.deleted', 'human', 'human', { key: 'SLATE-1' }, 1))
    FakeEventSource.instances[0].emit('board', { type: 'comment.deleted' })

    await waitFor(() => expect(rowTexts(dialog)).toHaveLength(5))
    expect(rowTexts(dialog)[0]).toContain('deleted a comment')
    expect(api.count(ACTIVITY)).toBeGreaterThan(before)
  })

  it('抽屉关着时看板有新事件，不会去读活动流', async () => {
    const { api, user } = setup()
    const dialog = await openDrawer(user)
    await within(dialog).findByText(/moved the task/)
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog', { name: en.activity.title })).toBeNull())
    const before = api.count(ACTIVITY)

    FakeEventSource.instances[0].emit('board', { type: 'card.created' })
    await new Promise((resolve) => setTimeout(resolve, 500))
    expect(api.count(ACTIVITY)).toBe(before)
  })

  it('点还在看板上的任务：抽屉关闭并打开这个任务；已删除的任务只显示文字', async () => {
    const events = [...defaultEvents(), ev(5, 'card.deleted', 'human', 'human', { key: 'SLATE-9', title: '已删的卡' }, 9)]
    const detail = { ...makeCard({ id: 2, title: '任务二', column_id: 2 }), column_instruction: '', dependencies: [], comments: [] }
    const { api, user } = setup(events, { 'GET /api/v1/cards/:id': detail })
    const dialog = await openDrawer(user)
    await within(dialog).findByText(/deleted the task/)
    expect(within(dialog).queryByRole('link', { name: /已删的卡/ })).toBeNull()
    expect(within(dialog).getByText('#9 已删的卡')).toBeInTheDocument()

    const link = within(dialog).getByRole('link', { name: '#2 任务二' })
    expect(link).toHaveAttribute('href', '/b/1?card=2')
    await user.click(link)
    // 任务弹窗一打开，抽屉会被标成隐藏，按角色查不到它，所以直接看抽屉的节点是不是真的卸掉了
    await waitFor(() => expect(document.querySelector('[data-slot="sheet-content"]')).toBeNull())
    await waitFor(() => expect(api.count('GET /api/v1/cards/:id')).toBeGreaterThan(0))
  })

  it('读取失败时显示错误和重试按钮，重试成功后出现列表', async () => {
    const events = defaultEvents()
    const ok = activityHandler(events)
    let failing = true
    const { user } = setup(events, {
      [ACTIVITY]: (call: FakeCall) => (failing ? { status: 429, body: { reason: 'ErrTooManyRequests', msg: 'x' } } : ok(call)),
    })
    const dialog = await openDrawer(user)
    expect(await within(dialog).findByRole('alert')).toHaveTextContent(en.errors.ErrTooManyRequests)
    failing = false
    await user.click(within(dialog).getByRole('button', { name: en.common.retry }))
    await waitFor(() => expect(rowTexts(dialog)).toHaveLength(4))
    expect(within(dialog).queryByRole('alert')).toBeNull()
  })

  it('没有任何事件时提示还没有活动', async () => {
    const { user } = setup([])
    const dialog = await openDrawer(user)
    expect(await within(dialog).findByText(en.activity.empty)).toBeInTheDocument()
    expect(within(dialog).queryByRole('radiogroup')).toBeNull()
  })
})
