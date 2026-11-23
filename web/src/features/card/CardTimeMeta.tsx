// 任务时间信息：只读，有值才显示一行。

import { useTranslation } from '@/i18n/react'
import type { CardDetail } from '@/lib/api/types'
import { formatCalendarDateTime, formatDateTime } from '@/lib/format'

interface CardTimeMetaProps {
  card: CardDetail
  lang: string
}

/** 创建、开始、完成时间 */
export function CardTimeMeta({ card, lang }: CardTimeMetaProps) {
  const { t } = useTranslation()
  const now = Date.now()
  const rows: { label: string; value: string; iso: string }[] = []
  const push = (label: string, iso: string | null) => {
    if (!iso) return
    const value = formatCalendarDateTime(iso, lang, now)
    if (value) rows.push({ label, value, iso })
  }
  push(t('card.field.created'), card.created_at)
  push(t('card.field.started'), card.started_at)
  push(t('card.field.finished'), card.finished_at)
  if (rows.length === 0) return null
  return (
    <div className="space-y-1 px-1 text-xs text-muted-foreground">
      {rows.map((row) => (
        <div key={row.label} className="flex justify-between gap-2">
          <span>{row.label}</span>
          <time dateTime={row.iso} title={formatDateTime(row.iso, lang)} className="font-mono">{row.value}</time>
        </div>
      ))}
    </div>
  )
}
