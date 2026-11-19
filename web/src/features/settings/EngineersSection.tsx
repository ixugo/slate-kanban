// 设置页「工程师」：名册列表，招募、编辑、解聘。每位工程师是 agent 的一个并发名额，保存时整体替换名册。

import { DicesIcon, Loader2Icon, PencilIcon, PlusIcon, Trash2Icon } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from '@/i18n/react'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import type { Engineer, EngineerInput, EngineerKind } from '@/lib/api/types'
import { ENGINEER_NAME_MAX, ENGINEER_PROMPT_MAX } from '@/lib/limits'
import { useErrorToast } from '@/lib/useErrorToast'
import { randomAvatarSeed } from './avatar'
import { EngineerAvatar } from './EngineerAvatar'
import { ENGINEER_KINDS, KIND_META } from './engineerKinds'
import { useSettingsMutations } from './useSettingsMutations'
import { validateDraft, type Draft, type DraftError } from './validation'

interface Props {
  boardId: number
  engineers: Engineer[]
}

/** 把名册转成提交用的入参 */
function toInputs(list: Engineer[]): EngineerInput[] {
  return list.map(({ id, name, kind, prompt, avatar }) => ({ id, name, kind, prompt, avatar }))
}

/** 工程师管理区块 */
export function EngineersSection({ boardId, engineers }: Props) {
  const { t } = useTranslation()
  const toastError = useErrorToast()
  const { updateBoard } = useSettingsMutations(boardId)
  const [editing, setEditing] = useState<Draft | null>(null)
  const [dismissing, setDismissing] = useState<Engineer | null>(null)

  const save = (next: EngineerInput[], done: () => void) =>
    updateBoard.mutate({ engineers: next }, { onSuccess: done, onError: toastError })
  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-ui text-muted-foreground">{t('settings.engineers.subtitle')}</p>
        <Button size="sm" onClick={() => setEditing({ id: 0, name: '', kind: 'dev', prompt: '', avatar: randomAvatarSeed() })}>
          <PlusIcon />
          {t('settings.engineers.recruit')}
        </Button>
      </div>
      {engineers.length === 0 ? (
        <div className="flex flex-col items-center gap-3 rounded-2xl bg-card px-4 py-10 text-center shadow-card">
          <EngineerAvatar seed="slate-empty" size={56} />
          <p className="text-ui text-muted-foreground">{t('settings.engineers.empty')}</p>
        </div>
      ) : (
        <ul className="grid gap-3 sm:grid-cols-2">
          {engineers.map((e) => (
            <li key={e.id} className="flex items-start gap-3 rounded-2xl bg-card p-4 shadow-card">
              <EngineerAvatar seed={e.avatar || `e${e.id}`} size={44} />
              <div className="min-w-0 flex-1 space-y-1.5">
                <p className="flex items-center gap-2 font-medium">
                  <span className="truncate">{e.name}</span>
                  <Badge className={KIND_META[e.kind].color}>{t(KIND_META[e.kind].label)}</Badge>
                </p>
                <p className="line-clamp-3 text-xs leading-relaxed text-muted-foreground">{e.prompt}</p>
              </div>
              <div className="flex shrink-0 gap-0.5">
                <Button type="button" size="icon-sm" variant="ghost" aria-label={t('common.edit')} onClick={() => setEditing({ ...e })}>
                  <PencilIcon />
                </Button>
                <Button type="button" size="icon-sm" variant="ghost" aria-label={t('settings.engineers.dismiss')} onClick={() => setDismissing(e)}>
                  <Trash2Icon />
                </Button>
              </div>
            </li>
          ))}
        </ul>
      )}
      {editing && (
        <EngineerDialog
          draft={editing}
          all={engineers}
          busy={updateBoard.isPending}
          onClose={() => setEditing(null)}
          onSubmit={(d) => {
            const next = d.id === 0 ? [...toInputs(engineers), { name: d.name, kind: d.kind, prompt: d.prompt, avatar: d.avatar }] : toInputs(engineers).map((e) => (e.id === d.id ? { ...d } : e))
            save(next, () => setEditing(null))
          }}
        />
      )}
      <AlertDialog open={dismissing !== null} onOpenChange={(v) => !v && setDismissing(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('settings.engineers.dismissTitle', { name: dismissing?.name ?? '' })}</AlertDialogTitle>
            <AlertDialogDescription>{t('settings.engineers.dismissBody')}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              onClick={(ev) => {
                ev.preventDefault()
                save(toInputs(engineers).filter((e) => e.id !== dismissing?.id), () => setDismissing(null))
              }}
            >
              {t('settings.engineers.dismiss')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

interface DialogProps {
  draft: Draft
  all: Engineer[]
  busy: boolean
  onClose: () => void
  onSubmit: (draft: Draft) => void
}

/** 招募或编辑一位工程师的对话框 */
function EngineerDialog({ draft, all, busy, onClose, onSubmit }: DialogProps) {
  const { t } = useTranslation()
  const [values, setValues] = useState<Draft>(draft)
  const [error, setError] = useState<DraftError | null>(null)
  const isNew = draft.id === 0

  const submit = (e: FormEvent) => {
    e.preventDefault()
    const found = validateDraft(values, all)
    setError(found)
    if (found) return
    onSubmit({ ...values, name: values.name.trim(), prompt: values.prompt.trim() })
  }

  return (
    <Dialog open onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{isNew ? t('settings.engineers.recruitTitle') : t('settings.engineers.editTitle')}</DialogTitle>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <div className="relative w-fit">
            <EngineerAvatar seed={values.avatar || `e${draft.id}`} size={64} />
            <button
              type="button"
              aria-label={t('settings.engineers.shuffleAvatar')}
              onClick={() => setValues({ ...values, avatar: randomAvatarSeed() })}
              className="absolute -right-2 -bottom-2 grid size-6 place-items-center rounded-full bg-card text-muted-foreground shadow-card ring-1 ring-border transition-colors hover:text-foreground"
            >
              <DicesIcon className="size-3.5" />
            </button>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="eng-name">{t('settings.engineers.name')}</Label>
            <Input id="eng-name" value={values.name} maxLength={ENGINEER_NAME_MAX} onChange={(e) => setValues({ ...values, name: e.target.value })} />
          </div>
          <div className="space-y-1.5">
            <Label>{t('settings.engineers.kind')}</Label>
            <Select value={values.kind} onValueChange={(v) => setValues({ ...values, kind: v as EngineerKind })}>
              <SelectTrigger aria-label={t('settings.engineers.kind')}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {ENGINEER_KINDS.map((k) => (
                  <SelectItem key={k} value={k}>
                    {t(KIND_META[k].label)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="eng-prompt">{t('settings.engineers.prompt')}</Label>
            <Textarea id="eng-prompt" rows={6} value={values.prompt} maxLength={ENGINEER_PROMPT_MAX} onChange={(e) => setValues({ ...values, prompt: e.target.value })} />
          </div>
          {error && (
            <p role="alert" className="text-xs text-destructive">
              {t(`settings.engineers.errors.${error}`)}
            </p>
          )}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" disabled={busy}>
              {busy && <Loader2Icon className="animate-spin" />}
              {isNew ? t('settings.engineers.recruit') : t('common.save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
