// 任务弹窗顶部：所在列、任务编号和更多操作。

import { XIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useTranslation } from '@/i18n/react'
import type { CardDetail, Column } from '@/lib/api/types'
import { useTemplateText } from '@/lib/useTemplateText'
import { DialogClose, DialogDescription, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { displayKey } from '@/lib/cardKey'
import { CardActions } from './CardActions'

interface CardHeaderBarProps {
  card: CardDetail | undefined
  columns: Column[]
  onColumn: (columnId: number) => void
  onDelete: () => void
  onArchive: () => void
  onFork: () => void
  actionBusy: boolean
}

/** 弹窗顶栏；任务还没加载出来时只显示通用标题 */
export function CardHeaderBar({ card, columns, onColumn, onDelete, onArchive, onFork, actionBusy }: CardHeaderBarProps) {
  const { t } = useTranslation()
  const { columnName } = useTemplateText()

  return (
    <div className="flex shrink-0 items-center gap-1.5 border-b border-border/60 py-3 pr-4 pl-4 sm:gap-3 sm:pl-6">
      {card?.archived_at && (
        <span className="shrink-0 rounded-full bg-secondary px-2 py-1 text-xs text-muted-foreground">{t('card.archive.readOnly')}</span>
      )}
      {card && !card.archived_at && (
        <Select value={String(card.column_id)} onValueChange={(v) => onColumn(Number(v))}>
          <SelectTrigger size="sm" aria-label={t('card.field.column')} className="max-w-[45%] rounded-full bg-secondary font-medium">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {columns.map((col) => (
              <SelectItem key={col.id} value={String(col.id)}>
                {columnName(col.name)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}
      <DialogTitle className="truncate font-mono text-ui font-medium tracking-normal text-muted-foreground">
        {card ? displayKey(card.key) : t('card.title')}
      </DialogTitle>
      <DialogDescription className="sr-only">{t('card.title')}</DialogDescription>
      <div className="ml-auto flex shrink-0 items-center gap-1 sm:gap-5">
        {card && (
          <CardActions archived={Boolean(card.archived_at)} busy={actionBusy} onFork={onFork} onArchive={onArchive} onDelete={onDelete} />
        )}
        <DialogClose asChild>
          <Button type="button" variant="secondary" size="icon-sm" aria-label={t('common.close')}>
            <XIcon className="size-3.5" />
          </Button>
        </DialogClose>
      </div>
    </div>
  )
}
