// 标签的显示名：有文字用文字，没有文字用颜色名，连颜色都没有就是"无文字"，给提示、无障碍名称和按钮名用。

import type { TFunction } from '@/i18n/react'
import { parseLabelColor } from '@/lib/labelColors'
import type { Tag } from '@/lib/api/types'

const SHADE_KEYS = { 1: 'dark', 2: 'medium', 3: 'light' } as const

/** 颜色编号的名称，例如“绿（浅）”；编号不认识返回空串 */
export function colorName(color: string, t: TFunction): string {
  const parsed = parseLabelColor(color)
  if (!parsed) return ''
  return t('label.colorName', { hue: t(`label.hues.${parsed.hue}`), shade: t(`label.shades.${SHADE_KEYS[parsed.shade]}`) })
}

/** 新看板起步的 6 个标签（服务端只存中文），按文字对应到译文的键；和服务端 seedTagTexts 保持一致 */
const SEED_TEXT_KEYS = new Map<string, 'feature' | 'docs' | 'mobile' | 'bug' | 'backend' | 'frontend'>([
  ['需求', 'feature'],
  ['文档', 'docs'],
  ['移动端', 'mobile'],
  ['Bug', 'bug'],
  ['服务端', 'backend'],
  ['前端', 'frontend'],
])

/** 标签文字的显示写法：和起步标签的中文完全相等时按界面语言翻译，其余原样显示 */
export function tagText(text: string, t: TFunction): string {
  const key = SEED_TEXT_KEYS.get(text)
  return key ? t(`label.seed.${key}`) : text
}

/** 标签的显示名 */
export function tagName(tag: Tag, t: TFunction): string {
  return tagText(tag.text, t) || colorName(tag.color, t) || t('label.blank')
}
