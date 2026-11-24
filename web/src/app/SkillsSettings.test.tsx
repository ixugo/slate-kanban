import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { en } from '@/i18n/locales/en'
import { installFakeApi } from '@/test/fakeApi'
import { renderApp } from '@/test/renderApp'
import { AppSettingsButton } from './AppSettingsButton'

const builtin = { name: 'slate-kanban', description: 'Dispatch', uri: 'skill://slate-kanban/SKILL.md', digest: 'sha256:123', size: 31, builtin: true }
const custom = { name: 'team-guide', description: 'Use for team reviews', uri: 'skill://team-guide/SKILL.md', digest: 'sha256:456', size: 42, builtin: false }
const markdown = '# Team guide\n\n    preserve indentation\n'

describe('Skill settings', () => {
  it('opens from global settings, reads the builtin, creates, edits and deletes a custom Skill', async () => {
    let rows = [builtin]
    const api = installFakeApi({
      'GET /api/v1/auth/status': { required: false, authenticated: true },
      'GET /api/v1/skills': () => ({ body: rows }),
      'GET /api/v1/skills/slate-kanban': { ...builtin, body: '# Built-in instructions' },
      'GET /api/v1/skills/team-guide': { ...custom, body: markdown },
      'POST /api/v1/skills': () => { rows = [builtin, custom]; return { body: { ...custom, body: markdown } } },
      'PUT /api/v1/skills/team-guide': () => { rows = [builtin, { ...custom, description: 'Updated description' }]; return { body: { ...custom, body: markdown } } },
      'DELETE /api/v1/skills/team-guide': () => { rows = [builtin]; return { body: {} } },
    })
    renderApp(<AppSettingsButton />)
    await userEvent.click(screen.getByRole('button', { name: en.nav.settings }))
    await userEvent.click(await screen.findByRole('tab', { name: 'Skills' }))
    expect(await screen.findByText('slate-kanban')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'View skill "slate-kanban"' }))
    const readOnly = await screen.findByRole('dialog', { name: 'slate-kanban' })
    expect(await within(readOnly).findByText('Built-in instructions')).toBeInTheDocument()
    expect(within(readOnly).queryByRole('button', { name: en.common.save })).not.toBeInTheDocument()
    await userEvent.click(within(readOnly).getByRole('button', { name: en.common.close }))
    await userEvent.click(screen.getByRole('button', { name: 'New skill' }))
    const create = await screen.findByRole('dialog', { name: 'New skill' })
    await userEvent.type(within(create).getByLabelText('Name'), 'team-guide')
    await userEvent.type(within(create).getByLabelText('Description'), custom.description)
    const editor = await within(create).findByRole('textbox', { name: 'Markdown body' })
    await act(async () => editor.dispatchEvent(new InputEvent('beforeinput', { inputType: 'insertText', data: markdown, bubbles: true, cancelable: true })))
    await userEvent.click(within(create).getByRole('button', { name: en.common.save }))
    await waitFor(() => expect(api.count('POST /api/v1/skills')).toBe(1))
    expect(api.calls.find((c) => c.method === 'POST')?.body).toEqual({ name: custom.name, description: custom.description, body: markdown })
    expect(await screen.findByText('team-guide')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'View skill "team-guide"' }))
    const edit = await screen.findByRole('dialog', { name: 'team-guide' })
    const description = await within(edit).findByLabelText('Description')
    await userEvent.clear(description)
    await userEvent.type(description, 'Updated description')
    expect(await within(edit).findByRole('textbox', { name: 'Markdown body' })).toHaveTextContent('preserve indentation')
    await userEvent.click(within(edit).getByRole('button', { name: en.common.save }))
    await waitFor(() => expect(api.count('PUT /api/v1/skills/team-guide')).toBe(1))
    expect(api.calls.find((c) => c.method === 'PUT')?.body).toEqual({ name: custom.name, description: 'Updated description', body: markdown })
    expect(await screen.findByText('Updated description')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Delete skill "team-guide"' }))
    const confirm = await screen.findByRole('alertdialog')
    await userEvent.click(within(confirm).getByRole('button', { name: en.common.delete }))
    await waitFor(() => expect(screen.queryByText('team-guide')).not.toBeInTheDocument())
    expect(api.count('DELETE /api/v1/skills/team-guide')).toBe(1)
    api.expectNoUnmatched()
  })
})
