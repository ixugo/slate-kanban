// 规则编辑器里「更多选项」默认是否展开等纯逻辑，便于单测与组件复用。

import type { Column, Rule } from '@/lib/api/types'
import type { TFunction } from '@/i18n/types'
import { isAutoRuleName } from './ruleAutoName'
import type { RuleFormState } from './ruleForm'

/** 是否存在「总是覆盖」的开始/结束时间动作 */
export function hasAlwaysTimeMode(state: RuleFormState): boolean {
  return state.actions.some(
    (a) => (a.type === 'set_started_at' || a.type === 'set_finished_at') && a.mode === 'always',
  )
}

/** 编辑已有规则时，更多选项是否默认展开 */
export function shouldExpandMoreOptions(
  rule: Rule | undefined,
  state: RuleFormState,
  columns: Column[],
  t: TFunction,
): boolean {
  if (!rule) return false
  if (state.condition.trim() !== '') return true
  if (rule.name.trim() !== '' && !isAutoRuleName(rule, columns, t)) return true
  if (hasAlwaysTimeMode(state)) return true
  return false
}