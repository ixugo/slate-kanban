// 自动化规则的创建/编辑对话框。

import { Loader2Icon, PlusIcon, Trash2Icon } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from '@/i18n/react'
import type { TranslationKey } from '@/i18n/types'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
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
import type { ActionType, Column, Rule } from '@/lib/api/types'
import { FILTER_MAX, RULE_ACTIONS_MAX, RULE_NAME_MAX } from '@/lib/limits'
import { useErrorToast } from '@/lib/useErrorToast'
import { ActionFields } from './ActionFields'
import {
  emptyActionRow,
  emptyRuleForm,
  formToCreateInput,
  formToUpdateInput,
  ruleToForm,
  type ActionFormRow,
  type RuleFormErrors,
  type RuleFormState,
} from './ruleForm'
import { RULE_PRESETS, buildRulePreset, type RulePresetId } from './rulePresets'
import { useSettingsMutations } from './useSettingsMutations'

const ACTION_TYPES: ActionType[] = [
  'set_started_at',
  'set_finished_at',
  'clear_started_at',
  'clear_finished_at',
  'set_due',
  'clear_due',
  'set_priority',
  'add_tag',
  'remove_tag',
  'set_human_only',
  'add_comment',
  'move_to_column',
]

interface Props {
  boardId: number
  columns: Column[]
  rule?: Rule
  open: boolean
  onOpenChange: (open: boolean) => void
}

