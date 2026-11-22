// 删除列：列里有任务时必须选 move_to。

import { Loader2Icon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from '@/i18n/react'
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import type { CardView, Column } from '@/lib/api/types'
import { toast } from '@/lib/toast'
import { useErrorToast } from '@/lib/useErrorToast'
import { useTemplateText } from '@/lib/useTemplateText'
import { useSettingsMutations } from './useSettingsMutations'
import { countCardsInColumn } from './validation'

interface Props {
  boardId: number
  column: Column
  columns: Column[]
  cards: CardView[]
  open: boolean
  onOpenChange: (open: boolean) => void
}

/** 删除列确认 */
export function DeleteColumnDialog({ boardId, column, columns, cards, open, onOpenChange }: Props) {
  const { columnName } = useTemplateText()
  const { t } = useTranslation()
  const toastError = useErrorToast()
  const { deleteColumn } = useSettingsMutations(boardId)
  const cardCount = countCardsInColumn(column.id, cards)
  const needMove = cardCount > 0
  const targets = columns.filter((c) => c.id !== column.id)
  const [moveTo, setMoveTo] = useState<number>(targets[0]?.id ?? 0)

  const canConfirm = !needMove || moveTo !== 0

  const onDelete = () => {
    if (!canConfirm) return
    deleteColumn.mutate(
      { columnId: column.id, moveTo: needMove ? moveTo : undefined },
      {
        onSuccess: () => {
          toast.success(t('settings.columns.deletedToast'))
          onOpenChange(false)
        },
        onError: toastError,
      },
    )
  }

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('settings.columns.deleteTitle', { name: columnName(column.name) })}</AlertDialogTitle>
          <AlertDialogDescription>
            {needMove ? t('settings.columns.deleteBodyMove', { count: cardCount }) : t('settings.columns.deleteBodyEmpty')}
          </AlertDialogDescription>
        </AlertDialogHeader>
        {needMove && (
          <div className="space-y-1.5">
            <Label>{t('settings.columns.moveTo')}</Label>
            <Select value={String(moveTo)} onValueChange={(v) => setMoveTo(Number(v))}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {targets.map((c) => (
                  <SelectItem key={c.id} value={String(c.id)}>
                    {columnName(c.name)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        )}
        <AlertDialogFooter>
          <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
          <Button variant="destructive" disabled={!canConfirm || deleteColumn.isPending} onClick={onDelete}>
            {deleteColumn.isPending && <Loader2Icon className="animate-spin" />}
            {t('common.delete')}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}