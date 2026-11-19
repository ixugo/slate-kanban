// 推荐自动化规则的预设：用于一键创建或填充编辑器，仅保留时间追踪三条。

import type { Column, CreateRuleInput } from '@/lib/api/types'
import { localizeColumnName } from '@/lib/templateText'
import { emptyActionRow, emptyRuleForm, formToCreateInput, type RuleFormState } from './ruleForm'

export type RulePresetId =
  | 'enter_progress_start'
  | 'enter_done_finish'
  | 'leave_done_clear_finish'

/** 设置页「推荐规则」仅展示这三条 */
export const RECOMMENDED_PRESET_IDS: RulePresetId[] = [
  'enter_progress_start',
  'enter_done_finish',
  'leave_done_clear_finish',
]

/** 按条件找列 id，找不到返回 0 */
function findColumn(columns: Column[], pred: (c: Column) => boolean): number {
  return columns.find(pred)?.id ?? 0
}

/** 根据预设 id 生成表单初值；找不到对应列时 columnId 为 0 */
export function buildRulePreset(id: RulePresetId, columns: Column[]): RuleFormState {
  const base = emptyRuleForm()
  const progressId = findColumn(columns, (c) => localizeColumnName(c.name, 'en') === 'In progress')
  const doneId = findColumn(columns, (c) => c.is_done)
  switch (id) {
    case 'enter_progress_start':
      return {
        ...base,
        name: '',
        trigger: 'card_entered',
        columnId: progressId,
        actions: [emptyActionRow('set_started_at')],
      }
    case 'enter_done_finish':
      return {
        ...base,
        trigger: 'card_entered',
        columnId: doneId,
        actions: [emptyActionRow('set_finished_at')],
      }
    case 'leave_done_clear_finish':
      return {
        ...base,
        trigger: 'card_left',
        columnId: doneId,
        actions: [emptyActionRow('clear_finished_at')],
      }
    default:
      return base
  }
}

/** 把预设直接转成创建请求体；名称由调用方传入（一般为自动命名） */
export function presetToCreateInput(presetId: RulePresetId, columns: Column[], name: string): CreateRuleInput | undefined {
  const form = buildRulePreset(presetId, columns)
  if (form.columnId === 0) return undefined
  const { input, errors } = formToCreateInput({ ...form, name, enabled: true }, columns)
  if (!input || Object.keys(errors).length > 0) return undefined
  return input
}