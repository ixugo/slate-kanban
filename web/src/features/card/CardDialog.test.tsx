import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import { Route, Routes } from 'react-router-dom'
import i18n from '@/i18n'
import { en } from '@/i18n/locales/en'
import { ja } from '@/i18n/locales/ja'
import { installFakeApi, type FakeCall } from '@/test/fakeApi'
import { makeCard, makeSnapshot } from '@/test/factories'
import { renderApp } from '@/test/renderApp'
import { installFakeEventSource } from '@/test/fakeEventSource'
import BoardPage from '@/features/board/BoardPage'
import { CARD_TAGS_MAX } from '@/lib/limits'
import { formatDateTime } from '@/lib/format'
import type { Tag, TagInput } from '@/lib/api/types'

const baseDetail = {
  ...makeCard({ id: 1, title: '详情卡', version: 2, tag_ids: [1], tags: ['bug'] }),
  column_instruction: 'do work',
  dependencies: [],
  comments: [],
  started_at: '2026-11-01T10:00:00Z',
  finished_at: null,
  claim_active: true,
  claimed_by: 'codex',
  claim_expires_at: '2026-11-18T12:00:00Z',
}

// 看板页里的卡片弹窗是懒加载的：先把它的模块加载好，第一个测试就不用在 findBy 的限时里等模块首次编译。
beforeAll(async () => {
  await import('@/features/card/CardDialog')
}, 30000)

afterEach(() => vi.unstubAllGlobals())

/** 取第一次 PATCH 请求的请求体 */
function patchBody(calls: FakeCall[]): unknown {
  return calls.find((c) => c.method === 'PATCH')?.body
}

function openBoardWithCard(detail: typeof baseDetail, events: object[] = [], library: Tag[] = []) {
  installFakeEventSource()
  let current = { ...detail }
  const api = installFakeApi({
    'GET /api/v1/boards/:id/snapshot': makeSnapshot({ tags: library, cards: [makeCard({ id: 1 })] }),
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

describe('CardDialog', () => {
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

  it('截止按钮：设了时间才在关闭弹层时提交，清除按钮提交空串，没改不提交', async () => {
    const api = openBoardWithCard({ ...baseDetail, due_at: null })
    const user = userEvent.setup()
    const dialog = await screen.findByRole('dialog')
    await within(dialog).findByLabelText(en.card.field.title)
    await user.click(within(dialog).getByRole('button', { name: en.card.field.due }))
    await user.keyboard('{Escape}')
    expect(api.count('PATCH /api/v1/cards/:id')).toBe(0)
    await user.click(within(dialog).getByRole('button', { name: en.card.field.due }))
    const input = await screen.findByLabelText(en.card.field.due)
    fireEvent.change(input, { target: { value: '2026-12-01T09:30' } })
    await user.keyboard('{Escape}')
    await waitFor(() => expect(api.count('PATCH /api/v1/cards/:id')).toBe(1))
    expect((patchBody(api.calls) as { due_at: string }).due_at).toBe(new Date('2026-12-01T09:30').toISOString())
  })

  it('截止编辑器用等宽字体显示已有的日期和时间', async () => {
    const dueAt = new Date('2026-12-01T09:30').toISOString()
    openBoardWithCard({ ...baseDetail, due_at: dueAt })
    const dialog = await screen.findByRole('dialog')
    await userEvent.click(await within(dialog).findByRole('button', { name: formatDateTime(dueAt, 'en') }))
    const input = await screen.findByLabelText(en.card.field.due)
    expect(input).toHaveClass('font-mono')
    expect(input).toHaveValue('2026-12-01T09:30')
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

  it('弹窗内同时有属性区、描述与评论', async () => {
    openBoardWithCard(baseDetail)
    const dialog = await screen.findByRole('dialog')
    await within(dialog).findByLabelText(en.card.field.title)
    expect(within(dialog).getByText(en.card.field.priority)).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: en.card.descEdit })).toBeInTheDocument()
    expect(within(dialog).getByText(en.card.timeline.title)).toBeInTheDocument()
  })

  it('「评论和活动」把评论和这张卡片的事件按时间排在一起，可收起详情', async () => {
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

  it('「评论和活动」里改了描述的那条带「查看变更」，点开在卡片弹窗上叠一层左右对比', async () => {
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
    })
    const dialog = await screen.findByRole('dialog')
    await userEvent.click(await within(dialog).findByRole('button', { name: en.activity.change.link }))
    const diff = await screen.findByRole('dialog', { name: en.activity.change.title })
    expect(await within(diff).findByText(en.activity.change.before)).toBeInTheDocument()
    expect(api.calls.filter((c) => c.path === '/api/v1/cards/1/changes/4')).toHaveLength(1)

    // Esc 只关掉最上面的对比弹窗，卡片弹窗还在
    await userEvent.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog', { name: en.activity.change.title })).toBeNull())
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  // 打开一个详情请求会失败的卡片，用来检查弹窗对不同失败原因的提示
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

  it('服务端说卡片不存在（404）时提示卡片已被删除', async () => {
    openBoardWithBrokenDetail(() => ({ status: 404, body: { reason: 'ErrCardNotFound', msg: '没有' } }))
    const dialog = await screen.findByRole('dialog')
    expect(await within(dialog).findByText(en.card.notFound)).toBeInTheDocument()
  })

  it('断网时如实提示连不上服务器，不说卡片已被删除', async () => {
    openBoardWithBrokenDetail(() => {
      throw new TypeError('Failed to fetch')
    })
    const dialog = await screen.findByRole('dialog')
    expect(await within(dialog).findByText(en.errors.ErrNetwork)).toBeInTheDocument()
    expect(within(dialog).queryByText(en.card.notFound)).toBeNull()
  })

  it('服务器出错（500）时也不说卡片已被删除', async () => {
    openBoardWithBrokenDetail(() => ({ status: 500, body: { reason: 'ErrInternal', msg: '服务器出错了' } }))
    const dialog = await screen.findByRole('dialog')
    expect(await within(dialog).findByText(en.errors.ErrInternal)).toBeInTheDocument()
    expect(within(dialog).queryByText(en.card.notFound)).toBeNull()
  })
})
