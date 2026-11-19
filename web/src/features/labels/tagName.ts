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

/** 标签的显示名 */
export function tagName(tag: Tag, t: TFunction): string {
  return tag.text || colorName(tag.color, t) || t('label.blank')
}
