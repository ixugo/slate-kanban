import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { en } from '@/i18n/locales/en'
import { installFakeApi, type FakeCall } from '@/test/fakeApi'
import { makeColumn } from '@/test/factories'
import { renderApp } from '@/test/renderApp'
import { WebhooksSection } from './WebhooksSection'

describe('Webhook subscription defaults', () => {
  it('checks visible events and excludes only the exact default actor and inbox choices', async () => {
    installFakeApi({ 'GET /api/v1/boards/:id/webhooks': [] })
    const columns = [
      makeColumn({ id: 1, name: 'Inbox' }),
      makeColumn({ id: 2, name: 'Inbox follow-up' }),
      makeColumn({ id: 3, name: '收件箱' }),
      makeColumn({ id: 4, name: '进行中' }),
    ]
    renderApp(<WebhooksSection boardId={1} columns={columns} />)
    await userEvent.click(screen.getByRole('button', { name: en.settings.webhooks.add }))
    const dialog = within(await screen.findByRole('dialog'))
    for (const label of Object.values(en.settings.webhooks.events)) {
      expect(dialog.getByRole('checkbox', { name: label })).toBeChecked()
    }
    for (const label of ['Inbox', '收件箱', en.settings.webhooks.actor.human, en.settings.webhooks.actor.system]) {
      expect(dialog.getByRole('checkbox', { name: label })).not.toBeChecked()
    }
    for (const label of ['Inbox follow-up', '进行中', en.settings.webhooks.actor.agent]) {
      expect(dialog.getByRole('checkbox', { name: label })).toBeChecked()
    }
  })

  it('only updates the changed subscription dimension and reopens with its saved selection', async () => {
    let saved = {
      id: 9, board_id: 1, name: 'Legacy', url: 'https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=test',
      type: 'wecom', enabled: true, secret_set: false, secret_mask: '',
      event_types: [], actor_kinds: [], move_column_ids: [], strict_filters: false,
    }
    const api = installFakeApi({
      'GET /api/v1/boards/:id/webhooks': () => ({ body: [saved] }),
      'PATCH /api/v1/webhooks/:id': (call: FakeCall) => {
        saved = { ...saved, ...(call.body as Partial<typeof saved>) }
        return { body: saved }
      },
    })
    const user = userEvent.setup()
    renderApp(<WebhooksSection boardId={1} columns={[makeColumn({ id: 2, name: 'Doing' })]} />)
    const open = async () => {
      await user.click(await screen.findByRole('button', { name: en.common.edit }))
      return within(await screen.findByRole('dialog'))
    }
    const dialog = await open()
    const event = dialog.getByRole('checkbox', { name: en.settings.webhooks.events.card_created })
    await user.click(event)
    await user.click(event)
    for (const label of Object.values(en.settings.webhooks.actor)) {
      await user.click(dialog.getByRole('checkbox', { name: label }))
    }
    await user.click(dialog.getByRole('button', { name: en.common.save }))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    const patch = api.calls.find((call) => call.method === 'PATCH')?.body
    expect(patch).toMatchObject({ actor_kinds: [], strict_filter_modes: { actor_kinds: true } })
    expect(patch).not.toHaveProperty('strict_filters')
    expect(patch).not.toHaveProperty('event_types')
    expect(patch).not.toHaveProperty('move_column_ids')
    const reopened = await open()
    expect(reopened.getByRole('checkbox', { name: en.settings.webhooks.actor.agent })).not.toBeChecked()
    expect(reopened.getByRole('checkbox', { name: en.settings.webhooks.events.card_created })).toBeChecked()
    expect(reopened.getByRole('checkbox', { name: 'Doing' })).toBeChecked()
  })

})
