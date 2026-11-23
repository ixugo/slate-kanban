// 设置页「通知」：活动 webhook 列表与编辑。

import { PencilIcon, PlusIcon, SendIcon, Trash2Icon } from 'lucide-react'
import type { ReactNode } from 'react'
import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation, type TFunction } from '@/i18n/react'
import type { TranslationKey } from '@/i18n/types'
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
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import type { Column, CreateWebhookInput, UpdateWebhookInput, WebhookFilterModes, WebhookType, WebhookView } from '@/lib/api/types'
import {
  createWebhook,
  deleteWebhook,
  listWebhooks,
  testWebhook,
  updateWebhook,
} from '@/lib/api/endpoints'
import { applyOptimistic, rollbackOptimistic } from '@/lib/optimistic'
import { queryKeys } from '@/lib/queryKeys'
import { toast } from '@/lib/toast'
import { useErrorToast } from '@/lib/useErrorToast'
import { validateWebhookForm, type WebhookFormValues } from './validation'

const EVENT_TYPES = [
  'board.created',
  'board.updated',
  'card.created',
  'card.updated',
  'card.moved',
  'card.deleted',
  'comment.added',
  'rule.created',
  'automation.fired',
] as const

const EVENT_LABEL_KEYS: Record<(typeof EVENT_TYPES)[number], TranslationKey> = {
  'board.created': 'settings.webhooks.events.board_created',
  'board.updated': 'settings.webhooks.events.board_updated',
  'card.created': 'settings.webhooks.events.card_created',
  'card.updated': 'settings.webhooks.events.card_updated',
  'card.moved': 'settings.webhooks.events.card_moved',
  'card.deleted': 'settings.webhooks.events.card_deleted',
  'comment.added': 'settings.webhooks.events.comment_added',
  'rule.created': 'settings.webhooks.events.rule_created',
  'automation.fired': 'settings.webhooks.events.automation_fired',
}

const ACTOR_KINDS = ['human', 'agent', 'system'] as const
// 保留老看板的默认收件箱名，以便升级后新建 Webhook 时仍排除此列。
const INBOX_NAMES = new Set(['收件箱', '收件匣', 'Inbox', '受信トレイ', '받은 편지함'])

interface Props {
  boardId: number
  columns: Column[]
}

