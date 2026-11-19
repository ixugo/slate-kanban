// 卡片弹窗顶部：所在列（下拉，选了就移动卡片）、卡片编号、删除按钮。对齐 Trello 卡片顶部的「列名按钮」。

import { Trash2Icon } from 'lucide-react'
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
}

/** 弹窗顶栏；卡片还没加载出来时只显示通用标题 */
export function CardHeaderBar({ card, columns, onColumn, onDelete, deleteBusy }: CardHeaderBarProps) {
  const { t } = useTranslation()
  const { columnName } = useTemplateText()

  return (
    <div className="flex shrink-0 items-center gap-3 border-b border-border/60 py-3 pr-16 pl-5 sm:pl-6">
      {card && (
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
      {card && (
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button type="button" variant="ghost" size="icon-sm" className="ml-auto text-muted-foreground" aria-label={t('common.delete')}>
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
  )
}
