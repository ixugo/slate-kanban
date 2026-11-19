import { describe, expect, it } from 'vitest'
import { zhCN } from '@/i18n/locales/zh-CN'
import { en } from '@/i18n/locales/en'
import type { TFunction } from '@/i18n/types'
import { makeColumn } from '@/test/factories'
import { buildRuleAutoName, truncateRuleName, autoNameForRule } from './ruleAutoName'
import type { Rule } from '@/lib/api/types'

const columns = [
  makeColumn({ id: 2, name: '进行中' }),
  makeColumn({ id: 3, name: '完成', is_done: true }),
]

/** 简易 t：按路径从文案树取值并替换 {{var}} */
function makeT(messages: typeof zhCN): TFunction {
  const fn = ((key: string, params?: Record<string, string | number>) => {
    const parts = key.split('.')
    let cur: unknown = messages
    for (const p of parts) {
      if (cur && typeof cur === 'object' && p in cur) cur = (cur as Record<string, unknown>)[p]
      else return key
    }
    if (typeof cur !== 'string') return key
    return cur.replace(/\{\{(\w+)\}\}/g, (_, name: string) => String(params?.[name] ?? ''))
  }) as TFunction
  return fn
}

describe('buildRuleAutoName', () => {
  it('中文：进入列时记录开始时间', () => {
    const t = makeT(zhCN)
    const name = buildRuleAutoName(
      'card_entered',
      2,
      [{ type: 'set_started_at', mode: 'if_empty' }],
      columns,
      t,
    )
    expect(name).toBe('进入「进行中」时：记录开始时间')
  })

  it('英文：同等含义', () => {
    const t = makeT(en)
    const enCols = [
      makeColumn({ id: 2, name: 'In progress' }),
      makeColumn({ id: 3, name: 'Done', is_done: true }),
    ]
    const name = buildRuleAutoName(
      'card_entered',
      2,
      [{ type: 'set_started_at', mode: 'if_empty' }],
      enCols,
      t,
    )
    expect(name).toBe('When a card enters “In progress”: record start time')
  })

  it('多动作用顿号连接', () => {
    const t = makeT(zhCN)
    const name = buildRuleAutoName(
      'card_entered',
      3,
      [{ type: 'set_finished_at' }, { type: 'add_tag', tag: 'done' }],
      columns,
      t,
    )
    expect(name).toContain('记录完成时间')
    expect(name).toContain('添加标签「done」')
    expect(name).toContain('、')
  })

  it('超长按字符截断并加省略号', () => {
    const long = '甲'.repeat(120)
    const out = truncateRuleName(long, 100)
    expect(Array.from(out).length).toBeLessThanOrEqual(100)
    expect(out.endsWith('…')).toBe(true)
  })

  it('emoji 按一个字符计', () => {
    const text = '😀'.repeat(99) + '尾'
    const out = truncateRuleName(text, 100)
    expect(Array.from(out).length).toBe(100)
  })
})

describe('autoNameForRule', () => {
  it('与规则字段一致', () => {
    const t = makeT(zhCN)
    const rule: Rule = {
      id: 1,
      board_id: 1,
      name: '占位',
      enabled: true,
      trigger: 'card_left',
      column_id: 3,
      condition: '',
      actions: [{ type: 'clear_finished_at' }],
      sort: 1,
      created_at: '',
      updated_at: '',
    }
    expect(autoNameForRule(rule, columns, t)).toBe('离开「完成」时：清除完成时间')
  })
})