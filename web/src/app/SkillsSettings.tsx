// 全局 Skill 管理：先加载目录，展开单条时才取 Markdown 正文。
import { ChevronRightIcon, PlusIcon, Trash2Icon } from 'lucide-react'
import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog'
import { LazyEditor, type LiveEditorDraft } from '@/features/editor/LazyEditor'
import { useTranslation } from '@/i18n/react'
import { createSkill, deleteSkill, getSkill, listSkills, updateSkill } from '@/lib/api/endpoints'
import type { SkillDetail, SkillInput } from '@/lib/api/types'
import { Markdown } from '@/lib/markdown'
import { SKILL_BODY_MAX, SKILL_DESCRIPTION_MAX, SKILL_NAME_MAX } from '@/lib/limits'
import { toast } from '@/lib/toast'
import { useErrorToast } from '@/lib/useErrorToast'

const listKey = ['skills', 'list'] as const
const detailKey = (name: string) => ['skills', 'detail', name] as const
const validName = /^[a-z0-9]+(?:-[a-z0-9]+)*$/

export function SkillsSettings() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const showError = useErrorToast()
  // null 表示未打开，空字符串表示新建，其余是当前名称。
  const [selected, setSelected] = useState<string | null>(null)
  const [deleting, setDeleting] = useState<string | null>(null)
  const skills = useQuery({ queryKey: listKey, queryFn: ({ signal }) => listSkills(signal) })
  const remove = useMutation({
    mutationFn: deleteSkill,
    onSuccess: (_, name) => {
      queryClient.removeQueries({ queryKey: detailKey(name) })
      void queryClient.invalidateQueries({ queryKey: listKey })
      setDeleting(null)
      toast.success(t('app.skills.deleted'))
    },
    onError: showError,
  })
  return (
    <div className="space-y-4">
      <p className="text-ui text-muted-foreground">{t('app.skills.intro')}</p>
      <div className="flex justify-end">
        <Button type="button" size="sm" onClick={() => setSelected('')}><PlusIcon />{t('app.skills.new')}</Button>
      </div>
      {skills.isPending && <p className="text-ui text-muted-foreground">{t('common.loading')}</p>}
      {skills.isError && (
        <div className="space-y-2" role="alert">
          <p className="text-ui text-destructive">{t('app.skills.loadFailed')}</p>
          <Button type="button" variant="outline" size="sm" onClick={() => void skills.refetch()}>{t('common.retry')}</Button>
        </div>
      )}
      {skills.data && (
        <ul className="divide-y divide-border rounded-2xl border border-border bg-card shadow-card">
          {skills.data.map((item) => (
            <li key={item.name} className="flex min-w-0 items-center gap-1 px-2 py-1">
              <button
                type="button"
                aria-label={t('app.skills.view', { name: item.name })}
                onClick={() => setSelected(item.name)}
                className="min-w-0 flex-1 rounded-xl p-2 text-left outline-none transition-colors hover:bg-secondary/60 focus-visible:ring-2 focus-visible:ring-ring"
              >
                <span className="flex min-w-0 items-center gap-2">
                  <span className="min-w-0 flex-1 break-all font-medium">{item.name}</span>
                  <span className="shrink-0 text-xs text-muted-foreground">{t(item.builtin ? 'app.skills.builtin' : 'app.skills.custom')}</span>
                  <ChevronRightIcon className="size-4 shrink-0 text-muted-foreground" aria-hidden />
                </span>
                <span className="mt-1 line-clamp-2 break-words text-xs text-muted-foreground">{item.description}</span>
              </button>
              {!item.builtin && (
                <Button type="button" variant="ghost" size="icon" className="text-destructive hover:text-destructive"
                  aria-label={t('app.skills.delete', { name: item.name })} title={t('app.skills.delete', { name: item.name })}
                  disabled={remove.isPending} onClick={() => setDeleting(item.name)}>
                  <Trash2Icon />
                </Button>
              )}
            </li>
          ))}
        </ul>
      )}
      {selected !== null && <SkillDialog key={selected} name={selected} onClose={() => setSelected(null)} />}
      <AlertDialog open={deleting !== null} onOpenChange={(open) => !open && !remove.isPending && setDeleting(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('app.skills.deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>{t('app.skills.deleteBody', { name: deleting ?? '' })}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={remove.isPending}>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction variant="destructive" disabled={remove.isPending} onClick={(event) => {
              event.preventDefault()
              if (deleting) remove.mutate(deleting)
            }}>{t(remove.isPending ? 'common.deleting' : 'common.delete')}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

function SkillDialog({ name, onClose }: { name: string; onClose: () => void }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const showError = useErrorToast()
  const detail = useQuery({
    queryKey: detailKey(name),
    queryFn: ({ signal }) => getSkill(name, signal),
    enabled: name !== '',
    refetchOnWindowFocus: false,
  })
  const save = useMutation({
    mutationFn: (input: SkillInput) => name ? updateSkill(name, input) : createSkill(input),
    onSuccess: (item) => {
      if (name && name !== item.name) queryClient.removeQueries({ queryKey: detailKey(name) })
      queryClient.setQueryData(detailKey(item.name), item)
      void queryClient.invalidateQueries({ queryKey: listKey })
      toast.success(t('app.skills.saved'))
      onClose()
    },
    onError: showError,
  })
  return (
    <Dialog open onOpenChange={(open) => !open && !save.isPending && onClose()}>
      <DialogContent className="max-h-[90dvh] w-[min(44rem,96vw)] max-w-none gap-0 overflow-y-auto sm:max-w-none" showCloseButton={!save.isPending}>
        <DialogHeader className="pr-10">
          <DialogTitle className="break-all">{name || t('app.skills.new')}</DialogTitle>
          <DialogDescription>{detail.data?.builtin ? t('app.skills.readOnly') : t('app.skills.markdownHint')}</DialogDescription>
        </DialogHeader>
        {name && detail.isError ? (
          <div className="mt-4 space-y-2" role="alert">
            <p className="text-destructive">{t('app.skills.loadFailed')}</p>
            <Button type="button" variant="outline" onClick={() => void detail.refetch()}>{t('common.retry')}</Button>
          </div>
        ) : name && (detail.isPending || detail.isFetching) ? (
          <p className="mt-4 text-muted-foreground">{t('common.loading')}</p>
        ) : detail.data?.builtin ? (
          <div className="mt-4 space-y-3">
            <p className="break-words text-muted-foreground">{detail.data.description}</p>
            <code className="block break-all rounded-lg bg-secondary p-2 text-xs">{detail.data.uri}</code>
            <div className="rounded-xl border border-border p-4"><Markdown source={detail.data.body} /></div>
          </div>
        ) : (
          <SkillForm initial={detail.data} busy={save.isPending} onCancel={onClose} onSave={(input) => save.mutate(input)} />
        )}
      </DialogContent>
    </Dialog>
  )
}

function SkillForm({ initial, busy, onCancel, onSave }: { initial?: SkillDetail; busy: boolean; onCancel: () => void; onSave: (input: SkillInput) => void }) {
  const { t } = useTranslation()
  const [name, setName] = useState(initial?.name ?? '')
  const [description, setDescription] = useState(initial?.description ?? '')
  const [draft, setDraft] = useState<LiveEditorDraft>({ value: initial?.body ?? '' })
  const bytes = new TextEncoder().encode(draft.value).length
  const nameOK = validName.test(name) && name.length <= SKILL_NAME_MAX
  const canSaveBody = (body: string) => !busy && nameOK && description.trim() !== '' && body.trim() !== '' && new TextEncoder().encode(body).length <= SKILL_BODY_MAX
  const canSave = canSaveBody(draft.value)
  const submit = (body: string) => {
    if (canSaveBody(body)) onSave({ name, description, body })
  }
  return (
    <form className="mt-4 space-y-4" onSubmit={(event) => {
      event.preventDefault()
      submit(draft.value)
    }}>
      <div className="grid gap-1.5 font-medium">
        <label htmlFor="skill-name">{t('app.skills.name')}</label>
        <Input id="skill-name" aria-describedby="skill-name-hint" value={name} onChange={(event) => setName(event.target.value)} maxLength={SKILL_NAME_MAX} autoComplete="off" disabled={busy} aria-invalid={name !== '' && !nameOK} />
        <span id="skill-name-hint" className="text-xs font-normal text-muted-foreground">{t('app.skills.nameHint')}</span>
      </div>
      {nameOK && <code className="block break-all rounded-lg bg-secondary p-2 text-xs">skill://{name}/SKILL.md</code>}
      <div className="grid gap-1.5 font-medium">
        <label htmlFor="skill-description">{t('app.skills.description')}</label>
        <Input id="skill-description" aria-describedby="skill-description-hint" value={description} onChange={(event) => setDescription(event.target.value)} maxLength={SKILL_DESCRIPTION_MAX} disabled={busy} />
        <span id="skill-description-hint" className="text-xs font-normal text-muted-foreground">{t('app.skills.descriptionHint')}</span>
      </div>
      <div className="space-y-1.5">
        <div className="flex items-baseline justify-between gap-2">
          <span className="font-medium">{t('app.skills.body')}</span>
          <span className={`text-xs ${bytes > SKILL_BODY_MAX ? 'text-destructive' : 'text-muted-foreground'}`}>{t('app.skills.bytes', { size: bytes, max: SKILL_BODY_MAX })}</span>
        </div>
        <div inert={busy}>
          <LazyEditor {...draft} onChange={setDraft} onSubmit={submit} max={SKILL_BODY_MAX} label={t('app.skills.body')} placeholder={t('app.skills.placeholder')} />
        </div>
      </div>
      <DialogFooter>
        <Button type="button" variant="outline" disabled={busy} onClick={onCancel}>{t('common.cancel')}</Button>
        <Button type="submit" disabled={!canSave}>{t(busy ? 'common.saving' : 'common.save')}</Button>
      </DialogFooter>
    </form>
  )
}
