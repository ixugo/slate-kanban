// 设置页「常规」：名称、说明、项目路径、只读 slug、删除看板。

import { Loader2Icon } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from '@/i18n/react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import type { Board } from '@/lib/api/types'
import { BOARD_DESC_MAX, BOARD_NAME_MAX, PROJECT_PATH_MAX } from '@/lib/limits'
import { useErrorToast } from '@/lib/useErrorToast'
import { DeleteBoardDialog } from './DeleteBoardDialog'
import { useSettingsMutations } from './useSettingsMutations'
import { validateGeneralForm, type GeneralFormErrors } from './validation'

interface Props {
  board: Board
}

/** 常规设置表单 */
export function GeneralSection({ board }: Props) {
  const { t } = useTranslation()
  const toastError = useErrorToast()
  const { updateBoard } = useSettingsMutations(board.id)
  const [name, setName] = useState(board.name)
  const [description, setDescription] = useState(board.description)
  const [projectPath, setProjectPath] = useState(board.project_path)
  const [errors, setErrors] = useState<GeneralFormErrors>({})
  const [deleteOpen, setDeleteOpen] = useState(false)

  const submit = (e: FormEvent) => {
    e.preventDefault()
    const found = validateGeneralForm({ name, description, projectPath })
    setErrors(found)
    if (Object.keys(found).length > 0) return
    updateBoard.mutate(
      {
        name: name.trim(),
        description: description.trim(),
        project_path: projectPath.trim(),
      },
      { onError: toastError },
    )
  }

  const fieldError = (key: keyof GeneralFormErrors, id: string) =>
    errors[key] ? (
      <p id={id} role="alert" className="text-xs text-destructive">
        {t(`settings.general.errors.${errors[key]}`)}
      </p>
    ) : null

  return (
    <div>
      <form onSubmit={submit} className="max-w-xl space-y-4 rounded-2xl bg-card p-5 shadow-card">
        <div className="space-y-1.5">
          <Label htmlFor="board-name">{t('settings.general.name')}</Label>
          <Input
            id="board-name"
            value={name}
            maxLength={BOARD_NAME_MAX}
            onChange={(e) => setName(e.target.value)}
            aria-invalid={!!errors.name}
            aria-describedby={errors.name ? 'board-name-error' : undefined}
          />
          {fieldError('name', 'board-name-error')}
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="board-desc">{t('settings.general.description')}</Label>
          <Textarea
            id="board-desc"
            value={description}
            maxLength={BOARD_DESC_MAX}
            rows={3}
            onChange={(e) => setDescription(e.target.value)}
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="board-path">{t('settings.general.projectPath')}</Label>
          <Input
            id="board-path"
            value={projectPath}
            maxLength={PROJECT_PATH_MAX}
            onChange={(e) => setProjectPath(e.target.value)}
            aria-invalid={!!errors.projectPath}
            aria-describedby="board-path-hint"
          />
          <p id="board-path-hint" className="text-xs text-muted-foreground">
            {t('settings.general.projectPathHint')}
          </p>
          {fieldError('projectPath', 'board-path-error')}
        </div>
        <div className="space-y-1.5">
          <Label>{t('settings.general.slug')}</Label>
          <p className="font-mono text-ui text-muted-foreground">{board.slug}</p>
          <p className="text-xs text-muted-foreground">{t('settings.general.slugHint')}</p>
        </div>
        <Button type="submit" disabled={updateBoard.isPending}>
          {updateBoard.isPending && <Loader2Icon className="animate-spin" />}
          {t('common.save')}
        </Button>
      </form>
      <section className="mt-8 max-w-xl space-y-3 rounded-2xl bg-destructive/10 p-5">
        <h3 className="text-body font-semibold tracking-tight text-destructive">{t('settings.general.dangerTitle')}</h3>
        <p className="text-ui text-muted-foreground">{t('settings.general.dangerBody')}</p>
        <Button type="button" variant="destructive" onClick={() => setDeleteOpen(true)}>
          {t('settings.general.deleteBoard')}
        </Button>
      </section>
      <DeleteBoardDialog board={board} open={deleteOpen} onOpenChange={setDeleteOpen} />
    </div>
  )
}