// 设置页「常规」：名称、AGENTS.md、只读 slug、删除项目。

import { Loader2Icon } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from '@/i18n/react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { LazyEditor, type LiveEditorDraft } from '@/features/editor/LazyEditor'
import type { Board } from '@/lib/api/types'
import { BOARD_DESC_MAX, BOARD_NAME_MAX } from '@/lib/limits'
import { DeleteBoardDialog } from './DeleteBoardDialog'
import { useSettingsMutations } from './useSettingsMutations'
import { validateGeneralForm, type GeneralFormErrors } from './validation'

interface Props {
  board: Board
}

/** 常规设置表单 */
export function GeneralSection({ board }: Props) {
  const { t } = useTranslation()
  const { updateBoard } = useSettingsMutations(board.id)
  const [name, setName] = useState(board.name)
  const [description, setDescription] = useState<LiveEditorDraft>({ value: board.description })
  const [errors, setErrors] = useState<GeneralFormErrors>({})
  const [deleteOpen, setDeleteOpen] = useState(false)

  const submit = (e: FormEvent) => {
    e.preventDefault()
    const found = validateGeneralForm({ name, description: description.value })
    setErrors(found)
    if (Object.keys(found).length > 0) return
    updateBoard.mutate({
      name: name.trim(),
      description: description.value,
    })
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
          <p className="text-ui font-medium">{t('settings.general.description')}</p>
          <LazyEditor
            label={t('settings.general.description')}
            {...description}
            max={BOARD_DESC_MAX}
            placeholder={t('boards.create.descriptionPlaceholder')}
            onChange={setDescription}
            compact
          />
        </div>
        <div className="space-y-1.5">
          <Label>{t('settings.general.slug')}</Label>
          <p className="font-mono text-ui text-muted-foreground">{board.slug}</p>
        </div>
        <Button type="submit" disabled={updateBoard.isPending}>
          {updateBoard.isPending && <Loader2Icon className="animate-spin" />}
          {t('common.save')}
        </Button>
      </form>
      <section className="mt-8 max-w-xl space-y-3 rounded-2xl bg-destructive/10 p-5">
        <p className="text-ui text-muted-foreground">{t('settings.general.dangerBody')}</p>
        <Button type="button" variant="destructive" onClick={() => setDeleteOpen(true)}>
          {t('settings.general.deleteBoard')}
        </Button>
      </section>
      <DeleteBoardDialog board={board} open={deleteOpen} onOpenChange={setDeleteOpen} />
    </div>
  )
}
