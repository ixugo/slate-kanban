// 全局设置里的「模板」页：展示内置模板，并管理导入的自定义模板。

import { Loader2Icon, Trash2Icon, UploadIcon } from 'lucide-react'
import { useRef, useState } from 'react'
import { useTranslation } from '@/i18n/react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { deleteProjectTemplate, importProjectTemplate, listProjectTemplates } from '@/lib/api/endpoints'
import { ApiError } from '@/lib/api/client'
import { useErrorToast } from '@/lib/useErrorToast'
import { toast } from '@/lib/toast'

/** 全局设置中的项目模板管理区 */
export function TemplatesSettings() {
  const { t } = useTranslation()
  const toastError = useErrorToast()
  const queryClient = useQueryClient()
  const fileRef = useRef<HTMLInputElement>(null)
  const [pendingFile, setPendingFile] = useState<File | null>(null)
  const [pendingName, setPendingName] = useState<string | null>(null)

  const templates = useQuery({
    queryKey: ['project-templates'],
    queryFn: ({ signal }) => listProjectTemplates(signal),
  })

  const customTemplates = templates.data?.filter((item) => !item.builtin)

  const remove = useMutation({
    mutationFn: (id: number) => deleteProjectTemplate(id),
    onSuccess: () => {
      toast.success(t('app.templates.deletedToast'))
      void queryClient.invalidateQueries({ queryKey: ['project-templates'] })
    },
    onError: toastError,
  })

  const doImport = useMutation({
    mutationFn: ({ data, replace }: { data: unknown; replace: boolean }) => importProjectTemplate(data, replace),
    onSuccess: () => {
      toast.success(t('app.templates.importedToast'))
      setPendingFile(null)
      setPendingName(null)
      void queryClient.invalidateQueries({ queryKey: ['project-templates'] })
    },
    onError: toastError,
  })

  /** 用户选中文件后先解析，再检查是否与现有模板重名 */
  const onFileChosen = async (file: File) => {
    let parsed: unknown
    try {
      const text = await file.text()
      parsed = JSON.parse(text) as unknown
    } catch {
      toast.error(t('app.templates.invalidFile'))
      return
    }
    const name = typeof parsed === 'object' && parsed !== null && 'name' in parsed ? String((parsed as { name: unknown }).name) : ''
    const exists = customTemplates?.some((item) => item.name === name)
    if (exists && name) {
      setPendingFile(file)
      setPendingName(name)
      return
    }
    doImport.mutate({ data: parsed, replace: false })
  }

  /** 用户在重名确认框里选择覆盖 */
  const confirmReplace = async () => {
    if (!pendingFile) return
    const text = await pendingFile.text()
    const parsed = JSON.parse(text) as unknown
    doImport.mutate({ data: parsed, replace: true })
  }

  return (
    <div className="space-y-4">
      <p className="text-ui text-muted-foreground">{t('app.templates.body')}</p>
      <div className="flex flex-wrap gap-2">
        <input
          ref={fileRef}
          type="file"
          accept="application/json,.json"
          className="sr-only"
          onChange={(e) => {
            const file = e.target.files?.[0]
            if (file) void onFileChosen(file)
            e.target.value = ''
          }}
        />
        <Button type="button" variant="outline" size="sm" onClick={() => fileRef.current?.click()} disabled={doImport.isPending}>
          {doImport.isPending ? <Loader2Icon className="animate-spin" /> : <UploadIcon />}
          {t('app.templates.import')}
        </Button>
      </div>
      {templates.isPending && <p className="text-ui text-muted-foreground">{t('common.loading')}</p>}
      {templates.isError && (
        <p className="text-ui text-destructive" role="alert">
          {templates.error instanceof ApiError ? templates.error.message : t('app.templates.loadFailed')}
        </p>
      )}
      <ul className="divide-y divide-border rounded-2xl border border-border bg-card shadow-card">
        <li className="flex items-center justify-between gap-3 px-4 py-3">
          <span className="text-ui font-medium">{t('boards.create.templates.dev')}</span>
          <span className="text-xs text-muted-foreground">{t('app.templates.builtin')}</span>
        </li>
        {customTemplates?.map((item) => (
          <li key={item.id} className="flex items-center justify-between gap-3 px-4 py-3">
            <span className="text-ui font-medium">{item.name}</span>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="text-destructive hover:text-destructive"
              aria-label={t('app.templates.delete', { name: item.name })}
              disabled={remove.isPending}
              onClick={() => remove.mutate(item.id)}
            >
              <Trash2Icon />
            </Button>
          </li>
        ))}
      </ul>

      <Dialog open={pendingName !== null} onOpenChange={(open) => !open && (setPendingName(null), setPendingFile(null))}>
        <DialogContent aria-describedby="template-replace-desc">
          <DialogHeader>
            <DialogTitle>{t('app.templates.replaceTitle')}</DialogTitle>
            <DialogDescription id="template-replace-desc">{t('app.templates.replaceBody', { name: pendingName ?? '' })}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => (setPendingName(null), setPendingFile(null))}>
              {t('common.cancel')}
            </Button>
            <Button type="button" onClick={() => void confirmReplace()} disabled={doImport.isPending}>
              {t('app.templates.replaceConfirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}