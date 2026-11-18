// 删除看板：必须输入看板 slug 才能确认，成功后回到看板列表。

import { Loader2Icon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from '@/i18n/react'
import { useNavigate } from 'react-router-dom'
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
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { Board } from '@/lib/api/types'
import { toast } from '@/lib/toast'
import { useErrorToast } from '@/lib/useErrorToast'
import { useSettingsMutations } from './useSettingsMutations'

interface Props {
  board: Board
  open: boolean
  onOpenChange: (open: boolean) => void
}

/** 删除看板确认框 */
export function DeleteBoardDialog({ board, open, onOpenChange }: Props) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const toastError = useErrorToast()
  const { deleteBoard } = useSettingsMutations(board.id)
  const [confirmSlug, setConfirmSlug] = useState('')

  const canConfirm = confirmSlug.trim().toUpperCase() === board.slug.toUpperCase()

  const onDelete = () => {
    if (!canConfirm) return
    deleteBoard.mutate(undefined, {
      onSuccess: () => {
        toast.success(t('settings.general.deletedToast'))
        navigate('/')
      },
      onError: toastError,
    })
  }

  return (
    <AlertDialog
      open={open}
      onOpenChange={(v) => {
        onOpenChange(v)
        if (!v) setConfirmSlug('')
      }}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('settings.general.deleteTitle')}</AlertDialogTitle>
          <AlertDialogDescription>{t('settings.general.deleteBody')}</AlertDialogDescription>
        </AlertDialogHeader>
        <div className="space-y-1.5">
          <Label htmlFor="confirm-slug">{t('settings.general.confirmSlug', { slug: board.slug })}</Label>
          <Input
            id="confirm-slug"
            value={confirmSlug}
            onChange={(e) => setConfirmSlug(e.target.value)}
            autoComplete="off"
          />
        </div>
        <AlertDialogFooter>
          <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
          <Button variant="destructive" disabled={!canConfirm || deleteBoard.isPending} onClick={onDelete}>
            {deleteBoard.isPending && <Loader2Icon className="animate-spin" />}
            {t('common.delete')}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}