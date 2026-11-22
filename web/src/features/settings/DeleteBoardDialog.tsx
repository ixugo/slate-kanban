// 删除项目：弹出模态框，必须原样输入项目名称才能确认，成功后回到项目列表。

import { Loader2Icon } from 'lucide-react'
import { useState, type FormEvent } from 'react'
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

/**
 * 删除项目确认框。
 * 删除会带走项目的全部列、任务、规则、评论和活动，没有回收站，所以用模态框拦住误点：
 * 输入的内容必须和项目名称完全一致（只忽略首尾空白，区分大小写）才放行；
 * 标题里的项目名加粗标红，并且一点就能全选，方便人复制后粘贴进输入框。
 */
export function DeleteBoardDialog({ board, open, onOpenChange }: Props) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const toastError = useErrorToast()
  const { deleteBoard } = useSettingsMutations(board.id)
  const [confirmName, setConfirmName] = useState('')

  const canConfirm = confirmName.trim() === board.name.trim()

  /** 提交表单（点按钮或在输入框里回车）：名称对上了才发删除请求，成功后提示并回到项目列表 */
  const onDelete = (e: FormEvent) => {
    e.preventDefault()
    if (!canConfirm || deleteBoard.isPending) return
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
        if (!v) setConfirmName('')
      }}
    >
      <AlertDialogContent>
        <form onSubmit={onDelete} className="grid gap-4">
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t('settings.general.deleteTitleBefore')}
              <strong className="font-bold break-all text-destructive select-all">{board.name}</strong>
              {t('settings.general.deleteTitleAfter')}
            </AlertDialogTitle>
            <AlertDialogDescription>{t('settings.general.deleteBody')}</AlertDialogDescription>
          </AlertDialogHeader>
          <div className="space-y-2">
            <Label htmlFor="confirm-name">{t('settings.general.confirmName')}</Label>
            <Input
              id="confirm-name"
              value={confirmName}
              onChange={(e) => setConfirmName(e.target.value)}
              autoComplete="off"
            />
          </div>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <Button type="submit" variant="destructive" disabled={!canConfirm || deleteBoard.isPending}>
              {deleteBoard.isPending && <Loader2Icon className="animate-spin" />}
              {t('common.delete')}
            </Button>
          </AlertDialogFooter>
        </form>
      </AlertDialogContent>
    </AlertDialog>
  )
}
