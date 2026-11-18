import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { en } from '@/i18n/locales/en'
import { RuleEditorDialog } from './RuleEditorDialog'
import { installFakeApi } from '@/test/fakeApi'
import { makeColumn, makeSnapshot } from '@/test/factories'
import { renderApp } from '@/test/renderApp'
import type { Rule } from '@/lib/api/types'

const columns = [
  makeColumn({ id: 2, name: '进行中', agent_pickup: true }),
  makeColumn({ id: 3, name: '完成', is_done: true }),
]

function renderDialog(rule?: Rule) {
  installFakeApi({
    'GET /api/v1/boards/:id/snapshot': makeSnapshot({ columns }),
    'POST /api/v1/boards/:id/rules': { id: 1 },
    'PATCH /api/v1/rules/:ruleId': {},
  })
  return renderApp(<RuleEditorDialog boardId={1} columns={columns} rule={rule} open onOpenChange={() => {}} />)
}

describe('RuleEditorDialog', () => {
  it('新建时更多选项默认折叠', () => {
    renderDialog()
    expect(screen.getByRole('button', { name: en.settings.rules.moreOptions })).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByLabelText(en.settings.rules.name)).not.toBeInTheDocument()
  })

  it('不填名称也能创建且 name 为自动句', async () => {
    const api = installFakeApi({
      'GET /api/v1/boards/:id/snapshot': makeSnapshot({ columns }),
      'POST /api/v1/boards/:id/rules': { id: 1 },
    })
    renderApp(<RuleEditorDialog boardId={1} columns={columns} open onOpenChange={() => {}} />)
    await userEvent.click(screen.getByRole('button', { name: en.common.create }))
    await waitFor(() => expect(api.count('POST /api/v1/boards/:id/rules')).toBe(1))
    const body = api.calls.find((c) => c.method === 'POST' && c.path.endsWith('/rules'))?.body as { name: string }
    expect(body.name).toContain('record start time')
  })

  it('自定义名称的规则默认展开更多选项', async () => {
    const rule: Rule = {
      id: 1,
      board_id: 1,
      name: '我的特殊规则',
      enabled: true,
      trigger: 'card_entered',
      column_id: 2,
      condition: '',
      actions: [{ type: 'set_started_at' }],
      sort: 1,
      created_at: '',
      updated_at: '',
    }
    renderDialog(rule)
    expect(screen.getByRole('button', { name: en.settings.rules.moreOptions })).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByLabelText(en.settings.rules.name)).toBeInTheDocument()
  })

  it('再加一个动作与删除', async () => {
    renderDialog()
    await userEvent.click(screen.getByRole('button', { name: en.settings.rules.editor.addAnotherAction }))
    const removes = screen.getAllByRole('button', { name: en.settings.rules.removeAction })
    expect(removes.length).toBe(2)
    await userEvent.click(removes[1])
    expect(screen.queryByRole('button', { name: en.settings.rules.removeAction })).not.toBeInTheDocument()
  })
})