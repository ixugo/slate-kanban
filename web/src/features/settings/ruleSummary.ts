// 自动化规则的白话摘要：产出 i18n 模板键和参数，由各语言拼成完整句子。

import type { Column, Rule, RuleAction, TriggerType } from '@/lib/api/types'

/** 一条动作的摘要片段 */
export interface ActionSummary {
  key: string
  params: Record<string, string | number>
}

/** 整段「当…时，做…」的片段 */
export interface RuleSummaryParts {
  whenKey: string
  whenParams: Record<string, string>
  conditionKey?: string
  actions: ActionSummary[]
}

const ANY = 0

/** 按列 id 找名称，找不到用 #id */
function columnLabel(columns: Column[], id: number): string {
  if (id === ANY) return ''
  return columns.find((c) => c.id === id)?.name ?? `#${id}`
}

/** 触发器对应的 when 模板键 */
function whenKey(trigger: TriggerType, columnId: number): { key: string; params: Record<string, string> } {
  const colPart = columnId === ANY ? 'any' : 'column'
  const base = `settings.rules.summary.when.${trigger}.${colPart}`
  if (columnId === ANY) return { key: base, params: {} }
  return { key: base, params: { column: '' } }
}

function actionSummary(a: RuleAction, columns: Column[]): ActionSummary {
  const col = a.column_id ? columnLabel(columns, a.column_id) : ''
  switch (a.type) {
    case 'set_started_at':
      return {
        key: a.mode === 'always' ? 'settings.rules.summary.action.setStartedAlways' : 'settings.rules.summary.action.setStartedIfEmpty',
        params: {},
      }
    case 'set_finished_at':
      return {
        key: a.mode === 'always' ? 'settings.rules.summary.action.setFinishedAlways' : 'settings.rules.summary.action.setFinishedIfEmpty',
        params: {},
      }
    case 'clear_started_at':
      return { key: 'settings.rules.summary.action.clearStarted', params: {} }
    case 'clear_finished_at':
      return { key: 'settings.rules.summary.action.clearFinished', params: {} }
    case 'set_due':
      return { key: 'settings.rules.summary.action.setDue', params: { days: a.days ?? 0 } }
    case 'clear_due':
      return { key: 'settings.rules.summary.action.clearDue', params: {} }
    case 'set_priority':
      return { key: 'settings.rules.summary.action.setPriority', params: { priority: a.priority ?? 0 } }
    case 'add_tag':
      return { key: 'settings.rules.summary.action.addTag', params: { tag: a.tag ?? '' } }
    case 'remove_tag':
      return { key: 'settings.rules.summary.action.removeTag', params: { tag: a.tag ?? '' } }
    case 'set_human_only':
      return {
        key: a.flag ? 'settings.rules.summary.action.humanOn' : 'settings.rules.summary.action.humanOff',
        params: {},
      }
    case 'add_comment':
      return { key: 'settings.rules.summary.action.addComment', params: { text: a.text ?? '' } }
    case 'move_to_column':
      return { key: 'settings.rules.summary.action.moveTo', params: { column: col } }
    default:
      return { key: 'settings.rules.summary.action.unknown', params: {} }
  }
}

/** 生成规则摘要各片段，组件里用 t() 拼接 */
export function buildRuleSummary(rule: Rule, columns: Column[]): RuleSummaryParts {
  const when = whenKey(rule.trigger, rule.column_id)
  if (rule.column_id !== ANY) when.params.column = columnLabel(columns, rule.column_id)
  const parts: RuleSummaryParts = {
    whenKey: when.key,
    whenParams: when.params,
    actions: rule.actions.map((a) => actionSummary(a, columns)),
  }
  if (rule.condition.trim() !== '') {
    parts.conditionKey = 'settings.rules.summary.condition'
  }
  return parts
}

/** 把动作摘要列表用「、」连接前的中间句键 */
export const RULE_SUMMARY_THEN_KEY = 'settings.rules.summary.then'