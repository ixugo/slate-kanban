// 根据触发器、列与动作列表生成一句可读规则名，供保存、列表展示与推荐规则文案复用。

import type { Column, Rule, RuleAction, TriggerType } from '@/lib/api/types'
import { RULE_NAME_MAX } from '@/lib/limits'
import type { TFunction } from '@/i18n/types'
import type { TranslationKey } from '@/i18n/types'
import type { ActionFormRow } from './ruleForm'

const ANY_COLUMN = 0

/** 按列 id 取展示名；找不到时用 #id */
function columnLabel(columns: Column[], id: number): string {
  if (id === ANY_COLUMN) return ''
  return columns.find((c) => c.id === id)?.name ?? `#${id}`
}

/** 单条动作用的自动命名片段键 */
function actionAutoKey(a: RuleAction): TranslationKey {
  const mode = a.mode ?? 'if_empty'
  switch (a.type) {
    case 'set_started_at':
      return mode === 'always'
        ? 'settings.rules.autoName.action.setStartedAlways'
        : 'settings.rules.autoName.action.setStarted'
    case 'set_finished_at':
      return mode === 'always'
        ? 'settings.rules.autoName.action.setFinishedAlways'
        : 'settings.rules.autoName.action.setFinished'
    case 'clear_started_at':
      return 'settings.rules.autoName.action.clearStarted'
    case 'clear_finished_at':
      return 'settings.rules.autoName.action.clearFinished'
    case 'set_due':
      return 'settings.rules.autoName.action.setDue'
    case 'clear_due':
      return 'settings.rules.autoName.action.clearDue'
    case 'set_priority':
      return 'settings.rules.autoName.action.setPriority'
    case 'add_tag':
      return 'settings.rules.autoName.action.addTag'
    case 'remove_tag':
      return 'settings.rules.autoName.action.removeTag'
    case 'set_human_only':
      return a.flag ? 'settings.rules.autoName.action.humanOn' : 'settings.rules.autoName.action.humanOff'
    case 'add_comment':
      return 'settings.rules.autoName.action.addComment'
    case 'move_to_column':
      return 'settings.rules.autoName.action.moveTo'
    default:
      return 'settings.rules.autoName.action.unknown'
  }
}

/** 动作参数，与 autoName.action.* 模板占位符对应 */
function actionAutoParams(a: RuleAction, columns: Column[]): Record<string, string | number> {
  const col = a.column_id ? columnLabel(columns, a.column_id) : ''
  switch (a.type) {
    case 'set_due':
      return { days: a.days ?? 0 }
    case 'set_priority':
      return { priority: a.priority ?? 0 }
    case 'add_tag':
    case 'remove_tag':
      return { tag: a.tag ?? '' }
    case 'add_comment':
      return { text: a.text ?? '' }
    case 'move_to_column':
      return { column: col }
    default:
      return {}
  }
}

/** 把动作列表译成用 join 连接的短句 */
function actionsPhrase(actions: RuleAction[], columns: Column[], t: TFunction): string {
  const join = t('settings.rules.autoName.join')
  return actions
    .map((a) => t(actionAutoKey(a), actionAutoParams(a, columns)))
    .join(join)
}

/** 触发器 + 列对应的 when 模板键 */
function whenAutoKey(trigger: TriggerType, columnId: number): TranslationKey {
  const part = columnId === ANY_COLUMN ? 'any' : 'column'
  return `settings.rules.autoName.when.${trigger}.${part}` as TranslationKey
}

/** 按字符数截断规则名，超出时加省略号（emoji 按单字符计） */
export function truncateRuleName(text: string, max = RULE_NAME_MAX): string {
  const chars = Array.from(text)
  if (chars.length <= max) return text
  const ell = '…'
  const take = Math.max(0, max - Array.from(ell).length)
  return `${chars.slice(0, take).join('')}${ell}`
}

/** 从触发器、列、动作生成一句话规则名 */
export function buildRuleAutoName(
  trigger: TriggerType,
  columnId: number,
  actions: RuleAction[],
  columns: Column[],
  t: TFunction,
): string {
  const actionsText = actionsPhrase(actions, columns, t)
  const whenKey = whenAutoKey(trigger, columnId)
  const whenParams: Record<string, string> = { actions: actionsText }
  if (columnId !== ANY_COLUMN) whenParams.column = columnLabel(columns, columnId)
  const full = t(whenKey, whenParams)
  return truncateRuleName(full)
}

/** 从已有规则生成自动名，便于与 rule.name 比较 */
export function autoNameForRule(rule: Rule, columns: Column[], t: TFunction): string {
  return buildRuleAutoName(rule.trigger, rule.column_id, rule.actions, columns, t)
}

/** 规则名是否等于自动生成的句子（忽略首尾空白） */
export function isAutoRuleName(rule: Rule, columns: Column[], t: TFunction): boolean {
  return rule.name.trim() === autoNameForRule(rule, columns, t)
}

/** 编辑器表单行转 RuleAction，仅用于自动命名预览 */
export function rowsToActionsForName(rows: ActionFormRow[]): RuleAction[] {
  return rows.map((row) => {
    const type = row.type
    switch (type) {
      case 'set_started_at':
      case 'set_finished_at':
        return { type, mode: row.mode || 'if_empty' }
      case 'clear_started_at':
      case 'clear_finished_at':
      case 'clear_due':
        return { type }
      case 'set_due':
        return { type, days: Number.parseInt(row.days, 10) || 0 }
      case 'set_priority':
        return { type, priority: Number.parseInt(row.priority, 10) || 0 }
      case 'add_tag':
      case 'remove_tag':
        return { type, tag: row.tag.trim() }
      case 'set_human_only':
        return { type, flag: row.humanOnly }
      case 'add_comment':
        return { type, text: row.text.trim() }
      case 'move_to_column':
        return { type, column_id: row.columnId }
      default:
        return { type: 'set_started_at' }
    }
  })
}

/** 从编辑器表单状态生成自动名 */
export function autoNameFromFormState(
  trigger: TriggerType,
  columnId: number,
  actions: ActionFormRow[],
  columns: Column[],
  t: TFunction,
): string {
  return buildRuleAutoName(trigger, columnId, rowsToActionsForName(actions), columns, t)
}