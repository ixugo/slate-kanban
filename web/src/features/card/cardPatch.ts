// 修改任务的本地预测：按提交的字段算出"改完大概是什么样"，供乐观更新先显示；最终以服务端返回为准。

import type { Card, Tag, UpdateCardInput } from '@/lib/api/types'

/** 时间字段：空串表示清除，其余原样 */
function toTime(value: string): string | null {
  return value === '' ? null : value
}

/**
 * 把 input 里出现的字段套到任务上。tags（带文字标签的文字）由标签库按 tag_ids 推出，
 * 依赖导致的 waiting、dependencies 要服务端算，这里不预测，等响应回来校准。
 */
export function applyCardPatch<T extends Card & { tags?: string[] }>(card: T, input: UpdateCardInput, library: Tag[]): T {
  const next: T = { ...card }
  if (input.title !== undefined) next.title = input.title
  if (input.description !== undefined) next.description = input.description
  if (input.priority !== undefined) next.priority = input.priority
  if (input.human_only !== undefined) next.human_only = input.human_only
  if (input.engineer_id !== undefined) next.engineer_id = input.engineer_id
  if (input.waits_on !== undefined) next.waits_on = input.waits_on
  if (input.branch !== undefined) next.branch = input.branch
  if (input.url !== undefined) next.url = input.url
  if (input.due_at !== undefined) next.due_at = toTime(input.due_at)
  if (input.started_at !== undefined) next.started_at = toTime(input.started_at)
  if (input.finished_at !== undefined) next.finished_at = toTime(input.finished_at)
  if (input.tag_ids !== undefined) {
    next.tag_ids = input.tag_ids
    if (next.tags !== undefined) {
      const byId = new Map(library.map((tag) => [tag.id, tag.text]))
      next.tags = input.tag_ids.map((id) => byId.get(id) ?? '').filter((text) => text !== '')
    }
  }
  return next
}

/** 放弃领取后的任务：领取者、租约、承担的工程师都清掉 */
export function clearClaim<T extends Card & { claim_active?: boolean }>(card: T): T {
  const next: T = { ...card, claimed_by: '', claim_expires_at: null, claim_engineer_id: 0 }
  if (next.claim_active !== undefined) next.claim_active = false
  return next
}
