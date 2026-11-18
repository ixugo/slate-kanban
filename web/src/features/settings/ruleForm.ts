// 规则编辑器的表单状态与 API 请求体互转，以及保存前的客户端校验。

import type { ActionType, CreateRuleInput, Rule, RuleAction, TimeMode, TriggerType } from '@/lib/api/types'
import {
  CARD_PRIORITY_MAX,
  CARD_TAG_MAX,
  FILTER_MAX,
  RULE_ACTIONS_MAX,
  RULE_COMMENT_MAX,
  RULE_DUE_DAYS_MAX,
  RULE_NAME_MAX,
} from '@/lib/limits'
import type { Column } from '@/lib/api/types'

/** 编辑器里一个动作的表单行 */
export interface ActionFormRow {
  id: string
  type: ActionType
  mode: TimeMode
  tag: string
  text: string
  columnId: number
  days: string
  priority: string
  humanOnly: boolean
}

/** 整条规则的表单状态 */
export interface RuleFormState {
  name: string
  enabled: boolean
  trigger: TriggerType
  columnId: number
  condition: string
  actions: ActionFormRow[]
}

export type RuleFormErrors = Partial<
  Record<
    'name' | 'condition' | 'actions' | 'columnId',
    | 'nameTooLong'
    | 'conditionTooLong'
    | 'actionsRequired'
    | 'actionsLimit'
    | 'columnRequired'
    | 'actionInvalid'
  >
>

let actionSeq = 0

/** 生成动作行的本地唯一 id，方便 React 列表 key */
export function newActionId(): string {
  actionSeq += 1
  return `action-${actionSeq}`
}

/** 默认空白动作行 */
export function emptyActionRow(type: ActionType = 'set_started_at'): ActionFormRow {
  return {
    id: newActionId(),
    type,
    mode: 'if_empty',
    tag: '',
    text: '',
    columnId: 0,
    days: '7',
    priority: '2',
    humanOnly: false,
  }
}

/** 从服务端规则回填表单 */
export function ruleToForm(rule: Rule): RuleFormState {
  return {
    name: rule.name,
    enabled: rule.enabled,
    trigger: rule.trigger,
    columnId: rule.column_id,
    condition: rule.condition,
    actions: rule.actions.map((a) => actionToRow(a)),
  }
}

function actionToRow(a: RuleAction): ActionFormRow {
  return {
    id: newActionId(),
    type: a.type,
    mode: a.mode ?? 'if_empty',
    tag: a.tag ?? '',
    text: a.text ?? '',
    columnId: a.column_id ?? 0,
    days: a.days !== undefined ? String(a.days) : '7',
    priority: a.priority !== undefined ? String(a.priority) : '2',
    humanOnly: a.flag ?? false,
  }
}

/** 表单转成创建/更新用的请求体；校验不通过抛不出错，只返回 errors */
export function formToCreateInput(
  state: RuleFormState,
  columns: Column[],
  resolveName?: (state: RuleFormState) => string,
): { input?: CreateRuleInput; errors: RuleFormErrors } {
  const errors = validateRuleForm(state, columns)
  if (Object.keys(errors).length > 0) return { errors }
  const trimmed = state.name.trim()
  const name = trimmed !== '' ? trimmed : (resolveName?.(state) ?? '')
  return {
    errors: {},
    input: {
      name,
      enabled: state.enabled,
      trigger: state.trigger,
      column_id: state.columnId,
      condition: state.condition.trim(),
      actions: state.actions.map((row) => rowToAction(row)),
    },
  }
}

/** 更新规则时 actions 必须非空数组 */
export function formToUpdateInput(
  state: RuleFormState,
  columns: Column[],
  resolveName?: (state: RuleFormState) => string,
): { actions?: RuleAction[]; patch: Omit<CreateRuleInput, 'actions'> & { actions?: RuleAction[] }; errors: RuleFormErrors } {
  const { input, errors } = formToCreateInput(state, columns, resolveName)
  if (!input || Object.keys(errors).length > 0) return { errors, patch: {} as CreateRuleInput }
  const { actions, ...rest } = input
  return { errors: {}, actions, patch: { ...rest, actions } }
}

function rowToAction(row: ActionFormRow): RuleAction {
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
      return { type, days: Number.parseInt(row.days, 10) }
    case 'set_priority':
      return { type, priority: Number.parseInt(row.priority, 10) }
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
}

/** 校验整条规则表单 */
export function validateRuleForm(state: RuleFormState, columns: Column[]): RuleFormErrors {
  const errors: RuleFormErrors = {}
  const name = state.name.trim()
  if (name !== '' && Array.from(name).length > RULE_NAME_MAX) errors.name = 'nameTooLong'
  if (Array.from(state.condition).length > FILTER_MAX) errors.condition = 'conditionTooLong'
  if (state.actions.length === 0) errors.actions = 'actionsRequired'
  if (state.actions.length > RULE_ACTIONS_MAX) errors.actions = 'actionsLimit'
  for (const row of state.actions) {
    if (!validateActionRow(row, columns)) {
      errors.actions = 'actionInvalid'
      break
    }
  }
  if (state.columnId !== 0 && !columns.some((c) => c.id === state.columnId)) {
    errors.columnId = 'columnRequired'
  }
  return errors
}

function validateActionRow(row: ActionFormRow, columns: Column[]): boolean {
  switch (row.type) {
    case 'set_due': {
      const days = Number.parseInt(row.days, 10)
      return Number.isFinite(days) && days >= 1 && days <= RULE_DUE_DAYS_MAX
    }
    case 'set_priority': {
      const p = Number.parseInt(row.priority, 10)
      return Number.isFinite(p) && p >= 0 && p <= CARD_PRIORITY_MAX
    }
    case 'add_tag':
    case 'remove_tag': {
      const tag = row.tag.trim()
      return tag !== '' && Array.from(tag).length <= CARD_TAG_MAX
    }
    case 'add_comment': {
      const text = row.text.trim()
      return text !== '' && Array.from(text).length <= RULE_COMMENT_MAX
    }
    case 'move_to_column':
      return row.columnId !== 0 && columns.some((c) => c.id === row.columnId)
    case 'set_started_at':
    case 'set_finished_at':
      return row.mode === 'if_empty' || row.mode === 'always'
    default:
      return true
  }
}

/** 新建规则的初始表单 */
export function emptyRuleForm(): RuleFormState {
  return {
    name: '',
    enabled: true,
    trigger: 'card_entered',
    columnId: 0,
    condition: '',
    actions: [emptyActionRow()],
  }
}