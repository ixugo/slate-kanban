// 标签列表的纯逻辑：去重、裁剪长度，方便单测。

import { CARD_TAG_MAX, CARD_TAGS_MAX } from '@/lib/limits'

export type TagApplyResult =
  | { ok: true; tags: string[] }
  | { ok: false; reason: 'empty' | 'duplicate' | 'limit' | 'tooLong' }

/** 去掉首尾空白并截断到单标签长度上限 */
export function normalizeTagInput(raw: string): string {
  return raw.trim().slice(0, CARD_TAG_MAX)
}

/** 在现有标签上追加一个；不区分大小写去重 */
export function applyTag(tags: string[], raw: string): TagApplyResult {
  const tag = normalizeTagInput(raw)
  if (!tag) return { ok: false, reason: 'empty' }
  if (tag.length > CARD_TAG_MAX) return { ok: false, reason: 'tooLong' }
  const lower = tag.toLowerCase()
  if (tags.some((t) => t.toLowerCase() === lower)) return { ok: false, reason: 'duplicate' }
  if (tags.length >= CARD_TAGS_MAX) return { ok: false, reason: 'limit' }
  return { ok: true, tags: [...tags, tag] }
}

/** 删除某一个标签（大小写精确匹配原串） */
export function removeTag(tags: string[], tag: string): string[] {
  return tags.filter((t) => t !== tag)
}

/** 删除最后一个标签 */
export function popTag(tags: string[]): string[] {
  return tags.slice(0, -1)
}