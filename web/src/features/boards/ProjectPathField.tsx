import { useId } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useTranslation } from '@/i18n/react'
import { PROJECT_PATH_MAX } from '@/lib/limits'
import type { useProjectPath } from './useProjectPath'

interface Props {
  path: ReturnType<typeof useProjectPath>
  disabled?: boolean
}

/** 工作目录以服务端所在机器为准；网页选填，桌面客户端必填。 */
export function ProjectPathField({ path, disabled }: Props) {
  const { t } = useTranslation()
  const id = useId()
  const error = path.status === 'empty' ? t('boards.projectPath.required')
    : path.status === 'missing' ? t('errors.ErrProjectPathNotFound')
      : path.status === 'invalid' ? t('boards.projectPath.invalid')
        : path.status === 'failed' ? t('errors.ErrProjectPathCheckFailed') : ''

  const hint = path.required ? t('boards.projectPath.requiredHint') : t('boards.projectPath.hint')

  return (
    <div className="space-y-1.5">
      <Label htmlFor={id}>
        {t('boards.projectPath.label')}
        {!path.required && <span className="font-normal text-muted-foreground"> · {t('common.optional')}</span>}
      </Label>
      <Input
        id={id}
        value={path.value}
        onChange={(e) => path.change(e.target.value)}
        onBlur={() => { if (!disabled) void path.validate() }}
        placeholder={t('boards.projectPath.placeholder')}
        maxLength={PROJECT_PATH_MAX}
        disabled={disabled}
        autoCapitalize="none"
        spellCheck={false}
        className="font-mono"
        aria-invalid={!!error}
        aria-describedby={error ? `${id}-hint ${id}-error` : `${id}-hint`}
      />
      <p id={`${id}-hint`} className="text-xs text-muted-foreground">{hint}</p>
      {path.status === 'checking' && <p role="status" className="text-xs text-muted-foreground">{t('boards.projectPath.checking')}</p>}
      {error && (
        <div className="flex flex-wrap items-center gap-2">
          <p id={`${id}-error`} role="alert" className="text-xs text-destructive">{error}</p>
          {path.status === 'failed' && (
            <Button type="button" variant="link" size="xs" onClick={() => void path.validate()} disabled={disabled}>
              {t('common.retry')}
            </Button>
          )}
        </div>
      )}
    </div>
  )
}
