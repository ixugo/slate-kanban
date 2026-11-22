// 任务时间信息：只读，有值才显示一行。

import { useTranslation } from '@/i18n/react'
import type { CardDetail } from '@/lib/api/types'
import { formatDateTime } from '@/lib/format'

interface CardTimeMetaProps {
  card: CardDetail
  lang: string
}

/** 创建、更新、开始、完成时间 */
export function CardTimeMeta({ card, lang }: CardTimeMetaProps) {
  const { t } = useTranslation()
  const rows: { key: string; label: string; value: string; iso: string }[] = []
  const push = (label: string, iso: string | null, optional = false) => {
    if (optional && !iso) return
    const text = formatDateTime(iso, lang)
    if (!text) return
    rows.push({ key: label, label, value: text, iso: iso ?? '' })
  }
  push(t('card.field.created'), card.created_at)
  push(t('card.field.updated'), card.updated_at)
  push(t('card.field.started'), card.started_at, true)
  push(t('card.field.finished'), card.finished_at, true)
  if (rows.length === 0) return null
  return (
    <div className="space-y-1 px-1 text-xs text-muted-foreground">
      {rows.map((row) => (
        <div key={row.key} className="flex justify-between gap-2">
          <span>{row.label}</span>
          <time dateTime={row.iso} className="font-mono">{row.value}</time>
        </div>
      ))}
    </div>
  )
}