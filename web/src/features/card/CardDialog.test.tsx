import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import { Route, Routes } from 'react-router-dom'
import i18n from '@/i18n'
import { en } from '@/i18n/locales/en'
import { ja } from '@/i18n/locales/ja'
import { zhCN } from '@/i18n/locales/zh-CN'
import { installFakeApi, type FakeCall } from '@/test/fakeApi'
import { makeCard, makeSnapshot } from '@/test/factories'
import { renderApp } from '@/test/renderApp'
import { installFakeEventSource } from '@/test/fakeEventSource'
import BoardPage from '@/features/board/BoardPage'
import { CARD_TAGS_MAX } from '@/lib/limits'
import { formatDate, formatDateTime } from '@/lib/format'
import type { Comment, Engineer, Tag, TagInput } from '@/lib/api/types'

const baseDetail = {
  ...makeCard({ id: 1, title: '详情卡', version: 2, tag_ids: [1], tags: ['bug'] }),
  column_instruction: 'do work',
  dependencies: [],
  comments: [] as Comment[],
  started_at: '2026-11-01T10:00:00Z' as string | null,
  finished_at: null as string | null,
  claim_active: true,
  claimed_by: 'codex',
  claim_expires_at: '2026-11-18T12:00:00Z',
}

// 看板页里的任务弹窗是懒加载的：先把它的模块加载好，第一个测试就不用在 findBy 的限时里等模块首次编译。
beforeAll(async () => {
  await import('@/features/card/CardDialog')
}, 30000)

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

/** 取第一次 PATCH 请求的请求体 */
function patchBody(calls: FakeCall[]): unknown {
  return calls.find((c) => c.method === 'PATCH')?.body
}

function openBoardWithCard(detail: typeof baseDetail, events: object[] = [], library: Tag[] = [], engineers: Engineer[] = []) {
  installFakeEventSource()
  let current = { ...detail }
  const api = installFakeApi({
    'GET /api/v1/boards/:id/snapshot': makeSnapshot({ tags: library, engineers, cards: [makeCard({ id: 1 })] }),
    'GET /api/v1/cards/:id': () => ({ body: current }),
    'PATCH /api/v1/cards/:id': (call: FakeCall) => {
      current = {
        ...current,
        ...(call.body as object),
        version: current.version + 1,
        dependencies: current.dependencies ?? [],
        comments: current.comments ?? [],
        tag_ids: (call.body as { tag_ids?: number[] }).tag_ids ?? current.tag_ids ?? [],
        waits_on: (call.body as { waits_on?: number[] }).waits_on ?? current.waits_on ?? [],
      }
      return { body: current }
    },
    'PATCH /api/v1/boards/:id': (call: FakeCall) => ({
      body: { id: 1, tags: (call.body as { tags: TagInput[] }).tags.map((tag, i) => ({ ...tag, id: tag.id ?? 100 + i })) },
    }),
    'POST /api/v1/cards/:id/release': { body: {} },
    'GET /api/v1/boards/:id/activity': { events, has_more: false, latest: 0, actors: [] },
  })
  renderApp(
    <Routes>
      <Route path="/b/:boardId" element={<BoardPage />} />
    </Routes>,
    { route: '/b/1?card=1' },
  )
  return api
}

function openCardReferences(commentBody = 'Comment #25, `#38`, [link #44](https://example.com) and \\#55.') {
  const root = {
    ...baseDetail,
    title: 'First card',
    description: 'Read #14 and #99.',
    comments: [{
      id: 1,
      board_id: 1,
      card_id: 1,
      author_kind: 'human',
      author: 'human',
      body: commentBody,
      created_at: '2026-11-18T10:00:00Z',
      updated_at: '2026-11-18T10:00:00Z',
    }],
  }
  const second = { ...baseDetail, id: 22, number: 14, key: 'TST-14', title: 'Second card', description: 'Read #25.', tag_ids: [7], due_at: '2026-12-01T00:00:00Z' }
  const third = { ...baseDetail, id: 33, number: 25, key: 'TST-25', title: 'Third card', description: '' }
  const api = installFakeApi({
    'GET /api/v1/boards/:id/snapshot': makeSnapshot({
      tags: [{ id: 7, text: 'Urgent', color: 'red-3' }],
      cards: [root, second, third],
    }),
    'GET /api/v1/cards/1': root,
    'GET /api/v1/cards/22': second,
    'GET /api/v1/cards/33': third,
    'GET /api/v1/boards/:id/activity': { events: [], has_more: false, latest: 0, actors: [] },
  })
  renderApp(
    <Routes>
      <Route path="/b/:boardId" element={<BoardPage />} />
    </Routes>,
    { route: '/b/1?card=1' },
  )
  return api
}

