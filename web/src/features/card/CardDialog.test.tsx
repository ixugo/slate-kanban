import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import { Route, Routes } from 'react-router-dom'
import { en } from '@/i18n/locales/en'
import { installFakeApi, type FakeCall } from '@/test/fakeApi'
import { makeCard, makeSnapshot } from '@/test/factories'
import { renderApp } from '@/test/renderApp'
import { installFakeEventSource } from '@/test/fakeEventSource'
import BoardPage from '@/features/board/BoardPage'
import { CARD_TAGS_MAX } from '@/lib/limits'

const baseDetail = {
  ...makeCard({ id: 1, title: '详情卡', version: 2, tags: ['bug'] }),
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

function openBoardWithCard(detail: typeof baseDetail) {
  installFakeEventSource()
  let current = { ...detail }
  const api = installFakeApi({
    'GET /api/v1/boards/:id/snapshot': makeSnapshot({ cards: [makeCard({ id: 1 })] }),
    'GET /api/v1/cards/:id': () => ({ body: current }),
    'PATCH /api/v1/cards/:id': (call: FakeCall) => {
      current = {
        ...current,
        ...(call.body as object),
        version: current.version + 1,
        dependencies: current.dependencies ?? [],
        comments: current.comments ?? [],
        tags: (call.body as { tags?: string[] }).tags ?? current.tags ?? [],
        waits_on: (call.body as { waits_on?: number[] }).waits_on ?? current.waits_on ?? [],
      }
      return { body: current }
    },
    'POST /api/v1/cards/:id/release': { body: {} },
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
      'GET /api/v1/cards/:id': { body: baseDetail },
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

  it('标签可添加，重复标签不重复提交', async () => {
    const api = openBoardWithCard({ ...baseDetail, tags: [] })
    const user = userEvent.setup()
    const dialog = await screen.findByRole('dialog')
    await within(dialog).findByLabelText(en.card.field.title)
    const input = await within(dialog).findByLabelText(en.card.field.tags)
    await user.type(input, 'alpha{Enter}')
    await waitFor(() => expect(api.count('PATCH /api/v1/cards/:id')).toBe(1))
    await user.type(input, 'ALPHA{Enter}')
    await waitFor(() => {
      const patches = api.calls.filter((c) => c.method === 'PATCH')
      const last = patches[patches.length - 1]?.body as { tags?: string[] }
      expect(last?.tags).toEqual(['alpha'])
    })
  })

  it('标签达到上限时内联提示', async () => {
    const full = Array.from({ length: CARD_TAGS_MAX }, (_, i) => `t${i}`)
    openBoardWithCard({ ...baseDetail, tags: full })
    const user = userEvent.setup()
    const dialog = await screen.findByRole('dialog')
    await within(dialog).findByLabelText(en.card.field.title)
    const input = await within(dialog).findByLabelText(en.card.field.tags)
    await user.type(input, 'overflow{Enter}')
    expect(await within(dialog).findByRole('alert')).toHaveTextContent(
      en.card.tagLimit.replace('{{max}}', String(CARD_TAGS_MAX)),
    )
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
    expect(within(dialog).getByText(en.card.comment.title)).toBeInTheDocument()
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
