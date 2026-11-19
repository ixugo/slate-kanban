// 判断已有规则是否与推荐预设等价，避免重复展示「添加」入口。

import type { Column, Rule, RuleAction } from '@/lib/api/types'
import { buildRulePreset, type RulePresetId, RECOMMENDED_PRESET_IDS } from './rulePresets'
import { formToCreateInput } from './ruleForm'

/** 比较两条动作在业务上是否相同（缺失 mode 视为 if_empty） */
function actionEqual(a: RuleAction, b: RuleAction): boolean {
  if (a.type !== b.type) return false
  switch (a.type) {
    case 'set_started_at':
    case 'set_finished_at':
      return (a.mode ?? 'if_empty') === (b.mode ?? 'if_empty')
    case 'set_due':
      return a.days === b.days
    case 'set_priority':
      return a.priority === b.priority
    case 'add_tag':
    case 'remove_tag':
      return (a.tag ?? '') === (b.tag ?? '')
    case 'set_human_only':
      return Boolean(a.flag) === Boolean(b.flag)
    case 'add_comment':
      return (a.text ?? '') === (b.text ?? '')
    case 'move_to_column':
      return a.column_id === b.column_id
    default:
      return true
  }
}

/** 动作列表逐项等价 */
export function actionsEqual(a: RuleAction[], b: RuleAction[]): boolean {
  if (a.length !== b.length) return false
  return a.every((act, i) => actionEqual(act, b[i]))
}

/** 推荐预设都绑定具体列；找不到列时不展示推荐项 */
export function presetNeedsColumn(_presetId: RulePresetId): boolean {
  return true
}

/** 判断规则是否与某预设等价：触发器、列、动作序列相同且无筛选条件 */
export function ruleMatchesPreset(rule: Rule, presetId: RulePresetId, columns: Column[]): boolean {
  if (rule.condition.trim() !== '') return false
  const form = buildRulePreset(presetId, columns)
  if (presetNeedsColumn(presetId) && form.columnId === 0) return false
  const { input } = formToCreateInput({ ...form, name: 'check', enabled: true }, columns)
  if (!input) return false
  return (
    rule.trigger === input.trigger &&
    rule.column_id === input.column_id &&
    actionsEqual(rule.actions, input.actions)
  )
}

/** 列出尚未配置的推荐预设 id（且项目里有对应列） */
export function listMissingRecommendedPresets(columns: Column[], rules: Rule[]): RulePresetId[] {
  return RECOMMENDED_PRESET_IDS.filter((id) => {
    const form = buildRulePreset(id, columns)
    if (presetNeedsColumn(id) && form.columnId === 0) return false
    return !rules.some((r) => ruleMatchesPreset(r, id, columns))
  })
}