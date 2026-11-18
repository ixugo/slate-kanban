// 新建看板对话框：名称必填，其余选填；起步模板三选一，语言取当前界面语言，起步的列和规则就是这种语言。

import { Loader2Icon } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
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
import { Textarea } from '@/components/ui/textarea'
import { useLanguage } from '@/i18n/useLanguage'
import type { BoardTemplate } from '@/lib/api/types'
import { BOARD_DESC_MAX, BOARD_NAME_MAX, PROJECT_PATH_MAX, SLUG_MAX } from '@/lib/limits'
import { cn } from '@/lib/utils'
import { useErrorToast } from '@/lib/useErrorToast'
import { useCreateBoard } from './useBoards'
import { validateBoardForm, type BoardFormErrors } from './validation'

/** 三种模板，按展示顺序排列 */
const TEMPLATES: BoardTemplate[] = ['dev', 'simple', 'empty']

interface FormProps {
  onDone: () => void
}

/** 对话框里的表单。单独成组件，是为了每次打开都是一份全新的状态 */
function CreateBoardForm({ onDone }: FormProps) {
  const { t } = useTranslation()
  const { lang } = useLanguage()
  const navigate = useNavigate()
  const toastError = useErrorToast()
  const create = useCreateBoard()
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [description, setDescription] = useState('')
  const [projectPath, setProjectPath] = useState('')
  const [template, setTemplate] = useState<BoardTemplate>('dev')
  const [errors, setErrors] = useState<BoardFormErrors>({})

  const submit = (e: FormEvent) => {
    e.preventDefault()
    const found = validateBoardForm({ name, slug, projectPath })
    setErrors(found)
    if (Object.keys(found).length > 0) return
    create.mutate(
      {
        name: name.trim(),
        slug: slug.trim() || undefined,
        description: description.trim() || undefined,
        project_path: projectPath.trim() || undefined,
        template,
        locale: lang,
      },
      {
        // 跳到新看板后本页面卸载，对话框随之消失，不需要再手动关闭
        onSuccess: (board) => navigate(`/b/${board.id}`),
        onError: toastError,
      },
    )
  }

  // 字段下方的错误提示；没有错误不渲染
  const fieldError = (key: keyof BoardFormErrors, id: string) =>
    errors[key] && (
      <p id={id} role="alert" className="text-xs text-destructive">
        {t(`boards.create.errors.${errors[key]}`)}
      </p>
    )

  return (
    <form onSubmit={submit} noValidate className="space-y-4">
      <div className="space-y-1.5">
        <Label htmlFor="board-name">{t('boards.create.name')}</Label>
        <Input
          id="board-name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder={t('boards.create.namePlaceholder')}
          maxLength={BOARD_NAME_MAX}
          aria-invalid={!!errors.name}
          aria-describedby={errors.name ? 'board-name-error' : undefined}
          autoFocus
        />
        {fieldError('name', 'board-name-error')}
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="board-slug">
          {t('boards.create.slug')} <span className="font-normal text-muted-foreground">· {t('common.optional')}</span>
        </Label>
        <Input
          id="board-slug"
          value={slug}
          onChange={(e) => setSlug(e.target.value.toUpperCase())}
          placeholder={t('boards.create.slugPlaceholder')}
          maxLength={SLUG_MAX}
          autoCapitalize="characters"
          spellCheck={false}
          className="font-mono"
          aria-invalid={!!errors.slug}
          aria-describedby={errors.slug ? 'board-slug-error board-slug-hint' : 'board-slug-hint'}
        />
        {fieldError('slug', 'board-slug-error')}
        <p id="board-slug-hint" className="text-xs text-muted-foreground">
          {t('boards.create.slugHint')}
        </p>
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="board-description">
          {t('boards.create.descriptionLabel')}{' '}
          <span className="font-normal text-muted-foreground">· {t('common.optional')}</span>
        </Label>
        <Textarea
          id="board-description"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          placeholder={t('boards.create.descriptionPlaceholder')}
          maxLength={BOARD_DESC_MAX}
          rows={2}
        />
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="board-path">
          {t('boards.create.projectPath')}{' '}
          <span className="font-normal text-muted-foreground">· {t('common.optional')}</span>
        </Label>
        <Input
          id="board-path"
          value={projectPath}
          onChange={(e) => setProjectPath(e.target.value)}
          placeholder={t('boards.create.projectPathPlaceholder')}
          maxLength={PROJECT_PATH_MAX}
          spellCheck={false}
          className="font-mono"
          aria-invalid={!!errors.projectPath}
          aria-describedby={errors.projectPath ? 'board-path-error board-path-hint' : 'board-path-hint'}
        />
        {fieldError('projectPath', 'board-path-error')}
        <p id="board-path-hint" className="text-xs text-muted-foreground">
          {t('boards.create.projectPathHint')}
        </p>
      </div>

      <fieldset className="space-y-1.5">
        <legend className="mb-1.5 text-xs font-medium">{t('boards.create.template')}</legend>
        <div className="grid gap-1.5">
          {TEMPLATES.map((key) => (
            <label
              key={key}
              className={cn(
                'flex cursor-pointer items-start gap-2.5 border p-2.5 transition-colors',
                'has-[:checked]:border-signal has-[:checked]:bg-signal/10 has-[:focus-visible]:ring-1 has-[:focus-visible]:ring-ring',
              )}
            >
              <input
                type="radio"
                name="board-template"
                value={key}
                checked={template === key}
                onChange={() => setTemplate(key)}
                className="mt-0.5 accent-[var(--signal)]"
              />
              <span className="space-y-0.5">
                <span className="block font-medium">{t(`boards.create.templates.${key}.name`)}</span>
                <span className="block text-muted-foreground">{t(`boards.create.templates.${key}.description`)}</span>
              </span>
            </label>
          ))}
        </div>
      </fieldset>

      <DialogFooter>
        <Button type="button" variant="outline" onClick={onDone}>
          {t('common.cancel')}
        </Button>
        <Button type="submit" disabled={create.isPending}>
          {create.isPending && <Loader2Icon className="animate-spin" />}
          {create.isPending ? t('boards.create.submitting') : t('boards.create.submit')}
        </Button>
      </DialogFooter>
    </form>
  )
}

interface DialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

/** 新建看板对话框；关闭时表单整个卸载，下次打开重新开始 */
export function CreateBoardDialog({ open, onOpenChange }: DialogProps) {
  const { t } = useTranslation()
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t('boards.create.title')}</DialogTitle>
          <DialogDescription>{t('boards.create.description')}</DialogDescription>
        </DialogHeader>
        <CreateBoardForm onDone={() => onOpenChange(false)} />
      </DialogContent>
    </Dialog>
  )
}
