import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { en } from '@/i18n/locales/en'
import { AutomationSection } from './AutomationSection'
import { installFakeApi } from '@/test/fakeApi'
import { makeColumn, makeSnapshot } from '@/test/factories'
import { renderApp } from '@/test/renderApp'
import type { Rule } from '@/lib/api/types'
import { buildRuleAutoName } from './ruleAutoName'
import type { TFunction } from '@/i18n/types'

function makeT(): TFunction {
  return ((key: string, params?: Record<string, string | number>) => {
    const parts = key.split('.')
    let cur: unknown = en
    for (const p of parts) {
      if (cur && typeof cur === 'object' && p in cur) cur = (cur as Record<string, unknown>)[p]
      else return key
    }
    if (typeof cur !== 'string') return key
    return cur.replace(/\{\{(\w+)\}\}/g, (_, n: string) => String(params?.[n] ?? ''))
  }) as TFunction
}

const columns = [
  makeColumn({ id: 1, name: '待办' }),
  makeColumn({ id: 2, name: '进行中', agent_pickup: true }),
  makeColumn({ id: 3, name: '完成', is_done: true }),
]

function renderSection(rules: Rule[] = [], cols = columns) {
  const snap = makeSnapshot({ rules, columns: cols })
  installFakeApi({
    'GET /api/v1/boards/:id/snapshot': snap,
    'POST /api/v1/boards/:id/rules': { id: 99 },
    'PATCH /api/v1/rules/:ruleId': {},
    'DELETE /api/v1/rules/:ruleId': {},
  })
  return renderApp(<AutomationSection boardId={1} columns={cols} rules={rules} />)
}

describe('AutomationSection 推荐规则', () => {
  it('无已有规则时展示三条推荐', () => {
    renderSection()
    expect(screen.getByRole('heading', { name: en.settings.rules.recommended })).toBeInTheDocument()
    const items = screen.getAllByRole('button', { name: new RegExp(en.settings.rules.recommendedAdd) })
    expect(items.length).toBe(3)
  })

  it('已有等价规则时不展示该推荐', () => {
    const t = makeT()
    const name = buildRuleAutoName('card_entered', 2, [{ type: 'set_started_at', mode: 'if_empty' }], columns, t)
    const rules: Rule[] = [
      {
        id: 1,
        board_id: 1,
        name,
        enabled: true,
        trigger: 'card_entered',
        column_id: 2,
        condition: '',
        actions: [{ type: 'set_started_at', mode: 'if_empty' }],
        sort: 1,
        created_at: '',
        updated_at: '',
      },
    ]
    renderSection(rules)
    const items = screen.getAllByRole('button', { name: new RegExp(en.settings.rules.recommendedAdd) })
    expect(items.length).toBe(2)
  })

  it('点击添加会 POST 创建规则', async () => {
    const api = installFakeApi({
      'GET /api/v1/boards/:id/snapshot': makeSnapshot({ rules: [], columns }),
      'POST /api/v1/boards/:id/rules': { id: 99 },
    })
    renderApp(<AutomationSection boardId={1} columns={columns} rules={[]} />)
    const addButtons = screen.getAllByRole('button', { name: new RegExp(en.settings.rules.recommendedAdd) })
    await userEvent.click(addButtons[0])
    await waitFor(() => expect(api.count('POST /api/v1/boards/:id/rules')).toBe(1))
    const body = api.calls.find((c) => c.method === 'POST' && c.path.endsWith('/rules'))?.body as {
      name: string
      trigger: string
    }
    expect(body.trigger).toBe('card_entered')
    expect(body.name).toContain('record start time')
  })

  it('没有 agent 列时不展示进入进行中推荐', () => {
    const cols = [makeColumn({ id: 1, name: '待办' }), makeColumn({ id: 3, name: '完成', is_done: true })]
    renderSection([], cols)
    const items = screen.getAllByRole('button', { name: new RegExp(en.settings.rules.recommendedAdd) })
    expect(items.length).toBe(2)
  })
})

describe('AutomationSection 我的规则', () => {
  it('开关切换发 PATCH', async () => {
    const t = makeT()
    const name = buildRuleAutoName('card_entered', 3, [{ type: 'set_finished_at' }], columns, t)
    const rules: Rule[] = [
      {
        id: 5,
        board_id: 1,
        name,
        enabled: true,
        trigger: 'card_entered',
        column_id: 3,
        condition: '',
        actions: [{ type: 'set_finished_at' }],
        sort: 1,
        created_at: '',
        updated_at: '',
      },
    ]
    const api = installFakeApi({
      'GET /api/v1/boards/:id/snapshot': makeSnapshot({ rules, columns }),
      'PATCH /api/v1/rules/:ruleId': {},
    })
    renderApp(<AutomationSection boardId={1} columns={columns} rules={rules} />)
    const sw = screen.getByRole('switch')
    await userEvent.click(sw)
    await waitFor(() => expect(api.count('PATCH /api/v1/rules/:ruleId')).toBe(1))
  })

  it('删除需确认', async () => {
    const rules: Rule[] = [
      {
        id: 5,
        board_id: 1,
        name: '自定义名',
        enabled: true,
        trigger: 'card_created',
        column_id: 0,
        condition: '',
        actions: [{ type: 'clear_due' }],
        sort: 1,
        created_at: '',
        updated_at: '',
      },
    ]
    const api = installFakeApi({
      'GET /api/v1/boards/:id/snapshot': makeSnapshot({ rules, columns }),
      'DELETE /api/v1/rules/:ruleId': {},
    })
    renderApp(<AutomationSection boardId={1} columns={columns} rules={rules} />)
    const myRules = screen.getByRole('region', { name: en.settings.rules.myRules })
    await userEvent.click(within(myRules).getByRole('button', { name: en.common.delete }))
    const dialog = await screen.findByRole('alertdialog')
    await userEvent.click(within(dialog).getByRole('button', { name: en.common.delete }))
    await waitFor(() => expect(api.count('DELETE /api/v1/rules/:ruleId')).toBe(1))
  })
})