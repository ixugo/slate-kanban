// 新建项目对话框：默认选择内置研发模板，同时可选空白模板或已导入的自定义模板。
// 内置模板的列和规则使用当前界面语言；项目路径由 agent 通过 MCP 回填。

import { Loader2Icon } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { useTranslation } from '@/i18n/react'
import { useNavigate } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { useLanguage } from '@/i18n/useLanguage'
import { BOARD_DESC_MAX, BOARD_NAME_MAX, SLUG_MAX } from '@/lib/limits'
import { useErrorToast } from '@/lib/useErrorToast'
import { listProjectTemplates } from '@/lib/api/endpoints'
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
  const [template, setTemplate] = useState('dev')
  const templates = useQuery({ queryKey: ['project-templates'], queryFn: ({ signal }) => listProjectTemplates(signal) })
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
        description: description || undefined,
        ...(template === 'dev' || template === 'empty'
          ? { template, locale: lang }
          : { project_template_id: Number(template) }),
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

      <div className="space-y-1.5">
        <Label htmlFor="board-template">{t('boards.create.template')}</Label>
        <select
          id="board-template"
          className="h-9 w-full rounded-lg border border-input bg-field px-2.5 text-ui outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/25"
          value={template}
          onChange={(e) => setTemplate(e.target.value)}
        >
          <option value="dev">{t('boards.create.templates.dev')}</option>
          <option value="empty">{t('boards.create.templates.empty')}</option>
          {templates.data?.filter((tpl) => !tpl.builtin).map((tpl) => (
            <option key={tpl.id} value={tpl.id}>{tpl.name}</option>
          ))}
        </select>
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
