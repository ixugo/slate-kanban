import { describe, expect, it } from 'vitest'
import { en } from '@/i18n/locales/en'
import { makeColumn } from '@/test/factories'
import type { Rule } from '@/lib/api/types'
import { buildRuleSummary, RULE_SUMMARY_THEN_KEY } from './ruleSummary'
import { buildRulePreset } from './rulePresets'

const columns = [
  makeColumn({ id: 2, name: 'In progress' }),
  makeColumn({ id: 3, name: 'Done', is_done: true }),
]

describe('规则摘要', () => {
  it('进入列触发器带列名', () => {
    const parts = buildRuleSummary(
      {
        id: 1,
        board_id: 1,
        name: 'r',
        enabled: true,
        trigger: 'card_entered',
        column_id: 2,
        condition: '',
        actions: [{ type: 'set_started_at', mode: 'if_empty' }],
        sort: 1,
        created_at: '',
        updated_at: '',
      } satisfies Rule,
      columns,
    )
    expect(parts.whenKey).toBe('settings.rules.summary.when.card_entered.column')
    expect(parts.whenParams.column).toBe('In progress')
    expect(parts.actions[0].key).toContain('setStartedIfEmpty')
  })

  it('有条件时带上 condition 键', () => {
    const parts = buildRuleSummary(
      {
        id: 1,
        board_id: 1,
        name: 'r',
        enabled: true,
        trigger: 'card_created',
        column_id: 0,
        condition: 'tag:bug',
        actions: [{ type: 'add_tag', tag: 'auto' }],
        sort: 1,
        created_at: '',
        updated_at: '',
      },
      columns,
    )
    expect(parts.conditionKey).toBe('settings.rules.summary.condition')
  })

  it('THEN 键存在于英文文案', () => {
    expect(en.settings.rules.summary.then).toBeTruthy()
    expect(RULE_SUMMARY_THEN_KEY).toBe('settings.rules.summary.then')
  })
})

describe('规则预设', () => {
  it('进入进行中预设选中 agent 列', () => {
    const form = buildRulePreset('enter_progress_start', columns)
    expect(form.columnId).toBe(2)
    expect(form.actions[0].type).toBe('set_started_at')
  })
})