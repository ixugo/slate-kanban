// 设置页表单的客户端校验：规则和后端 board.param、colum.param 对齐，减少白等一次无效请求。

import {
  BOARD_NAME_MAX,
  COLUMN_DESC_MAX,
  COLUMN_INSTRUCTION_MAX,
  COLUMN_NAME_MAX,
  COLUMNS_PER_BOARD_MAX,
} from '@/lib/limits'
import type { Column } from '@/lib/api/types'

/** 常规设置表单的字段 */
export interface GeneralFormValues {
  name: string
  description: string
}

/** 出错字段对应的文案键后缀（settings.general.errors.<后缀>） */
export type GeneralFormErrors = Partial<Record<'name', 'nameRequired'>>

/** 列表单的字段 */
export interface ColumnFormValues {
  name: string
  description: string
  instruction: string
  agentPickup: boolean
  isDone: boolean
  nextColumnId: number
}

export type ColumnFormErrors = Partial<
  Record<'name' | 'nextColumnId', 'nameRequired' | 'nextSelf' | 'limitReached'>
>

/** 看板常规字段校验 */
export function validateGeneralForm(values: GeneralFormValues): GeneralFormErrors {
  const errors: GeneralFormErrors = {}
  const name = values.name.trim()
  if (name === '' || Array.from(name).length > BOARD_NAME_MAX) errors.name = 'nameRequired'
  return errors
}

/** 列字段校验；selfId 用于判断「下一列不能是自己」 */
export function validateColumnForm(
  values: ColumnFormValues,
  selfId: number,
  columnCount: number,
  isCreate: boolean,
): ColumnFormErrors {
  const errors: ColumnFormErrors = {}
  const name = values.name.trim()
  if (name === '' || Array.from(name).length > COLUMN_NAME_MAX) errors.name = 'nameRequired'
  if (Array.from(values.description).length > COLUMN_DESC_MAX) errors.name = 'nameRequired'
  if (Array.from(values.instruction).length > COLUMN_INSTRUCTION_MAX) errors.name = 'nameRequired'
  if (values.nextColumnId !== 0 && values.nextColumnId === selfId) errors.nextColumnId = 'nextSelf'
  if (isCreate && columnCount >= COLUMNS_PER_BOARD_MAX) errors.name = 'limitReached'
  return errors
}

/** 某一列里有多少张卡片 */
export function countCardsInColumn(columnId: number, cards: { column_id: number }[]): number {
  return cards.filter((c) => c.column_id === columnId).length
}

/** 按 sort 排好序的列 id 列表上移一位；已在最上返回 null */
export function moveColumnUp(columns: Column[], columnId: number): number[] | null {
  const sorted = [...columns].sort((a, b) => a.sort - b.sort || a.id - b.id)
  const idx = sorted.findIndex((c) => c.id === columnId)
  if (idx <= 0) return null
  const ids = sorted.map((c) => c.id)
  ;[ids[idx - 1], ids[idx]] = [ids[idx], ids[idx - 1]]
  return ids
}

/** 下移一位 */
export function moveColumnDown(columns: Column[], columnId: number): number[] | null {
  const sorted = [...columns].sort((a, b) => a.sort - b.sort || a.id - b.id)
  const idx = sorted.findIndex((c) => c.id === columnId)
  if (idx < 0 || idx >= sorted.length - 1) return null
  const ids = sorted.map((c) => c.id)
  ;[ids[idx], ids[idx + 1]] = [ids[idx + 1], ids[idx]]
  return ids
}