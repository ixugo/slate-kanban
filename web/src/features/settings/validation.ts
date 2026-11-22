// 设置页表单的客户端校验：规则和后端 board.param、colum.param 对齐，减少白等一次无效请求。

import {
  BOARD_NAME_MAX,
  COLUMN_INSTRUCTION_MAX,
  COLUMN_NAME_MAX,
  COLUMNS_PER_BOARD_MAX,
  ENGINEERS_PER_BOARD_MAX,
  WEBHOOK_NAME_MAX,
  WEBHOOK_URL_MAX,
} from '@/lib/limits'
import type { WebhookType } from '@/lib/api/types'
import type { Column, Engineer, EngineerKind } from '@/lib/api/types'

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
  instruction: string
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
  if (Array.from(values.instruction).length > COLUMN_INSTRUCTION_MAX) errors.name = 'nameRequired'
  if (values.nextColumnId !== 0 && values.nextColumnId === selfId) errors.nextColumnId = 'nextSelf'
  if (isCreate && columnCount >= COLUMNS_PER_BOARD_MAX) errors.name = 'limitReached'
  return errors
}

/** 某一列里有多少个任务 */
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

/** 表单里的一位工程师；id 为 0 表示新招募 */
export interface Draft {
  id: number
  name: string
  kind: EngineerKind
  prompt: string
  /** 头像随机种子，空表示沿用原来的（新招募的由服务端生成） */
  avatar: string
}

/** 校验失败的原因，对应 settings.engineers.errors 下的文案键 */
export type DraftError = 'nameRequired' | 'promptRequired' | 'nameTaken' | 'limitReached'

/** 校验一位工程师：名称和提示词必填，名称在名册里不重名（不区分大小写），名册不超过上限 */
export function validateDraft(draft: Draft, all: Engineer[]): DraftError | null {
  if (draft.name.trim() === '') return 'nameRequired'
  if (draft.prompt.trim() === '') return 'promptRequired'
  const key = draft.name.trim().toLowerCase()
  if (all.some((e) => e.id !== draft.id && e.name.trim().toLowerCase() === key)) return 'nameTaken'
  if (draft.id === 0 && all.length >= ENGINEERS_PER_BOARD_MAX) return 'limitReached'
  return null
}

/** webhook 表单字段 */
export interface WebhookFormValues {
  name: string
  type: WebhookType
  url: string
  secret: string
  secretClear: boolean
  enabled: boolean
  eventTypes: string[]
  actorKinds: string[]
  moveColumnIds: number[]
}

export type WebhookFormErrors = Partial<Record<'name' | 'url', 'nameRequired' | 'urlRequired' | 'urlHttps'>>

const WEBHOOK_HOSTS: Record<WebhookType, string[]> = {
  dingtalk: ['oapi.dingtalk.com'],
  wecom: ['qyapi.weixin.qq.com'],
  feishu: ['open.feishu.cn', 'open.larksuite.com'],
}

/** webhook 表单客户端校验，与后端 URL 规则对齐 */
export function validateWebhookForm(values: WebhookFormValues, type: WebhookType): WebhookFormErrors {
  const errors: WebhookFormErrors = {}
  const name = values.name.trim()
  if (name === '' || Array.from(name).length > WEBHOOK_NAME_MAX) errors.name = 'nameRequired'
  const url = values.url.trim()
  if (url === '' || Array.from(url).length > WEBHOOK_URL_MAX) {
    errors.url = 'urlRequired'
    return errors
  }
  try {
    const parsed = new URL(url)
    const host = parsed.hostname.toLowerCase()
    if (parsed.protocol !== 'https:' || !WEBHOOK_HOSTS[type].includes(host)) {
      errors.url = 'urlHttps'
    }
  } catch {
    errors.url = 'urlRequired'
  }
  return errors
}