/** 活动 webhook 配置区块 */
export function WebhooksSection({ boardId, columns }: Props) {
  const { t } = useTranslation()
  const toastError = useErrorToast()
  const queryClient = useQueryClient()
  const key = queryKeys.webhooks(boardId)
  const { data: webhooks = [], isPending } = useQuery({
    queryKey: key,
    queryFn: ({ signal }) => listWebhooks(boardId, signal),
  })
  const invalidate = () => void queryClient.invalidateQueries({ queryKey: key })

  const [editing, setEditing] = useState<WebhookView | 'new' | null>(null)
  const [deleting, setDeleting] = useState<WebhookView | null>(null)

  // 开关和删除都先改列表再请求：失败时列表还原并提示
  const toggle = useMutation({
    mutationFn: ({ id, enabled }: { id: number; enabled: boolean }) => updateWebhook(id, { enabled }),
    onMutate: ({ id, enabled }) =>
      applyOptimistic<WebhookView[]>(queryClient, key, (old) => old.map((w) => (w.id === id ? { ...w, enabled } : w))),
    onError: (err, _v, previous) => {
      rollbackOptimistic(queryClient, key, previous)
      toastError(err)
    },
    onSettled: invalidate,
  })
  const remove = useMutation({
    mutationFn: (id: number) => deleteWebhook(id),
    onMutate: (id) => {
      setDeleting(null)
      return applyOptimistic<WebhookView[]>(queryClient, key, (old) => old.filter((w) => w.id !== id))
    },
    onSuccess: () => toast.success(t('settings.webhooks.deletedToast')),
    onError: (err, _id, previous) => {
      rollbackOptimistic(queryClient, key, previous)
      toastError(err)
    },
    onSettled: invalidate,
  })
  const sendTest = useMutation({
    mutationFn: (id: number) => testWebhook(id),
    onSuccess: (res) => {
      if (res.ok) toast.success(t('settings.webhooks.testOk'))
      else toast.error(res.message)
    },
    onError: toastError,
  })

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-sm text-muted-foreground">{t('settings.webhooks.subtitle')}</p>
        <Button size="sm" onClick={() => setEditing('new')}>
          <PlusIcon />
          {t('settings.webhooks.add')}
        </Button>
      </div>
      {isPending ? (
        <p className="text-sm text-muted-foreground">{t('common.loading')}</p>
      ) : webhooks.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t('settings.webhooks.empty')}</p>
      ) : (
        <ul className="divide-y divide-border overflow-hidden rounded-2xl bg-card shadow-card">
          {webhooks.map((wh) => (
            <li key={wh.id} className="space-y-1 px-4 py-3.5">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <WebhookTitle name={wh.name} type={wh.type} t={t} />
                <div className="flex items-center gap-2">
                  <Switch
                    checked={wh.enabled}
                    aria-label={`${t('settings.webhooks.enabled')}: ${wh.name}`}
                    onCheckedChange={(v) => toggle.mutate({ id: wh.id, enabled: v })}
                  />
                  <Button
                    type="button"
                    size="icon-sm"
                    variant="ghost"
                    aria-label={t('settings.webhooks.test')}
                    disabled={sendTest.isPending}
                    onClick={() => sendTest.mutate(wh.id)}
                  >
                    <SendIcon />
                  </Button>
                  <Button type="button" size="icon-sm" variant="ghost" aria-label={t('common.edit')} onClick={() => setEditing(wh)}>
                    <PencilIcon />
                  </Button>
                  <Button
                    type="button"
                    size="icon-sm"
                    variant="ghost"
                    aria-label={t('common.delete')}
                    onClick={() => setDeleting(wh)}
                  >
                    <Trash2Icon />
                  </Button>
                </div>
              </div>
              <p className="truncate text-xs text-muted-foreground">{wh.url}</p>
            </li>
          ))}
        </ul>
      )}
      {editing !== null && (
        <WebhookDialog
          boardId={boardId}
          columns={columns}
          initial={editing === 'new' ? null : editing}
          onClose={() => setEditing(null)}
          onSaved={() => {
            invalidate()
            setEditing(null)
          }}
        />
      )}
      <AlertDialog open={deleting !== null} onOpenChange={(open) => !open && setDeleting(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('settings.webhooks.deleteTitle', { name: deleting?.name ?? '' })}</AlertDialogTitle>
            <AlertDialogDescription>{t('settings.webhooks.deleteBody')}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <Button
              variant="destructive"
              disabled={remove.isPending}
              onClick={() => deleting && remove.mutate(deleting.id)}
            >
              {t('common.delete')}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

function WebhookTitle({ name, type, t }: { name: string; type: WebhookType; t: TFunction }) {
  return (
    <span className="text-ui font-medium">
      {name}
      <span className="ml-2 text-xs font-normal text-muted-foreground">{t(`settings.webhooks.types.${type}`)}</span>
    </span>
  )
}

interface DialogProps {
  boardId: number
  columns: Column[]
  initial: WebhookView | null
  onClose: () => void
  onSaved: () => void
}

/** 新增或编辑 webhook 的对话框 */
function WebhookDialog({ boardId, columns, initial, onClose, onSaved }: DialogProps) {
  const { t } = useTranslation()
  const toastError = useErrorToast()
  const isNew = initial === null
  const [original] = useState(() => formFromWebhook(initial, columns))
  const [values, setValues] = useState(original)
  const [errors, setErrors] = useState<ReturnType<typeof validateWebhookForm>>({})

  const save = useMutation({
    mutationFn: async () => {
      const body: CreateWebhookInput & Pick<UpdateWebhookInput, 'secret_clear'> = {
        name: values.name.trim(),
        type: values.type,
        url: values.url.trim(),
        enabled: values.enabled,
      }
      const modes: WebhookFilterModes = {}
      if (isNew || !sameSelection(original.eventTypes, values.eventTypes)) {
        modes.event_types = true
        body.event_types = values.eventTypes
      }
      if (isNew || !sameSelection(original.actorKinds, values.actorKinds)) {
        modes.actor_kinds = true
        body.actor_kinds = values.actorKinds
      }
      if (isNew || !sameSelection(original.moveColumnIds, values.moveColumnIds)) {
        modes.move_column_ids = true
        body.move_column_ids = values.moveColumnIds
      }
      if (Object.keys(modes).length) body.strict_filter_modes = modes
      if (values.secret.trim() !== '') body.secret = values.secret.trim()
      if (values.secretClear) body.secret_clear = true
      if (isNew) return createWebhook(boardId, body)
      return updateWebhook(initial!.id, body)
    },
    onSuccess: () => {
      toast.success(isNew ? t('settings.webhooks.createdToast') : t('settings.webhooks.savedToast'))
      onSaved()
    },
    onError: toastError,
  })

  const changeSubscriptions = (patch: Partial<Pick<WebhookFormValues, 'eventTypes' | 'actorKinds' | 'moveColumnIds'>>) => {
    setValues((previous) => ({ ...previous, ...patch }))
  }

  const submit = () => {
    const next = validateWebhookForm(values, values.type)
    setErrors(next)
    if (Object.keys(next).length > 0) return
    save.mutate()
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{isNew ? t('settings.webhooks.addTitle') : t('settings.webhooks.editTitle')}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <Field label={t('settings.webhooks.name')} error={errors.name && t(`settings.webhooks.errors.${errors.name}`)}>
            <Input value={values.name} onChange={(e) => setValues({ ...values, name: e.target.value })} />
          </Field>
          <Field label={t('settings.webhooks.type')}>
            <select
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
              value={values.type}
              onChange={(e) => setValues({ ...values, type: e.target.value as WebhookType })}
            >
              {(['dingtalk', 'wecom', 'feishu'] as const).map((typ) => (
                <option key={typ} value={typ}>
                  {t(`settings.webhooks.types.${typ}`)}
                </option>
              ))}
            </select>
          </Field>
          <Field label={t('settings.webhooks.url')} error={errors.url && t(`settings.webhooks.errors.${errors.url}`)}>
            <Input value={values.url} onChange={(e) => setValues({ ...values, url: e.target.value })} />
          </Field>
          {values.type !== 'wecom' && (
            <Field label={t('settings.webhooks.secret')} hint={t('settings.webhooks.secretHint')}>
              <Input
                type="password"
                autoComplete="off"
                placeholder={initial?.secret_set ? t('settings.webhooks.secretKeep') : ''}
                value={values.secret}
                onChange={(e) => setValues({ ...values, secret: e.target.value, secretClear: false })}
              />
              {initial?.secret_set && (
                <label className="mt-2 flex items-center gap-2 text-xs text-muted-foreground">
                  <input
                    type="checkbox"
                    checked={values.secretClear}
                    onChange={(e) => setValues({ ...values, secretClear: e.target.checked })}
                  />
                  {t('settings.webhooks.secretClear')}
                </label>
              )}
            </Field>
          )}
          <Field label={t('settings.webhooks.eventTypes')}>
            <MultiCheck
              options={EVENT_TYPES.map((ev) => ({ value: ev, label: t(EVENT_LABEL_KEYS[ev]) }))}
              selected={values.eventTypes}
              onChange={(eventTypes) => changeSubscriptions({ eventTypes })}
            />
          </Field>
          <Field label={t('settings.webhooks.actorKinds')}>
            <MultiCheck
              options={ACTOR_KINDS.map((k) => ({
                value: k,
                label: t(`settings.webhooks.actor.${k}`),
              }))}
              selected={values.actorKinds}
              onChange={(actorKinds) => changeSubscriptions({ actorKinds })}
            />
          </Field>
          <Field label={t('settings.webhooks.moveColumns')}>
            <MultiCheck
              options={columns.map((c) => ({ value: String(c.id), label: c.name }))}
              selected={values.moveColumnIds.map(String)}
              onChange={(ids) => changeSubscriptions({ moveColumnIds: ids.map(Number) })}
            />
          </Field>
          <label className="flex items-center gap-2 text-sm">
            <Switch checked={values.enabled} onCheckedChange={(enabled) => setValues({ ...values, enabled })} />
            {t('settings.webhooks.enabled')}
          </label>
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose}>
            {t('common.cancel')}
          </Button>
          <Button type="button" disabled={save.isPending} onClick={submit}>
            {t('common.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function formFromWebhook(initial: WebhookView | null, columns: Column[]): WebhookFormValues {
  if (!initial) {
    return {
      name: '',
      type: 'wecom',
      url: '',
      secret: '',
      secretClear: false,
      enabled: true,
      eventTypes: [...EVENT_TYPES],
      actorKinds: ['agent'],
      moveColumnIds: columns.filter((column) => !INBOX_NAMES.has(column.name)).map((column) => column.id),
    }
  }
  return {
    name: initial.name,
    type: initial.type,
    url: initial.url,
    secret: '',
    secretClear: false,
    enabled: initial.enabled,
    eventTypes: selection(initial.event_types, [...EVENT_TYPES], initial.strict_filter_modes?.event_types ?? initial.strict_filters),
    actorKinds: selection(initial.actor_kinds, [...ACTOR_KINDS], initial.strict_filter_modes?.actor_kinds ?? initial.strict_filters),
    moveColumnIds: selection(initial.move_column_ids, columns.map(({ id }) => id), initial.strict_filter_modes?.move_column_ids ?? initial.strict_filters),
  }
}

function sameSelection<T>(before: T[], after: T[]): boolean {
  return before.length === after.length && before.every((value) => after.includes(value))
}

function selection<T>(selected: T[] | undefined, all: T[], strict?: boolean): T[] {
  return strict || selected?.length ? selected ?? [] : all
}

function Field({
  label,
  hint,
  error,
  children,
}: {
  label: string
  hint?: string
  error?: string
  children: ReactNode
}) {
  return (
    <div className="space-y-1.5">
      <Label>{label}</Label>
      {hint ? <p className="text-xs text-muted-foreground">{hint}</p> : null}
      {children}
      {error ? <p className="text-xs text-destructive">{error}</p> : null}
    </div>
  )
}

function MultiCheck<T extends string>({
  options,
  selected,
  onChange,
}: {
  options: { value: T; label: string }[]
  selected: T[]
  onChange: (v: T[]) => void
}) {
  const toggle = (value: T) => {
    if (selected.includes(value)) onChange(selected.filter((v) => v !== value))
    else onChange([...selected, value])
  }
  return (
    <div className="space-y-1 rounded-md border border-border p-2 text-sm">
      <ul className="max-h-32 space-y-1 overflow-y-auto">
        {options.map((o) => (
          <li key={o.value}>
            <label className="flex cursor-pointer items-center gap-2">
              <input type="checkbox" checked={selected.includes(o.value)} onChange={() => toggle(o.value)} />
              <span>{o.label}</span>
            </label>
          </li>
        ))}
      </ul>
    </div>
  )
}