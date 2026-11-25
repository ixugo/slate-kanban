// 登录页：输入访问口令。只在服务端启用了口令、当前浏览器还没通过时出现。

import { type FormEvent, useState } from 'react'
import { useTranslation } from '@/i18n/react'
import { LanguageMenu } from '@/app/LanguageMenu'
import { Logo } from '@/app/Logo'
import { ThemeMenu } from '@/app/ThemeMenu'
import { startWindowDrag } from '@/app/windowDrag'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ApiError } from '@/lib/api/client'
import { describeError } from '@/lib/errors'
import { PASSCODE_MAX } from '@/lib/limits'
import { useDocumentTitle } from '@/lib/useDocumentTitle'
import { useLogin } from './useAuth'

/** 登录页组件；口令错了、被限流了都在输入框下面直接说明 */
export default function LoginPage() {
  const { t } = useTranslation()
  useDocumentTitle(t('auth.title'))
  const [passcode, setPasscode] = useState('')
  const login = useLogin()

  // 提交口令；空口令不发请求，免得白占一次服务端的猜口令次数
  const submit = (event: FormEvent) => {
    event.preventDefault()
    const value = passcode.trim()
    if (value === '' || login.isPending) return
    login.mutate(value)
  }

  // 401 是口令不对，用专门的提示；其他失败（被限流、断网等）按通用的错误文案
  const error = login.error
  const message =
    error === null ? '' : error instanceof ApiError && error.status === 401 ? t('auth.wrong') : describeError(error, t).message

  return (
    <div className="flex min-h-dvh flex-col">
      <header className="window-titlebar flex h-14 shrink-0 items-center justify-end gap-0.5 px-3 sm:px-5" onMouseDown={startWindowDrag}>
        <LanguageMenu />
        <ThemeMenu />
      </header>
      <main className="flex flex-1 items-center justify-center p-6">
        <form onSubmit={submit} className="flex w-full max-w-sm flex-col gap-5 rounded-[1.5rem] bg-card p-8 shadow-card">
          <Logo className="size-12" />
          <div className="space-y-1.5">
            <h1 className="text-title font-semibold tracking-tight">{t('auth.title')}</h1>
            <p className="text-ui text-muted-foreground">{t('auth.description')}</p>
          </div>
          <div className="space-y-2">
            <Label htmlFor="passcode">{t('auth.passcode')}</Label>
            <Input
              id="passcode"
              type="password"
              autoComplete="current-password"
              autoFocus
              maxLength={PASSCODE_MAX}
              value={passcode}
              onChange={(event) => setPasscode(event.target.value)}
              aria-invalid={message !== ''}
              aria-describedby={message !== '' ? 'passcode-error' : undefined}
            />
            {message !== '' && (
              <p id="passcode-error" role="alert" className="text-ui text-destructive">
                {message}
              </p>
            )}
          </div>
          <Button type="submit" disabled={passcode.trim() === '' || login.isPending}>
            {t('auth.submit')}
          </Button>
        </form>
      </main>
    </div>
  )
}
