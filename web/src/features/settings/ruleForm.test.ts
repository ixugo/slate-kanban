import { describe, expect, it } from 'vitest'
import { makeColumn } from '@/test/factories'
import {
  emptyRuleForm,
  formToCreateInput,
  ruleToForm,
  validateRuleForm,
  type ActionFormRow,
} from './ruleForm'

const columns = [
  makeColumn({ id: 1, name: '待办' }),
  makeColumn({ id: 2, name: '进行中', agent_pickup: true }),
  makeColumn({ id: 3, name: '完成', is_done: true }),
]

function stateWith(actions: Partial<ActionFormRow>[]) {
  const base = emptyRuleForm()
  return {
    ...base,
    name: '测试规则',
    columnId: 2,
    actions: actions.map((a, i) => ({
      id: `a-${i}`,
      type: 'set_started_at' as const,
      mode: 'if_empty' as const,
      tag: '',
      text: '',
      columnId: 0,
      days: '7',
      priority: '2',
      humanOnly: false,
      ...a,
    })),
  }
}

describe('ruleForm 转换', () => {
  const types: ActionFormRow['type'][] = [
    'set_started_at',
    'set_finished_at',
    'clear_started_at',
    'clear_finished_at',
    'set_due',
    'clear_due',
    'set_priority',
    'add_tag',
    'remove_tag',
    'set_human_only',
    'add_comment',
    'move_to_column',
  ]

  it.each(types)('动作 %s 可转成请求体', (type) => {
    const row = stateWith([{ type, tag: 'x', text: 'hi', columnId: 3, days: '3', priority: '1', humanOnly: true, mode: 'always' }])
    const { input, errors } = formToCreateInput(row, columns)
    expect(errors).toEqual({})
    expect(input?.actions[0].type).toBe(type)
  })

  it('缺名称时拒绝', () => {
    const { errors } = formToCreateInput({ ...stateWith([{ type: 'clear_due' }]), name: '' }, columns)
    expect(errors.name).toBe('nameRequired')
  })

  it('动作过多时拒绝', () => {
    const s = stateWith(Array.from({ length: 21 }, () => ({ type: 'clear_due' as const })))
    expect(validateRuleForm(s, columns).actions).toBe('actionsLimit')
  })

  it('ruleToForm 与 formToCreateInput 往返', () => {
    const { input } = formToCreateInput(
      stateWith([
        { type: 'add_tag', tag: 'bug' },
        { type: 'move_to_column', columnId: 1 },
      ]),
      columns,
    )
    expect(input).toBeDefined()
    const back = ruleToForm({
      id: 1,
      board_id: 1,
      name: input!.name,
      enabled: true,
      trigger: input!.trigger,
      column_id: input!.column_id ?? 0,
      condition: input!.condition ?? '',
      actions: input!.actions,
      sort: 1,
      created_at: '',
      updated_at: '',
    })
    const again = formToCreateInput(back, columns)
    expect(again.errors).toEqual({})
    expect(again.input?.actions).toEqual(input!.actions)
  })
})