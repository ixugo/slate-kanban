// 新增或编辑一列。

import { Loader2Icon } from 'lucide-react'
import { useState, type FormEvent, type ReactNode } from 'react'
import { useTranslation } from '@/i18n/react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import type { Column } from '@/lib/api/types'
import { COLUMN_INSTRUCTION_MAX, COLUMN_NAME_MAX } from '@/lib/limits'
import { useErrorToast } from '@/lib/useErrorToast'
import { useTemplateText } from '@/lib/useTemplateText'
import { useSettingsMutations } from './useSettingsMutations'
import { validateColumnForm, type ColumnFormErrors, type ColumnFormValues } from './validation'

interface Props {
  boardId: number
  columns: Column[]
  column?: Column
  open: boolean
  onOpenChange: (open: boolean) => void
}

/** 列表单对话框 */
export function ColumnDialog({ boardId, columns, column, open, onOpenChange }: Props) {
  const { t } = useTranslation()
  const { columnName } = useTemplateText()
  const toastError = useErrorToast()
  const { createColumn, updateColumn } = useSettingsMutations(boardId)
  const [values, setValues] = useState<ColumnFormValues>(() =>
    column
      ? {
          name: column.name,
          instruction: column.instruction,
          isDone: column.is_done,
          nextColumnId: column.next_column_id,
        }
      : emptyValues(),
  )
  const [errors, setErrors] = useState<ColumnFormErrors>({})

  const others = columns.filter((c) => c.id !== (column?.id ?? 0))
  const busy = createColumn.isPending || updateColumn.isPending

  const submit = (e: FormEvent) => {
    e.preventDefault()
    const found = validateColumnForm(values, column?.id ?? 0, columns.length, !column)
    setErrors(found)
    if (Object.keys(found).length > 0) return
    const payload = {
      name: values.name.trim(),
      instruction: values.instruction.trim(),
      is_done: values.isDone,
      next_column_id: values.nextColumnId,
    }
    if (column) {
      // 乐观更新：列先改好、对话框先关，失败时还原并提示
      updateColumn.mutate({ columnId: column.id, input: payload })
      onOpenChange(false)
    } else {
      createColumn.mutate(payload, { onSuccess: () => onOpenChange(false), onError: toastError })
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent key={column?.id ?? 'new'} className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{column ? t('settings.columns.editTitle') : t('settings.columns.createTitle')}</DialogTitle>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <Field label={t('settings.columns.name')} error={errors.name}>
            <Input value={values.name} maxLength={COLUMN_NAME_MAX} onChange={(e) => setValues({ ...values, name: e.target.value })} />
          </Field>
          <Field label={t('settings.columns.instruction')}>
            <Textarea value={values.instruction} maxLength={COLUMN_INSTRUCTION_MAX} rows={4} onChange={(e) => setValues({ ...values, instruction: e.target.value })} />
          </Field>
          <ToggleRow
            id="col-done"
            label={t('settings.columns.isDone')}
            checked={values.isDone}
            onCheckedChange={(v) => setValues({ ...values, isDone: v })}
          />
          <Field label={t('settings.columns.nextColumn')} error={errors.nextColumnId}>
            <Select
              value={String(values.nextColumnId)}
              onValueChange={(v) => setValues({ ...values, nextColumnId: Number(v) })}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="0">{t('common.none')}</SelectItem>
                {others.map((c) => (
                  <SelectItem key={c.id} value={String(c.id)}>
                    {columnName(c.name)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
          {(errors.name || errors.nextColumnId) && (
            <p role="alert" className="text-xs text-destructive">
              {t(
                `settings.columns.errors.${(errors.name ?? errors.nextColumnId) as 'nameRequired' | 'nextSelf' | 'limitReached'}`,
              )}
            </p>
          )}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" disabled={busy}>
              {busy && <Loader2Icon className="animate-spin" />}
              {column ? t('common.save') : t('common.create')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function emptyValues(): ColumnFormValues {
  return { name: '', instruction: '', isDone: false, nextColumnId: 0 }
}

function Field({
  label,
  children,
  hint,
  error,
}: {
  label: string
  children: ReactNode
  hint?: string
  error?: string
}) {
  return (
    <div className="space-y-1.5">
      <Label>{label}</Label>
      {children}
      {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
      {error && (
        <p role="alert" className="text-xs text-destructive">
          {error}
        </p>
      )}
    </div>
  )
}

function ToggleRow({
  id,
  label,
  hint,
  checked,
  onCheckedChange,
}: {
  id: string
  label: string
  hint?: string
  checked: boolean
  onCheckedChange: (v: boolean) => void
}) {
  return (
    <div className="flex items-start justify-between gap-3">
      <div>
        <Label htmlFor={id}>{label}</Label>
        {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
      </div>
      <Switch id={id} checked={checked} onCheckedChange={onCheckedChange} />
    </div>
  )
}