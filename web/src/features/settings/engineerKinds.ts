// 工程师类型的展示信息：下拉里的顺序、文案键和标签颜色。五种标签是同一种填充样式，只有颜色不同。

import type { TranslationKey } from '@/i18n/types'
import type { EngineerKind } from '@/lib/api/types'

/** 一种类型的展示信息 */
interface KindMeta {
  /** 文案键 */
  label: TranslationKey
  /** 标签的颜色类名（淡底加深字，深色模式字更亮） */
  color: string
}

/** 类型的显示顺序就是下拉里的顺序；dev 用主题主色，review 用 agent 色，其余各一种颜色 */
export const ENGINEER_KINDS: readonly EngineerKind[] = ['dev', 'frontend', 'backend', 'ui', 'review']

export const KIND_META: Record<EngineerKind, KindMeta> = {
  dev: { label: 'settings.engineers.kindDev', color: 'bg-primary/12 text-primary' },
  frontend: { label: 'settings.engineers.kindFrontend', color: 'bg-sky-500/12 text-sky-700 dark:text-sky-300' },
  backend: { label: 'settings.engineers.kindBackend', color: 'bg-emerald-500/12 text-emerald-700 dark:text-emerald-300' },
  ui: { label: 'settings.engineers.kindUi', color: 'bg-pink-500/12 text-pink-700 dark:text-pink-300' },
  review: { label: 'settings.engineers.kindReview', color: 'bg-agent/12 text-agent' },
}

/** 这种类型是不是做研发（除了测试都是）：只有研发类工程师能被卡片绑定 */
export function isDevKind(kind: EngineerKind): boolean {
  return kind !== 'review'
}