/** 规则编辑对话框 */
export function RuleEditorDialog({ boardId, columns, rule, open, onOpenChange }: Props) {
  const { t } = useTranslation()
  const toastError = useErrorToast()
  const { createRule, updateRule } = useSettingsMutations(boardId)
  const [state, setState] = useState<RuleFormState>(() => (rule ? ruleToForm(rule) : emptyRuleForm()))
  const [errors, setErrors] = useState<RuleFormErrors>({})

  const busy = createRule.isPending || updateRule.isPending

  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (rule) {
      const { patch, errors: found } = formToUpdateInput(state, columns)
      setErrors(found)
      if (Object.keys(found).length > 0) return
      updateRule.mutate(
        { ruleId: rule.id, input: patch },
        { onSuccess: () => onOpenChange(false), onError: toastError },
      )
      return
    }
    const { input, errors: found } = formToCreateInput(state, columns)
    setErrors(found)
    if (!input || Object.keys(found).length > 0) return
    createRule.mutate(input, { onSuccess: () => onOpenChange(false), onError: toastError })
  }

  const applyPreset = (id: RulePresetId) => {
    setState(buildRulePreset(id, columns))
    setErrors({})
  }

  const setAction = (index: number, row: ActionFormRow) => {
    setState((s) => ({ ...s, actions: s.actions.map((a, i) => (i === index ? row : a)) }))
  }

  const addAction = () => {
    if (state.actions.length >= RULE_ACTIONS_MAX) return
    setState((s) => ({ ...s, actions: [...s.actions, emptyActionRow()] }))
  }

  const removeAction = (index: number) => {
    setState((s) => ({ ...s, actions: s.actions.filter((_, i) => i !== index) }))
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent key={rule?.id ?? 'new'} className="max-h-[90vh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{rule ? t('settings.rules.editTitle') : t('settings.rules.createTitle')}</DialogTitle>
          <DialogDescription>{t('settings.rules.editDesc')}</DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="rule-name">{t('settings.rules.name')}</Label>
            <Input
              id="rule-name"
              value={state.name}
              maxLength={RULE_NAME_MAX}
              onChange={(e) => setState({ ...state, name: e.target.value })}
              aria-invalid={!!errors.name}
            />
            {errors.name && (
              <p role="alert" className="text-xs text-destructive">
                {t(`settings.rules.errors.${errors.name}`)}
              </p>
            )}
          </div>
          <div className="flex items-center justify-between gap-3">
            <Label htmlFor="rule-enabled">{t('settings.rules.enabled')}</Label>
            <Switch
              id="rule-enabled"
              checked={state.enabled}
              onCheckedChange={(v) => setState({ ...state, enabled: v })}
            />
          </div>
          <TriggerFields state={state} columns={columns} onChange={setState} errors={errors} />
          <ConditionField state={state} onChange={setState} errors={errors} />
          <fieldset className="space-y-2">
            <legend className="text-sm font-medium">{t('settings.rules.actionsTitle')}</legend>
            {errors.actions && (
              <p role="alert" className="text-xs text-destructive">
                {t(`settings.rules.errors.${errors.actions}`)}
              </p>
            )}
            {state.actions.map((row, index) => (
              <div key={row.id} className="space-y-2 border border-border p-3">
                <div className="flex items-center gap-2">
                  <Select
                    value={row.type}
                    onValueChange={(v) => setAction(index, { ...emptyActionRow(v as ActionType), id: row.id })}
                  >
                    <SelectTrigger className="flex-1">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {ACTION_TYPES.map((type) => (
                        <SelectItem key={type} value={type}>
                          {t(`settings.rules.actionTypes.${type}`)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    disabled={state.actions.length <= 1}
                    onClick={() => removeAction(index)}
                    aria-label={t('settings.rules.removeAction')}
                  >
                    <Trash2Icon />
                  </Button>
                </div>
                <ActionFields row={row} columns={columns} onChange={(next) => setAction(index, next)} />
              </div>
            ))}
            <Button type="button" variant="outline" size="sm" onClick={addAction} disabled={state.actions.length >= RULE_ACTIONS_MAX}>
              <PlusIcon />
              {t('settings.rules.addAction')}
            </Button>
          </fieldset>
          <PresetsRow onPick={applyPreset} />
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" disabled={busy}>
              {busy && <Loader2Icon className="animate-spin" />}
              {rule ? t('common.save') : t('common.create')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function TriggerFields({
  state,
  columns,
  onChange,
  errors,
}: {
  state: RuleFormState
  columns: Column[]
  onChange: (s: RuleFormState) => void
  errors: RuleFormErrors
}) {
  const { t } = useTranslation()
  return (
    <div className="grid gap-3 sm:grid-cols-2">
      <div className="space-y-1.5">
        <Label>{t('settings.rules.trigger')}</Label>
        <Select value={state.trigger} onValueChange={(v) => onChange({ ...state, trigger: v as RuleFormState['trigger'] })}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="card_created">{t('settings.rules.triggers.card_created')}</SelectItem>
            <SelectItem value="card_entered">{t('settings.rules.triggers.card_entered')}</SelectItem>
            <SelectItem value="card_left">{t('settings.rules.triggers.card_left')}</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-1.5">
        <Label>{t('settings.rules.triggerColumn')}</Label>
        <Select
          value={String(state.columnId)}
          onValueChange={(v) => onChange({ ...state, columnId: Number(v) })}
        >
          <SelectTrigger aria-invalid={!!errors.columnId}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="0">{t('settings.rules.anyColumn')}</SelectItem>
            {columns.map((c) => (
              <SelectItem key={c.id} value={String(c.id)}>
                {c.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    </div>
  )
}

function ConditionField({
  state,
  onChange,
  errors,
}: {
  state: RuleFormState
  onChange: (s: RuleFormState) => void
  errors: RuleFormErrors
}) {
  const { t } = useTranslation()
  return (
    <div className="space-y-1.5">
      <Label htmlFor="rule-condition">
        {t('settings.rules.condition')}{' '}
        <span className="font-normal text-muted-foreground">· {t('common.optional')}</span>
      </Label>
      <Textarea
        id="rule-condition"
        value={state.condition}
        maxLength={FILTER_MAX}
        rows={2}
        placeholder={t('settings.rules.conditionPlaceholder')}
        onChange={(e) => onChange({ ...state, condition: e.target.value })}
        aria-invalid={!!errors.condition}
        aria-describedby="rule-condition-hint"
      />
      <p id="rule-condition-hint" className="text-xs text-muted-foreground">
        {t('settings.rules.conditionHint')}
      </p>
      {errors.condition && (
        <p role="alert" className="text-xs text-destructive">
          {t(`settings.rules.errors.${errors.condition}`)}
        </p>
      )}
    </div>
  )
}

/** 预设按钮行 */
function PresetsRow({ onPick }: { onPick: (id: RulePresetId) => void }) {
  const { t } = useTranslation()
  return (
    <div className="space-y-2 border-t pt-3">
      <p className="text-sm font-medium">{t('settings.rules.presetsTitle')}</p>
      <div className="flex flex-wrap gap-2">
        {RULE_PRESETS.map((p) => (
          <Button key={p.id} type="button" variant="secondary" size="sm" onClick={() => onPick(p.id)}>
            {t(p.labelKey as TranslationKey)}
          </Button>
        ))}
      </div>
    </div>
  )
}