// 新建项目对话框：名称必填，其余选填；起步模板用一个开关在「研发流程」和「空白」之间切换，
// 语言取当前界面语言，起步的列和规则就是这种语言。项目路径由 agent 通过 MCP 回填，这里不问。

import { Loader2Icon } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from '@/i18n/react'
import { useNavigate } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { useLanguage } from '@/i18n/useLanguage'
import type { BoardTemplate } from '@/lib/api/types'
import { BOARD_DESC_MAX, BOARD_NAME_MAX, SLUG_MAX } from '@/lib/limits'
import { useErrorToast } from '@/lib/useErrorToast'
import { useCreateBoard } from './useBoards'
import { validateBoardForm, type BoardFormErrors } from './validation'

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
  const [template, setTemplate] = useState<BoardTemplate>('dev')
  const [errors, setErrors] = useState<BoardFormErrors>({})

  const submit = (e: FormEvent) => {
    e.preventDefault()
    const found = validateBoardForm({ name, slug })
    setErrors(found)
    if (Object.keys(found).length > 0) return
    create.mutate(
      {
        name: name.trim(),
        slug: slug.trim() || undefined,
        description: description.trim() || undefined,
        template,
        locale: lang,
      },
      {
        // 跳到新项目后本页面卸载，对话框随之消失，不需要再手动关闭
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

      <div className="flex items-center justify-between gap-3 rounded-2xl bg-secondary px-4 py-3">
        <Label htmlFor="board-template">{t('boards.create.template')}</Label>
        <div className="flex items-center gap-2.5">
          <span className="text-ui text-muted-foreground" aria-live="polite">
            {t(template === 'dev' ? 'boards.create.templates.dev' : 'boards.create.templates.empty')}
          </span>
          <Switch
            id="board-template"
            checked={template === 'dev'}
            onCheckedChange={(on) => setTemplate(on ? 'dev' : 'empty')}
            aria-label={t('boards.create.templateSwitch')}
          />
        </div>
      </div>

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

/** 新建项目对话框；关闭时表单整个卸载，下次打开重新开始。没有说明文字，所以显式声明不需要 describedby */
export function CreateBoardDialog({ open, onOpenChange }: DialogProps) {
  const { t } = useTranslation()
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-md" aria-describedby={undefined}>
        <DialogHeader>
          <DialogTitle>{t('boards.create.title')}</DialogTitle>
        </DialogHeader>
        <CreateBoardForm onDone={() => onOpenChange(false)} />
      </DialogContent>
    </Dialog>
  )
}
