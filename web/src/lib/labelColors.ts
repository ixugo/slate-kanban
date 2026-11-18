// 标签颜色：10 个色相，每个三档深浅（1 最深，3 最浅），共 30 种，编号和服务端一致，例如 green-2。

import type { CSSProperties } from 'react'

/** 色相，顺序就是色板里的列顺序（上五个、下五个） */
export const LABEL_HUES = ['green', 'yellow', 'orange', 'red', 'purple', 'blue', 'sky', 'lime', 'pink', 'gray'] as const
export type LabelHue = (typeof LABEL_HUES)[number]

/** 深浅档位，1 最深 */
export const LABEL_SHADES = [1, 2, 3] as const

/** 每个色相从深到浅的三个背景色 */
const BACKGROUNDS: Record<LabelHue, [string, string, string]> = {
  green: ['#164b35', '#1f845a', '#4bce97'],
  yellow: ['#533f04', '#946f00', '#f5cd47'],
  orange: ['#5f3811', '#c25100', '#fea362'],
  red: ['#5d1f1a', '#c9372c', '#f87168'],
  purple: ['#352c63', '#6e5dc6', '#b8acf6'],
  blue: ['#09326c', '#0c66e4', '#85b8ff'],
  sky: ['#1d474c', '#227d9b', '#6cc3e0'],
  lime: ['#37471f', '#5b7f24', '#94c748'],
  pink: ['#50253f', '#cd519d', '#e774bb'],
  gray: ['#2c333a', '#626f86', '#b3b9c4'],
}

/** 浅色底上的字用深色，其余用白色，保证读得清 */
const DARK_TEXT = '#1d2125'
const LIGHT_TEXT = '#ffffff'

/** 拆出色相和深浅；不认识的编号返回 null */
export function parseLabelColor(color: string): { hue: LabelHue; shade: 1 | 2 | 3 } | null {
  const [hue, shade] = color.split('-')
  const n = Number(shade)
  if (!(LABEL_HUES as readonly string[]).includes(hue) || (n !== 1 && n !== 2 && n !== 3)) return null
  return { hue: hue as LabelHue, shade: n }
}

/** 颜色编号对应的内联样式；没有颜色或编号不认识时返回 undefined，调用方用默认样式 */
export function labelStyle(color: string): CSSProperties | undefined {
  const parsed = parseLabelColor(color)
  if (!parsed) return undefined
  return { backgroundColor: BACKGROUNDS[parsed.hue][parsed.shade - 1], color: parsed.shade === 3 ? DARK_TEXT : LIGHT_TEXT }
}
