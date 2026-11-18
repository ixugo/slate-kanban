import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { Route, Routes } from 'react-router-dom'
import { en } from '@/i18n/locales/en'
import { installFakeApi } from '@/test/fakeApi'
import { makeCard, makeSnapshot } from '@/test/factories'
import { renderApp } from '@/test/renderApp'
import { installFakeEventSource } from '@/test/fakeEventSource'
import BoardPage from '@/features/board/BoardPage'

const detail = {
  ...makeCard({ id: 1, title: '详情卡', version: 2 }),
  column_instruction: 'do work',
  dependencies: [],
  comments: [],
}

afterEach(() => vi.unstubAllGlobals())

const renderWithCard = () => {
  installFakeEventSource()
  return renderApp(
    <Routes>
      <Route path="/b/:boardId" element={<BoardPage />} />
    </Routes>,
    { route: '/b/1?card=1' },
  )
}

describe('CardDialog', () => {
  it('?card= 打开后修改标题带 version', async () => {
    const api = installFakeApi({
      'GET /api/v1/boards/:id/snapshot': makeSnapshot({ cards: [makeCard({ id: 1 })] }),
      'GET /api/v1/cards/:id': detail,
      'PATCH /api/v1/cards/:id': makeCard({ id: 1, title: '新标题', version: 3 }),
    })
    const user = userEvent.setup()
    renderWithCard()
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
    const api = installFakeApi({
      'GET /api/v1/boards/:id/snapshot': makeSnapshot({ cards: [makeCard({ id: 1 })] }),
      'GET /api/v1/cards/:id': detail,
      'PATCH /api/v1/cards/:id': () => ({
        status: 409,
        body: { reason: 'ErrCardVersionConflict', msg: '冲突' },
      }),
    })
    const user = userEvent.setup()
    renderWithCard()
    const dialog = await screen.findByRole('dialog')
    const title = await within(dialog).findByLabelText(en.card.field.title)
    await user.clear(title)
    await user.type(title, '保留')
    await user.tab()
    await waitFor(() => expect(api.count('PATCH /api/v1/cards/:id')).toBe(1))
    expect(title).toHaveValue('保留')
  })
})