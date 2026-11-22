// 任务弹窗顶部：所在列（下拉，选了就移动任务）、任务编号、删除按钮。对齐 Trello 卡片顶部的「列名按钮」。

import { ArchiveIcon, Trash2Icon } from 'lucide-react'
import { useTranslation } from '@/i18n/react'
import type { CardDetail, Column } from '@/lib/api/types'
import { useTemplateText } from '@/lib/useTemplateText'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { displayKey } from '@/lib/cardKey'

interface CardHeaderBarProps {
  card: CardDetail | undefined
  columns: Column[]
  onColumn: (columnId: number) => void
  onDelete: () => void
  deleteBusy: boolean
  onArchive: () => void
  actionBusy: boolean
}

/** 弹窗顶栏；任务还没加载出来时只显示通用标题 */
export function CardHeaderBar({ card, columns, onColumn, onDelete, deleteBusy, onArchive, actionBusy }: CardHeaderBarProps) {
  const { t } = useTranslation()
  const { columnName } = useTemplateText()

  return (
    <div className="flex shrink-0 items-center gap-1.5 border-b border-border/60 py-3 pr-12 pl-4 sm:gap-3 sm:pr-16 sm:pl-6">
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
      {card && !card.archived_at && (
        <div className="ml-auto flex shrink-0 items-center gap-0.5">
          {!card.archived_at && (
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button type="button" variant="ghost" size="icon-sm" disabled={actionBusy} aria-label={t('card.archive.action')} title={t('card.archive.action')}>
                  <ArchiveIcon />
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>{t('card.archive.confirmTitle')}</AlertDialogTitle>
                  <AlertDialogDescription>{t('card.archive.confirmBody')}</AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
                  <AlertDialogAction onClick={onArchive}>{t('card.archive.action')}</AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          )}
          {!card.archived_at && (
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button type="button" variant="ghost" size="icon-sm" className="text-muted-foreground" aria-label={t('common.delete')}>
                  <Trash2Icon />
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>{t('card.deleteTitle')}</AlertDialogTitle>
                  <AlertDialogDescription>{t('card.deleteBody')}</AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
                  <AlertDialogAction disabled={deleteBusy} onClick={onDelete}>
                    {t('common.delete')}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          )}
        </div>
      )}
    </div>
  )
}