describe('CardDialog', () => {
  it('依赖弹层保持打开，首个保存未返回时仍可勾选第二项', async () => {
    const api = openCardReferences()
    let current = { ...baseDetail, waits_on: [] as number[] }
    api.set('GET /api/v1/cards/1', () => ({ body: current }))
    api.set('PATCH /api/v1/cards/1', (call: FakeCall) => {
      const input = call.body as { version: number; waits_on: number[] }
      if (input.version !== current.version) return { status: 409, body: { reason: 'ErrCardVersionConflict' } }
      current = { ...current, waits_on: input.waits_on, version: current.version + 1 }
      return { body: current }
    })
    const originalFetch = vi.mocked(fetch).getMockImplementation()!
    let finishFirst!: () => void
    const firstResponse = new Promise<void>((resolve) => { finishFirst = resolve })
    let firstPatch = true
    vi.mocked(fetch).mockImplementation(async (...args) => {
      if (args[1]?.method === 'PATCH' && firstPatch) {
        firstPatch = false
        await firstResponse
      }
      return originalFetch(...args)
    })
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: en.card.field.deps }))
    const search = await screen.findByRole('textbox', { name: en.card.depSearch })
    const list = () => within(screen.getByRole('list', { name: en.card.depAdd }))
    try {
      await user.click(list().getByLabelText('#14 Second card'))
      expect(screen.getByRole('textbox', { name: en.card.depSearch })).toBe(search)
      const second = list().getByLabelText('#25 Third card')
      await user.click(second)
      expect(second).toHaveAttribute('aria-checked', 'true')
    } finally {
      finishFirst()
    }
    await waitFor(() => expect(current.waits_on).toEqual([22, 33]))
    expect(api.calls.filter((call) => call.method === 'PATCH').map((call) => call.body)).toEqual([
      { version: 2, waits_on: [22] },
      { version: 3, waits_on: [22, 33] },
    ])
  })

  it('首次点击归档即发送请求，并在当前详情显示归档状态', async () => {
    const api = openBoardWithCard(baseDetail)
    const archiveRoute = 'POST /api/v1/cards/:id/archive'
    const archived = { ...baseDetail, archived_at: '2026-11-23T03:00:00Z', claim_active: false }
    api.set(archiveRoute, () => {
      api.set('GET /api/v1/cards/:id', archived)
      return { body: archived }
    })
    const user = userEvent.setup()
    const dialog = await screen.findByRole('dialog')
    const more = await within(dialog).findByRole('button', { name: en.common.more })
    await user.click(more)
    await user.click(within(dialog).getByRole('button', { name: en.card.archive.action }))

    await waitFor(() => expect(api.count(archiveRoute)).toBe(1))
    expect(await within(dialog).findByText(en.card.archive.readOnly)).toBeInTheDocument()
    expect(within(dialog).queryByRole('button', { name: en.card.archive.action })).toBeNull()
    expect(more).toHaveFocus()
    expect(within(dialog).getByRole('button', { name: en.common.delete })).toHaveAttribute('data-variant', 'destructive')
    api.expectNoUnmatched()
  })

  it('评论里的编号打开同板任务，Markdown 链接、代码和转义保留', async () => {
    const api = openCardReferences()
    const user = userEvent.setup()
    const dialog = await screen.findByRole('dialog')
    expect(await within(dialog).findByRole('link', { name: 'link #44' })).toHaveAttribute('href', 'https://example.com')
    expect(within(dialog).queryByRole('button', { name: /^#(38|44|55)$/ })).toBeNull()
    const reference = await within(dialog).findByRole('button', { name: '#25' })
    expect(reference).toHaveAccessibleDescription('Third card')
    reference.focus()
    await user.keyboard('{Enter}')
    expect(await within(dialog).findByDisplayValue('Third card')).toBeInTheDocument()
    expect(api.count('GET /api/v1/cards/25')).toBe(0)
    await user.keyboard('{Escape}')
    expect(await within(dialog).findByDisplayValue('First card')).toBeInTheDocument()
  })

  it('旧同板任务键显示为可预览的编号，普通文本、代码和链接保持原样', async () => {
    openCardReferences([
      '旧写法TST-14，现有引用 #25。',
      'OTHER-14 TST-98 ref/TST-14 TST-14.md',
      '`TST-14` [TST-14](https://example.com/TST-14) https://example.com/TST-14',
      '```text',
      'TST-14',
      '```',
    ].join('\n'))
    const user = userEvent.setup()
    const dialog = await screen.findByRole('dialog')
    const comment = await within(dialog).findByText(/OTHER-14 TST-98 ref\/TST-14 TST-14\.md/)
    const reference = within(comment).getByRole('button', { name: '#14' })
    expect(within(comment).getByRole('button', { name: '#25' })).toBeInTheDocument()
    expect(within(comment).getByRole('link', { name: 'TST-14' })).toHaveAttribute('href', 'https://example.com/TST-14')
    expect(within(comment).getByRole('link', { name: 'https://example.com/TST-14' })).toBeInTheDocument()
    expect(dialog.querySelector('pre code')).toHaveTextContent('TST-14')
    expect([...dialog.querySelectorAll('p code')].some((node) => node.textContent === 'TST-14')).toBe(true)
    await user.hover(reference)
    expect(screen.getByRole('tooltip')).toHaveTextContent('Second card')
    await user.click(reference)
    expect(await within(dialog).findByDisplayValue('Second card')).toBeInTheDocument()
  })

  it('描述和评论在鼠标悬浮时只对存在的同板任务显示磁贴预览', async () => {
    openCardReferences()
    const user = userEvent.setup()
    const dialog = await screen.findByRole('dialog')
    const description = await within(dialog).findByRole('button', { name: '#14' })
    await user.hover(description)
    // 悬浮显示的是和看板上一样的任务磁贴：编号、标题、标签、截止时间
    const preview = screen.getByRole('tooltip')
    expect(preview).toHaveTextContent('14')
    expect(preview).toHaveTextContent('Second card')
    expect(preview).toHaveTextContent('Urgent')
    expect(preview).toHaveTextContent(formatDate('2026-12-01T00:00:00Z', 'en'))
    await user.hover(within(dialog).getByDisplayValue('First card'))
    expect(screen.queryByRole('tooltip')).toBeNull()
    await user.hover(within(dialog).getByRole('button', { name: '#99' }))
    expect(screen.queryByRole('tooltip')).toBeNull()
    const comment = within(dialog).getByRole('button', { name: '#25' })
    await user.hover(comment)
    expect(screen.getByRole('tooltip')).toHaveTextContent('Third card')
    await user.click(comment)
    expect(await within(dialog).findByDisplayValue('Third card')).toBeInTheDocument()
    expect(screen.queryByRole('tooltip')).toBeNull()
  })

  it('引用用同板卡号找 ID，关闭按钮和 Escape 每次返回一层', async () => {
    const api = openCardReferences()
    const user = userEvent.setup()
    const dialog = await screen.findByRole('dialog')
    await user.click(await within(dialog).findByRole('button', { name: '#14' }))
    expect(await within(dialog).findByDisplayValue('Second card')).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: '#25' }))
    expect(await within(dialog).findByDisplayValue('Third card')).toBeInTheDocument()
    expect(screen.getAllByRole('dialog')).toHaveLength(1)
    await user.keyboard('{Escape}')
    expect(await within(dialog).findByDisplayValue('Second card')).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: en.common.close }))
    expect(await within(dialog).findByDisplayValue('First card')).toBeInTheDocument()
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(api.count('GET /api/v1/cards/14')).toBe(0)
    api.expectNoUnmatched()
  })

  it('点击遮罩关掉整叠详情，找不到引用时留在原卡', async () => {
    openCardReferences()
    const user = userEvent.setup()
    const dialog = await screen.findByRole('dialog')
    await user.click(await within(dialog).findByRole('button', { name: '#99' }))
    expect(await screen.findByText(en.card.notFound)).toBeInTheDocument()
    expect(within(dialog).getByDisplayValue('First card')).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: '#14' }))
    expect(await within(dialog).findByDisplayValue('Second card')).toBeInTheDocument()
    const overlay = document.querySelector('[data-slot="dialog-overlay"]')
    expect(overlay).not.toBeNull()
    await user.click(overlay as HTMLElement)
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })

  it('?card= 打开后修改标题带 version', async () => {
    const api = openBoardWithCard(baseDetail)
    const user = userEvent.setup()
    const dialog = await screen.findByRole('dialog')
    const title = await within(dialog).findByLabelText(en.card.field.title)
    await user.clear(title)
    await user.type(title, '新标题')
    await user.tab()
    await waitFor(() => expect(api.count('PATCH /api/v1/cards/:id')).toBe(1))
    expect(api.calls.find((c) => c.method === 'PATCH')?.body).toMatchObject({ title: '新标题', version: 2 })
    api.expectNoUnmatched()
  })

  it('版本冲突时不丢输入框内容（仍可编辑）', async () => {
    installFakeEventSource()
    const api = installFakeApi({
      'GET /api/v1/boards/:id/snapshot': makeSnapshot({ cards: [makeCard({ id: 1 })] }),
      'GET /api/v1/cards/:id': baseDetail,
      'PATCH /api/v1/cards/:id': () => ({
        status: 409,
        body: { reason: 'ErrCardVersionConflict', msg: '冲突' },
      }),
    })
    const user = userEvent.setup()
    renderApp(
      <Routes>
        <Route path="/b/:boardId" element={<BoardPage />} />
      </Routes>,
      { route: '/b/1?card=1' },
    )
    const dialog = await screen.findByRole('dialog')
    const title = await within(dialog).findByLabelText(en.card.field.title)
    await user.clear(title)
    await user.type(title, '保留')
    await user.tab()
    await waitFor(() => expect(api.count('PATCH /api/v1/cards/:id')).toBe(1))
    expect(title).toHaveValue('保留')
  })

  it('标签按编号提交；输入已有标签的文字不重复新建', async () => {
    const library = [{ id: 1, text: 'bug', color: 'red-2' }]
    const api = openBoardWithCard({ ...baseDetail, tag_ids: [], tags: [] }, [], library)
    const user = userEvent.setup()
    const dialog = await screen.findByRole('dialog')
    await within(dialog).findByLabelText(en.card.field.title)
    await user.click(within(dialog).getByRole('button', { name: en.card.field.tags }))
    const input = await screen.findByLabelText(en.card.field.tags)
    await user.type(input, 'alpha{Enter}')
    await waitFor(() => expect(api.count('PATCH /api/v1/cards/:id')).toBe(1))
    const cardPatch = api.calls.find((c) => c.method === 'PATCH' && c.path.includes('/cards/'))
    expect((cardPatch?.body as { tag_ids?: number[] } | undefined)?.tag_ids).toEqual([101])
    await user.type(input, 'BUG{Enter}')
    await waitFor(() => expect(api.count('PATCH /api/v1/cards/:id')).toBe(2))
    expect(api.count('PATCH /api/v1/boards/:id')).toBe(1)
  })

  it('标签达到上限时内联提示', async () => {
    const library = Array.from({ length: CARD_TAGS_MAX }, (_, i) => ({ id: i + 1, text: `t${i}`, color: '' }))
    const full = library.map((tag) => tag.id)
    openBoardWithCard({ ...baseDetail, tag_ids: full, tags: library.map((tag) => tag.text) }, [], library)
    const user = userEvent.setup()
    const dialog = await screen.findByRole('dialog')
    await within(dialog).findByLabelText(en.card.field.title)
    await user.click(within(dialog).getByRole('button', { name: en.card.field.tags }))
    const input = await screen.findByRole('textbox', { name: en.card.field.tags })
    await user.type(input, 'overflow{Enter}')
    expect(await screen.findByText(en.card.tagLimit.replace('{{max}}', String(CARD_TAGS_MAX)))).toBeInTheDocument()
  })

  it('设了标签后单独占一行：小标题加大号标签，属性按钮一排里不再有标签按钮；没有标签时才是一排里的按钮', async () => {
    const library = [{ id: 1, text: 'bug', color: 'red-2' }]
    openBoardWithCard({ ...baseDetail, tag_ids: [1], tags: ['bug'] }, [], library)
    const dialog = await screen.findByRole('dialog')
    await within(dialog).findByLabelText(en.card.field.title)
    const section = within(dialog).getByRole('region', { name: en.card.field.tags })
    expect(within(section).getByText('bug')).toHaveStyle({ backgroundColor: '#c9372c' })
    expect(within(section).getByRole('button', { name: en.card.field.tags })).toBeInTheDocument()
    expect(within(dialog).getAllByRole('button', { name: en.card.field.tags })).toHaveLength(1)
  })

  it('优先级按钮：显示当前值，点开选另一档就提交，选同一档不提交', async () => {
    const api = openBoardWithCard({ ...baseDetail, priority: 2 })
    const user = userEvent.setup()
    const dialog = await screen.findByRole('dialog')
    await within(dialog).findByLabelText(en.card.field.title)
    await user.click(within(dialog).getByRole('button', { name: `${en.card.field.priority}: ${en.board.priority.medium}` }))
    await user.click(await screen.findByRole('radio', { name: en.board.priority.medium }))
    expect(api.count('PATCH /api/v1/cards/:id')).toBe(0)
    await user.click(within(dialog).getByRole('button', { name: `${en.card.field.priority}: ${en.board.priority.medium}` }))
    await user.click(await screen.findByRole('radio', { name: en.board.priority.high }))
    await waitFor(() => expect(api.count('PATCH /api/v1/cards/:id')).toBe(1))
    expect((patchBody(api.calls) as { priority: number }).priority).toBe(3)
  })

  it('日期按钮：勾选并填写后点保存才提交；取消或没改不提交；取消勾选并保存提交空串', async () => {
    const api = openBoardWithCard({ ...baseDetail, due_at: null, started_at: null })
    const user = userEvent.setup()
    const dialog = await screen.findByRole('dialog')
    await within(dialog).findByLabelText(en.card.field.title)
    const open = () => user.click(within(dialog).getByRole('button', { name: en.card.field.dates }))
    await open()
    await user.click(await screen.findByRole('button', { name: en.common.cancel }))
    expect(api.count('PATCH /api/v1/cards/:id')).toBe(0)

    await open()
    await user.click(await screen.findByRole('checkbox', { name: en.card.field.due }))
    const date = screen.getByLabelText(en.card.field.due, { selector: 'input[type="date"]' })
    fireEvent.change(date, { target: { value: '2026-12-01' } })
    fireEvent.change(screen.getByLabelText(en.card.dueTime), { target: { value: '09:30' } })
    expect(api.count('PATCH /api/v1/cards/:id')).toBe(0)
    await user.click(screen.getByRole('button', { name: en.common.save }))
    await waitFor(() => expect(api.count('PATCH /api/v1/cards/:id')).toBe(1))
    expect(patchBody(api.calls)).toMatchObject({ due_at: new Date('2026-12-01T09:30').toISOString() })
    expect(patchBody(api.calls)).not.toHaveProperty('started_at')
  })

  it('日期弹窗：开始日期可以单独设置，截止早于开始时不能保存', async () => {
    const api = openBoardWithCard({ ...baseDetail, due_at: null, started_at: null })
    const user = userEvent.setup()
    const dialog = await screen.findByRole('dialog')
    await within(dialog).findByLabelText(en.card.field.title)
    await user.click(within(dialog).getByRole('button', { name: en.card.field.dates }))
    await user.click(await screen.findByRole('checkbox', { name: en.card.field.startDate }))
    await user.click(screen.getByRole('checkbox', { name: en.card.field.due }))
    fireEvent.change(screen.getByLabelText(en.card.field.startDate, { selector: 'input[type="date"]' }), { target: { value: '2026-12-10' } })
    fireEvent.change(screen.getByLabelText(en.card.field.due, { selector: 'input[type="date"]' }), { target: { value: '2026-12-01' } })
    const popover = document.querySelector('[data-slot="popover-content"]') as HTMLElement
    expect(await within(popover).findByRole('alert')).toHaveTextContent(en.card.datesOrder)
    expect(screen.getByRole('button', { name: en.common.save })).toBeDisabled()
    await user.click(screen.getByRole('checkbox', { name: en.card.field.due }))
    expect(within(popover).queryByRole('alert')).toBeNull()
    await user.click(screen.getByRole('button', { name: en.common.save }))
    await waitFor(() => expect(api.count('PATCH /api/v1/cards/:id')).toBe(1))
    expect(patchBody(api.calls)).toMatchObject({ started_at: new Date('2026-12-10T00:00').toISOString() })
    expect(patchBody(api.calls)).not.toHaveProperty('due_at')
  })

  it('设了日期以后，「日期」按钮挪到标签区下面显示具体日期，用等宽字体；弹窗里取消勾选再保存会清除', async () => {
    const dueAt = new Date('2026-12-01T09:30').toISOString()
    const api = openBoardWithCard({ ...baseDetail, due_at: dueAt, started_at: null })
    const user = userEvent.setup()
    const dialog = await screen.findByRole('dialog')
    await within(dialog).findByLabelText(en.card.field.title)
    expect(within(dialog).queryByRole('button', { name: en.card.field.dates })).toBeNull()
    const button = await within(dialog).findByRole('button', { name: formatDateTime(dueAt, 'en') })
    expect(button.querySelector('span')).toHaveClass('font-mono')
    await user.click(button)
    expect(await screen.findByLabelText(en.card.field.due, { selector: 'input[type="date"]' })).toHaveValue('2026-12-01')
    expect(screen.getByLabelText(en.card.dueTime)).toHaveValue('09:30')
    await user.click(screen.getByRole('checkbox', { name: en.card.field.due }))
    await user.click(screen.getByRole('button', { name: en.common.save }))
    await waitFor(() => expect(api.count('PATCH /api/v1/cards/:id')).toBe(1))
    expect(patchBody(api.calls)).toMatchObject({ due_at: '' })
  })

  it('描述编辑时按 Esc 只放弃修改，不关掉整个弹窗', async () => {
    openBoardWithCard({ ...baseDetail, description: '旧描述' })
    const user = userEvent.setup()
    const dialog = await screen.findByRole('dialog')
    await user.click(await within(dialog).findByText('旧描述'))
    const editor = await within(dialog).findByRole('textbox', { name: en.card.field.description })
    fireEvent.keyDown(editor, { key: 'Escape', code: 'Escape' })
    await waitFor(() => expect(within(dialog).queryByRole('textbox', { name: en.card.field.description })).toBeNull())
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })

  it('仅人工按钮是开关：点一下提交取反的值', async () => {
    const api = openBoardWithCard({ ...baseDetail, human_only: false })
    const user = userEvent.setup()
    const dialog = await screen.findByRole('dialog')
    await within(dialog).findByLabelText(en.card.field.title)
    const chip = within(dialog).getByRole('button', { name: en.card.field.humanOnly })
    expect(chip).toHaveAttribute('aria-pressed', 'false')
    await user.click(chip)
    await waitFor(() => expect(api.count('PATCH /api/v1/cards/:id')).toBe(1))
    expect((patchBody(api.calls) as { human_only: boolean }).human_only).toBe(true)
  })

  it('显示开始时间、无完成时间，领取可释放', async () => {
    const api = openBoardWithCard(baseDetail)
    const dialog = await screen.findByRole('dialog')
    await within(dialog).findByLabelText(en.card.field.title)
    expect(within(dialog).getByText(en.card.field.started)).toBeInTheDocument()
    expect(within(dialog).queryByText(en.card.field.finished)).toBeNull()
    await userEvent.click(within(dialog).getByRole('button', { name: en.card.claim.release }))
    await waitFor(() => expect(api.count('POST /api/v1/cards/:id/release')).toBe(1))
  })

  it('底部隐藏更新时间，并按本地日期显示今天、昨天和更早的时间', async () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date(2026, 0, 1, 13, 38))
    await i18n.changeLanguage('zh-CN')
    const created = new Date(2025, 11, 30, 23, 59).toISOString()
    const started = new Date(2025, 11, 31, 23, 59).toISOString()
    const finished = new Date(2026, 0, 1, 13, 37).toISOString()
    openBoardWithCard({
      ...baseDetail,
      updated_at: new Date(2026, 0, 1, 13, 38).toISOString(),
      created_at: created,
      started_at: started,
      finished_at: finished,
    })
    const dialog = within(await screen.findByRole('dialog'))
    await dialog.findByLabelText(zhCN.card.field.title)
    const today = await dialog.findByText('今天 13:37', { selector: 'time' })
    expect(today).toHaveAttribute('datetime', finished)
    expect(today).toHaveAttribute('title', formatDateTime(finished, 'zh-CN'))
    expect(dialog.getByText('昨天 23:59', { selector: 'time' })).toHaveAttribute('datetime', started)
    expect(dialog.getByText(formatDateTime(created, 'zh-CN'), { selector: 'time' })).toBeInTheDocument()
    expect(dialog.queryByText(zhCN.card.field.updated)).not.toBeInTheDocument()
  })

  it('领取横幅只给租约时间使用等宽字体，保留日语词序', async () => {
    await i18n.changeLanguage('ja')
    openBoardWithCard(baseDetail)
    const dialog = await screen.findByRole('dialog')
    await within(dialog).findByRole('button', { name: ja.card.claim.release })
    const formatted = formatDateTime(baseDetail.claim_expires_at, 'ja')
    const expiry = within(dialog).getByText(formatted, { selector: '.font-mono' })
    expect(expiry.parentElement).toHaveTextContent(`codex が作業中（${formatted} まで）`)
    expect(expiry.parentElement).not.toHaveClass('font-mono')
  })

  it('桌面调度的评论和移动事件显示工程师名字，不出现身份名', async () => {
    const at = '2026-11-18T10:00:00Z'
    const actor = 'desktop-5xfr3jfu5z2sitwvgckxdkj3n6-3'
    const comment = { board_id: 1, card_id: 1, created_at: at, updated_at: at }
    openBoardWithCard({
      ...baseDetail,
      comments: [{ ...comment, id: 1, author_kind: 'agent', author: actor, body: '桌面写的评论' }],
    } as typeof baseDetail, [{
      id: 9, board_id: 1, card_id: 1, type: 'card.moved', actor_kind: 'agent', actor,
      payload: { from: '待办', to: '进行中' }, created_at: '2026-11-18T11:00:00Z',
    }], [], [{ id: 3, name: 'Atlas' } as Engineer])
    const dialog = within(await screen.findByRole('dialog'))
    await dialog.findByText('桌面写的评论')
    const moved = await dialog.findByText(/moved the task from/)
    expect(moved.parentElement).toHaveTextContent('Atlas moved the task')
    expect(dialog.getAllByText('Atlas').length).toBeGreaterThanOrEqual(2)
    expect(dialog.queryByText(actor)).toBeNull()
    expect(dialog.queryByText(/desktop-/)).toBeNull()
  })

  it('评论里人写的显示为「我」，不出现原始的 human；agent 显示自己的名字加 Agent', async () => {
    const at = '2026-11-18T10:00:00Z'
    const comment = { board_id: 1, card_id: 1, created_at: at, updated_at: at }
    openBoardWithCard({
      ...baseDetail,
      comments: [
        { ...comment, id: 1, author_kind: 'human', author: 'human', body: '人写的评论' },
        { ...comment, id: 2, author_kind: 'agent', author: 'codex', body: 'agent 写的评论' },
        { ...comment, id: 3, author_kind: 'system', author: 'system', body: '系统写的评论' },
      ],
    } as typeof baseDetail)
    const dialog = await screen.findByRole('dialog')
    await within(dialog).findByText('人写的评论')
    expect(within(dialog).getByText(en.card.comment.human)).toBeInTheDocument()
    expect(within(dialog).queryByText('human')).toBeNull()
    expect(within(dialog).getByText('codex')).toBeInTheDocument()
    expect(within(dialog).getByText(en.card.comment.agent)).toBeInTheDocument()
    expect(within(dialog).getByText(en.card.comment.system)).toBeInTheDocument()
    expect(within(dialog).queryByText('system')).toBeNull()
  })

  it('评论未满一分钟时显示刚刚，满一分钟后恢复为相对时间', async () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    const now = new Date(2026, 0, 1, 13, 38).getTime()
    vi.setSystemTime(now)
    await i18n.changeLanguage('zh-CN')
    const recent = new Date(now - 59_000).toISOString()
    const older = new Date(now - 60_000).toISOString()
    openBoardWithCard({
      ...baseDetail,
      comments: [
        { id: 1, board_id: 1, card_id: 1, author_kind: 'human', author: 'human', body: '59 秒', created_at: recent, updated_at: recent },
        { id: 2, board_id: 1, card_id: 1, author_kind: 'human', author: 'human', body: '60 秒', created_at: older, updated_at: older },
      ],
    }, [
      { id: 9, board_id: 1, card_id: 1, type: 'card.moved', actor_kind: 'agent', actor: 'codex', payload: { from: 'Alpha', to: 'Beta' }, created_at: recent },
    ])
    const dialog = within(await screen.findByRole('dialog'))
    const timeFor = (body: HTMLElement) => body.closest('li')?.querySelector('time')
    const recentTime = timeFor(await dialog.findByText('59 秒', { selector: 'p' }))
    expect(recentTime).toHaveTextContent('刚刚')
    expect(recentTime).toHaveAttribute('datetime', recent)
    expect(recentTime).toHaveAttribute('title', formatDateTime(recent, 'zh-CN'))
    expect(timeFor(dialog.getByText('60 秒', { selector: 'p' }))).toHaveTextContent('1分钟前')
    expect(timeFor(await dialog.findByText(/Alpha/))).toHaveTextContent('59秒钟前')
    vi.setSystemTime(now + 1000)
    await waitFor(() => expect(recentTime).toHaveTextContent('1分钟前'))
  })

  it('弹窗内同时有属性区、描述与评论', async () => {
    openBoardWithCard(baseDetail)
    const dialog = await screen.findByRole('dialog')
    await within(dialog).findByLabelText(en.card.field.title)
    expect(within(dialog).getByText(en.card.field.priority)).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: en.card.descEdit })).toBeInTheDocument()
    expect(within(dialog).getByText(en.card.timeline.title)).toBeInTheDocument()
  })

  it('「评论和活动」把评论和这个任务的事件按时间排在一起，可收起详情', async () => {
    const at = '2026-11-18T10:00:00Z'
    const later = '2026-11-18T11:00:00Z'
    const api = openBoardWithCard(
      {
        ...baseDetail,
        comments: [{ id: 1, board_id: 1, card_id: 1, author_kind: 'human', author: 'human', body: '一条评论', created_at: at, updated_at: at }],
      } as typeof baseDetail,
      [{ id: 9, board_id: 1, card_id: 1, type: 'card.moved', actor_kind: 'agent', actor: 'codex', payload: { from: 'Alpha', to: 'Beta' }, created_at: later }],
    )
    const dialog = await screen.findByRole('dialog')
    expect(await within(dialog).findByText('一条评论')).toBeInTheDocument()
    expect(await within(dialog).findByText(/Alpha/)).toBeInTheDocument()
    const call = api.calls.find((c) => c.path.endsWith('/activity'))
    expect(call?.query.get('card_id')).toBe('1')
    expect(call?.query.get('types')).toContain('card.moved')

    await userEvent.click(within(dialog).getByRole('button', { name: en.card.timeline.hideDetails }))
    expect(within(dialog).queryByText(/Alpha/)).toBeNull()
    expect(within(dialog).getByText('一条评论')).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: en.card.timeline.showDetails })).toBeInTheDocument()
  })

  it('卡片活动显示桌面领取事件中的工程师名', async () => {
    openBoardWithCard(baseDetail, [{
      id: 9, board_id: 1, card_id: 1, type: 'card.claimed', actor_kind: 'agent',
      actor: 'desktop-5xfr3jfu5z2sitwvgckxdkj3n6', payload: { engineer_id: 1, engineer_name: 'Atlas' },
      created_at: '2026-11-18T11:00:00Z',
    }])
    const dialog = within(await screen.findByRole('dialog'))
    const action = await dialog.findByText(en.activity.event.cardClaimed)
    expect(action.parentElement).toHaveTextContent('Atlas claimed the task')
  })

  it('「评论和活动」里改了描述的那条带「查看变更」，点开在任务弹窗上叠一层左右对比', async () => {
    const api = openBoardWithCard(baseDetail, [
      {
        id: 9,
        board_id: 1,
        card_id: 1,
        type: 'card.updated',
        actor_kind: 'human',
        actor: 'human',
        payload: { key: 'SLATE-1', title: '详情卡', fields: ['description'], description_change_id: 4 },
        created_at: '2026-11-18T11:00:00Z',
      },
    ])
    api.set('GET /api/v1/cards/:id/changes/:id', {
      id: 4,
      board_id: 1,
      card_id: 1,
      field: 'description',
      old_value: '旧内容',
      new_value: '新内容',
      actor_kind: 'human',
      actor: 'human',
      created_at: '2026-11-18T11:00:00Z',
      previous_at: '2026-11-18T08:00:00Z',
    })
    const dialog = await screen.findByRole('dialog')
    await userEvent.click(await within(dialog).findByRole('button', { name: en.activity.change.link }))
    const diff = await screen.findByRole('dialog', { name: en.activity.change.title })
    expect(await within(diff).findByText(formatDateTime('2026-11-18T08:00:00Z', 'en'))).toBeInTheDocument()
    expect(api.calls.filter((c) => c.path === '/api/v1/cards/1/changes/4')).toHaveLength(1)

    // Esc 只关掉最上面的对比弹窗，任务弹窗还在
    await userEvent.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog', { name: en.activity.change.title })).toBeNull())
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  // 打开一个详情请求会失败的任务，用来检查弹窗对不同失败原因的提示
  function openBoardWithBrokenDetail(detail: () => { status?: number; body?: unknown }) {
    installFakeEventSource()
    installFakeApi({
      'GET /api/v1/boards/:id/snapshot': makeSnapshot({ cards: [makeCard({ id: 1 })] }),
      'GET /api/v1/cards/:id': detail,
    })
    renderApp(
      <Routes>
        <Route path="/b/:boardId" element={<BoardPage />} />
      </Routes>,
      { route: '/b/1?card=1' },
    )
  }

  it('服务端说任务不存在（404）时提示任务已被删除', async () => {
    openBoardWithBrokenDetail(() => ({ status: 404, body: { reason: 'ErrCardNotFound', msg: '没有' } }))
    const dialog = await screen.findByRole('dialog')
    expect(await within(dialog).findByText(en.card.notFound)).toBeInTheDocument()
  })

  it('断网时如实提示连不上服务器，不说任务已被删除', async () => {
    openBoardWithBrokenDetail(() => {
      throw new TypeError('Failed to fetch')
    })
    const dialog = await screen.findByRole('dialog')
    expect(await within(dialog).findByText(en.errors.ErrNetwork)).toBeInTheDocument()
    expect(within(dialog).queryByText(en.card.notFound)).toBeNull()
  })

  it('服务器出错（500）时也不说任务已被删除', async () => {
    openBoardWithBrokenDetail(() => ({ status: 500, body: { reason: 'ErrInternal', msg: '服务器出错了' } }))
    const dialog = await screen.findByRole('dialog')
    expect(await within(dialog).findByText(en.errors.ErrInternal)).toBeInTheDocument()
    expect(within(dialog).queryByText(en.card.notFound)).toBeNull()
  })
})
