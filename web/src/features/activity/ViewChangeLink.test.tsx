import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { setLanguage } from '@/i18n'
import { en } from '@/i18n/locales/en'
import { installFakeApi } from '@/test/fakeApi'
import { makeCard } from '@/test/factories'
import { renderApp } from '@/test/renderApp'
import type { BoardEvent, CardChange } from '@/lib/api/types'
import { ActivityItem } from './ActivityItem'

const CHANGE_ROUTE = 'GET /api/v1/cards/:id/changes/:id'

// 一次描述修改：第二行改了，末尾多了一行，所以右边新增 2 行、左边删掉 1 行
const change: CardChange = {
  id: 7,
  board_id: 1,
  card_id: 5,
  field: 'description',
  old_value: 'line A\nline B\nline C',
  new_value: 'line A\nline B2\nline C\nline D',
  actor_kind: 'human',
  actor: 'human',
  created_at: '2026-11-18T08:00:00Z',
}

// 造一条「修改了描述」的事件，payload 里带变更记录编号
function descriptionEvent(extra: Partial<BoardEvent> = {}): BoardEvent {
  return {
    id: 1,
    board_id: 1,
    card_id: 5,
    type: 'card.updated',
    actor_kind: 'human',
    actor: 'human',
    payload: { key: 'SLATE-5', title: '旧标题', fields: ['description'], description_change_id: 7 },
    created_at: '2026-11-18T08:00:00Z',
    ...extra,
  }
}

// 渲染活动流里的一条；cards 决定这张卡片「现在还在不在」
function renderItem(event: BoardEvent, cards = [makeCard({ id: 5 })]) {
  const cardsById = new Map(cards.map((c) => [c.id, c]))
  return renderApp(
    <ul>
      <ActivityItem event={event} cardsById={cardsById} onOpenCard={() => {}} />
    </ul>,
  )
}

describe('活动里的「查看变更」', () => {
  it('句子后面是带下划线的链接，点开之前不发请求', () => {
    const api = installFakeApi({ [CHANGE_ROUTE]: change })
    renderItem(descriptionEvent())
    const link = screen.getByRole('button', { name: en.activity.change.link })
    expect(link).toHaveClass('underline')
    expect(link.closest('p')).toHaveTextContent('Me changed Description, view changes')
    expect(api.count(CHANGE_ROUTE)).toBe(0)
  })

  it('中文界面读作「修改了描述，查看变更」，弹窗里的标题也是中文', async () => {
    installFakeApi({ [CHANGE_ROUTE]: change })
    await setLanguage('zh-CN')
    try {
      renderItem(descriptionEvent())
      const link = screen.getByRole('button', { name: '查看变更' })
      expect(link.closest('p')).toHaveTextContent('我 修改了描述，查看变更')
      await userEvent.click(link)
      const dialog = await screen.findByRole('dialog', { name: '描述的变更' })
      expect(await within(dialog).findByText('修改前')).toBeInTheDocument()
      expect(within(dialog).getByText('修改后')).toBeInTheDocument()
    } finally {
      await setLanguage('en')
    }
  })

  it('变更对比弹窗使用固定宽高，短内容也不会缩成小窗', async () => {
    installFakeApi({ [CHANGE_ROUTE]: { ...change, old_value: 'x', new_value: 'y' } })
    renderItem(descriptionEvent())
    await userEvent.click(screen.getByRole('button', { name: en.activity.change.link }))
    const dialog = await screen.findByRole('dialog', { name: en.activity.change.title })
    expect(dialog).toHaveClass('h-[min(72dvh,42rem)]')
    expect(dialog).toHaveClass('w-[min(64rem,96vw)]')
  })

  it('点击后请求这次变更，弹窗里左边旧版本、右边新版本，并统计增删行', async () => {
    const api = installFakeApi({ [CHANGE_ROUTE]: change })
    renderItem(descriptionEvent())
    await userEvent.click(screen.getByRole('button', { name: en.activity.change.link }))

    const dialog = await screen.findByRole('dialog', { name: en.activity.change.title })
    await waitFor(() => expect(within(dialog).getByText(en.activity.change.before)).toBeInTheDocument())
    expect(within(dialog).getByText(en.activity.change.after)).toBeInTheDocument()
    expect(api.calls.map((c) => c.path)).toEqual(['/api/v1/cards/5/changes/7'])
    expect(dialog).toHaveTextContent('Me · ')
    expect(dialog.querySelectorAll('[data-kind="change"]')).toHaveLength(2)
    expect(dialog.querySelectorAll('[data-kind="insert"]')).toHaveLength(2)
    expect(within(dialog).getByText('+2')).toBeInTheDocument()
    expect(within(dialog).getByText('−1')).toBeInTheDocument()
  })

  it('关掉再打开同一条，直接用缓存，不重复请求', async () => {
    const api = installFakeApi({ [CHANGE_ROUTE]: change })
    renderItem(descriptionEvent())
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: en.activity.change.link }))
    const dialog = await screen.findByRole('dialog')
    await within(dialog).findByText(en.activity.change.before)

    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    await user.click(screen.getByRole('button', { name: en.activity.change.link }))
    await within(await screen.findByRole('dialog')).findByText(en.activity.change.before)
    expect(api.count(CHANGE_ROUTE)).toBe(1)
  })

  it('记录已经不存在时给出提示，点重试会再请求一次', async () => {
    let missing = true
    const api = installFakeApi({
      [CHANGE_ROUTE]: () => (missing ? { status: 404, body: { reason: 'ErrCardChangeNotFound', msg: 'gone' } } : { body: change }),
    })
    renderItem(descriptionEvent())
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: en.activity.change.link }))
    const dialog = await screen.findByRole('dialog')
    expect(await within(dialog).findByRole('alert')).toHaveTextContent(en.errors.ErrCardChangeNotFound)

    missing = false
    await user.click(within(dialog).getByRole('button', { name: en.common.retry }))
    await within(dialog).findByText(en.activity.change.before)
    expect(api.count(CHANGE_ROUTE)).toBe(2)
    expect(within(dialog).queryByRole('alert')).toBeNull()
  })

  it('前后内容一样时说明没有区别', async () => {
    installFakeApi({ [CHANGE_ROUTE]: { ...change, old_value: 'same', new_value: 'same' } })
    renderItem(descriptionEvent())
    await userEvent.click(screen.getByRole('button', { name: en.activity.change.link }))
    const dialog = await screen.findByRole('dialog')
    expect(await within(dialog).findByText(en.activity.change.identical)).toBeInTheDocument()
  })

  it('旧事件没有变更记录编号，就没有链接', () => {
    installFakeApi({})
    renderItem(descriptionEvent({ payload: { key: 'SLATE-5', title: '旧标题', fields: ['description'] } }))
    expect(screen.queryByRole('button', { name: en.activity.change.link })).toBeNull()
  })

  it('卡片已经删除时，没有链接', () => {
    installFakeApi({})
    renderItem(descriptionEvent(), [])
    expect(screen.queryByRole('button', { name: en.activity.change.link })).toBeNull()
  })

  it('改的不是描述的事件，没有链接', () => {
    installFakeApi({})
    renderItem(descriptionEvent({ payload: { key: 'SLATE-5', title: 't', fields: ['title'] } }))
    expect(screen.getByText('changed Title')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: en.activity.change.link })).toBeNull()
  })
})
