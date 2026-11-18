// 常用自动化规则的预设：只填充表单，用户仍可修改后再保存。

import type { Column } from '@/lib/api/types'
import { emptyActionRow, emptyRuleForm, type RuleFormState } from './ruleForm'

export type RulePresetId =
  | 'enter_progress_start'
  | 'enter_done_finish'
  | 'leave_done_clear_finish'
  | 'created_add_tag'
  | 'enter_set_priority'

/** 预设清单：id 与展示用文案键 */
export const RULE_PRESETS: { id: RulePresetId; labelKey: string }[] = [
  { id: 'enter_progress_start', labelKey: 'settings.rules.presets.enterProgressStart' },
  { id: 'enter_done_finish', labelKey: 'settings.rules.presets.enterDoneFinish' },
  { id: 'leave_done_clear_finish', labelKey: 'settings.rules.presets.leaveDoneClearFinish' },
  { id: 'created_add_tag', labelKey: 'settings.rules.presets.createdAddTag' },
  { id: 'enter_set_priority', labelKey: 'settings.rules.presets.enterSetPriority' },
]

/** 按名称或 agent_pickup / is_done 找列 id */
function findColumn(columns: Column[], pred: (c: Column) => boolean): number {
  return columns.find(pred)?.id ?? 0
}

/** 根据预设 id 生成表单初值；找不到对应列时 columnId 为 0（任意列） */
export function buildRulePreset(id: RulePresetId, columns: Column[]): RuleFormState {
  const base = emptyRuleForm()
  const progressId = findColumn(columns, (c) => c.agent_pickup)
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
    case 'created_add_tag':
      return {
        ...base,
        trigger: 'card_created',
        columnId: 0,
        actions: [{ ...emptyActionRow('add_tag'), tag: 'auto' }],
      }
    case 'enter_set_priority':
      return {
        ...base,
        trigger: 'card_entered',
        columnId: progressId,
        actions: [{ ...emptyActionRow('set_priority'), priority: '2' }],
      }
    default:
      return base
  }
}