// 自动化规则的创建/编辑对话框：两句话描述触发与动作，高级项收在「更多选项」。

import { ChevronRightIcon, Loader2Icon, PlusIcon, Trash2Icon } from 'lucide-react'
import { useMemo, useState, type FormEvent } from 'react'
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
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import type { ActionType, Column, Rule } from '@/lib/api/types'
import { FILTER_MAX, RULE_ACTIONS_MAX, RULE_NAME_MAX } from '@/lib/limits'
import { useErrorToast } from '@/lib/useErrorToast'
import { ActionFields } from './ActionFields'
import { autoNameFromFormState } from './ruleAutoName'
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
import { shouldExpandMoreOptions } from './ruleEditorHelpers'
import { useSettingsMutations } from './useSettingsMutations'

const TIME_ACTIONS: ActionType[] = [
  'set_started_at',
  'set_finished_at',
  'clear_started_at',
  'clear_finished_at',
]

const CARD_ACTIONS: ActionType[] = [
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
  const [moreOpen, setMoreOpen] = useState(() => shouldExpandMoreOptions(rule, rule ? ruleToForm(rule) : emptyRuleForm(), columns, t))

  // 名称留空时用来生成"进入「进行中」时：记录开始时间"这样的一句话；列或语言变了才重新生成
  const resolveName = useMemo(
    () => (s: RuleFormState) =>
      autoNameFromFormState(s.trigger, s.columnId, s.actions, columns, t),
    [columns, t],
  )

  const busy = createRule.isPending || updateRule.isPending

  /** 提交：编辑走 PATCH，新建走 POST；名称留空时由 resolveName 自动生成一句话 */
  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (rule) {
      const { patch, errors: found } = formToUpdateInput(state, columns, resolveName)
      setErrors(found)
      if (Object.keys(found).length > 0) return
      // 乐观更新：规则先改好、对话框先关，失败时还原并提示
      updateRule.mutate({ ruleId: rule.id, input: patch })
      onOpenChange(false)
      return
    }
    const { input, errors: found } = formToCreateInput({ ...state, enabled: true }, columns, resolveName)
    setErrors(found)
    if (!input || Object.keys(found).length > 0) return
    createRule.mutate(input, { onSuccess: () => onOpenChange(false), onError: toastError })
  }

  /** 替换第 index 个动作 */
  const setAction = (index: number, row: ActionFormRow) => {
    setState((s) => ({ ...s, actions: s.actions.map((a, i) => (i === index ? row : a)) }))
  }

  /** 末尾追加一个空动作，最多 RULE_ACTIONS_MAX 个 */
  const addAction = () => {
    if (state.actions.length >= RULE_ACTIONS_MAX) return
    setState((s) => ({ ...s, actions: [...s.actions, emptyActionRow()] }))
  }

  /** 删除第 index 个动作；至少保留一个 */
  const removeAction = (index: number) => {
    if (state.actions.length <= 1) return
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
          <WhenRow state={state} columns={columns} errors={errors} onChange={setState} />
          <fieldset className="space-y-3">
            <legend className="sr-only">{t('settings.rules.actionsTitle')}</legend>
            {errors.actions && (
              <p role="alert" className="text-xs text-destructive">
                {t(`settings.rules.errors.${errors.actions}`)}
              </p>
            )}
            {state.actions.map((row, index) => (
              <ThenRow
                key={row.id}
                row={row}
                index={index}
                columns={columns}
                canRemove={state.actions.length > 1}
                onChange={(next) => setAction(index, next)}
                onRemove={() => removeAction(index)}
              />
            ))}
            <Button
              type="button"
              variant="link"
              size="sm"
              className="h-auto p-0"
              onClick={addAction}
              disabled={state.actions.length >= RULE_ACTIONS_MAX}
            >
              <PlusIcon />
              {t('settings.rules.editor.addAnotherAction')}
            </Button>
          </fieldset>
          <MoreOptionsSection open={moreOpen} onOpenChange={setMoreOpen} state={state} errors={errors} onChange={setState} />
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

/** 第一行：当 [触发] [列] */
function WhenRow({
  state,
  columns,
  errors,
  onChange,
}: {
  state: RuleFormState
  columns: Column[]
  errors: RuleFormErrors
  onChange: (s: RuleFormState) => void
}) {
  const { t } = useTranslation()
  return (
    <div className="flex flex-wrap items-center gap-2 text-sm">
      <span>{t('settings.rules.editor.when')}</span>
      <Select value={state.trigger} onValueChange={(v) => onChange({ ...state, trigger: v as RuleFormState['trigger'] })}>
        <SelectTrigger className="w-auto min-w-[8rem]" aria-label={t('settings.rules.trigger')}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="card_created">{t('settings.rules.triggers.card_created')}</SelectItem>
          <SelectItem value="card_entered">{t('settings.rules.triggers.card_entered')}</SelectItem>
          <SelectItem value="card_left">{t('settings.rules.triggers.card_left')}</SelectItem>
        </SelectContent>
      </Select>
      <Select value={String(state.columnId)} onValueChange={(v) => onChange({ ...state, columnId: Number(v) })}>
        <SelectTrigger className="w-auto min-w-[8rem]" aria-invalid={!!errors.columnId} aria-label={t('settings.rules.triggerColumn')}>
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
      {errors.columnId && (
        <p role="alert" className="w-full text-xs text-destructive">
          {t(`settings.rules.errors.${errors.columnId}`)}
        </p>
      )}
    </div>
  )
}

/** 第二行：就 [动作] + 参数 */
function ThenRow({
  row,
  index,
  columns,
  canRemove,
  onChange,
  onRemove,
}: {
  row: ActionFormRow
  index: number
  columns: Column[]
  canRemove: boolean
  onChange: (row: ActionFormRow) => void
  onRemove: () => void
}) {
  const { t } = useTranslation()
  const label = index === 0 ? t('settings.rules.editor.thenDo') : t('settings.rules.editor.andDo')
  return (
    <div className="space-y-2 rounded-2xl bg-secondary/50 p-3.5">
      <div className="flex flex-wrap items-start gap-2">
        <span className="pt-2 text-sm">{label}</span>
        <ActionTypeSelect row={row} onChange={onChange} />
        {canRemove && (
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            onClick={onRemove}
            aria-label={t('settings.rules.removeAction')}
          >
            <Trash2Icon />
          </Button>
        )}
      </div>
      {row.type !== 'set_due' && row.type !== 'clear_due' && (
        <ActionFields row={row} columns={columns} onChange={onChange} hideTimeMode />
      )}
    </div>
  )
}

/** 分组动作下拉 */
function ActionTypeSelect({ row, onChange }: { row: ActionFormRow; onChange: (row: ActionFormRow) => void }) {
  const { t } = useTranslation()
  return (
    <Select
      value={row.type}
      onValueChange={(v) => onChange({ ...emptyActionRow(v as ActionType), id: row.id })}
    >
      <SelectTrigger className="min-w-[10rem] flex-1">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectGroup>
          <SelectLabel>{t('settings.rules.actionGroups.time')}</SelectLabel>
          {(row.type === 'set_due' || row.type === 'clear_due') && (
            <SelectItem value={row.type} disabled>
              {t('settings.rules.inactiveAction', { name: t(`settings.rules.actionTypes.${row.type}`) })}
            </SelectItem>
          )}
          {TIME_ACTIONS.map((type) => (
            <SelectItem key={type} value={type}>
              {t(`settings.rules.actionTypes.${type}` as TranslationKey)}
            </SelectItem>
          ))}
        </SelectGroup>
        <SelectGroup>
          <SelectLabel>{t('settings.rules.actionGroups.card')}</SelectLabel>
          {CARD_ACTIONS.map((type) => (
            <SelectItem key={type} value={type}>
              {t(`settings.rules.actionTypes.${type}` as TranslationKey)}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}

/** 折叠区：自定义名称、筛选、时间写入模式 */
function MoreOptionsSection({
  open,
  onOpenChange,
  state,
  errors,
  onChange,
}: {
  open: boolean
  onOpenChange: (v: boolean) => void
  state: RuleFormState
  errors: RuleFormErrors
  onChange: (s: RuleFormState) => void
}) {
  const { t } = useTranslation()
  const timeRows = state.actions
    .map((row, index) => ({ row, index }))
    .filter((x) => x.row.type === 'set_started_at' || x.row.type === 'set_finished_at')
  return (
    <div className="border-t border-border pt-3">
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="h-auto p-0 text-muted-foreground"
        aria-expanded={open}
        onClick={() => onOpenChange(!open)}
      >
        <ChevronRightIcon className={`transition-transform duration-200 ease-spring ${open ? 'rotate-90' : ''}`} />
        {t('settings.rules.moreOptions')}
      </Button>
      {open && (
        <div className="mt-3 space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="rule-name">{t('settings.rules.name')}</Label>
            <Input
              id="rule-name"
              value={state.name}
              maxLength={RULE_NAME_MAX}
              onChange={(e) => onChange({ ...state, name: e.target.value })}
              aria-invalid={!!errors.name}
            />
            {errors.name && (
              <p role="alert" className="text-xs text-destructive">
                {t(`settings.rules.errors.${errors.name}`)}
              </p>
            )}
          </div>
          <TimeModeFields state={state} timeRows={timeRows} onChange={onChange} />
          <ConditionField state={state} onChange={onChange} errors={errors} />
        </div>
      )}
    </div>
  )
}

/** 更多选项里：为「记录开始/完成时间」动作选择只在为空时写入，还是总是覆盖 */
function TimeModeFields({
  state,
  timeRows,
  onChange,
}: {
  state: RuleFormState
  timeRows: { row: ActionFormRow; index: number }[]
  onChange: (s: RuleFormState) => void
}) {
  if (timeRows.length === 0) return null
  return (
    <div className="space-y-2">
      {timeRows.map(({ row, index }) => (
        <ActionFields
          key={row.id}
          row={row}
          columns={[]}
          onChange={(next) => {
            const actions = state.actions.map((a, i) => (i === index ? next : a))
            onChange({ ...state, actions })
          }}
        />
      ))}
    </div>
  )
}

/** 更多选项里的筛选条件输入框：只有满足条件的任务才会触发这条规则，留空表示所有任务 */
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