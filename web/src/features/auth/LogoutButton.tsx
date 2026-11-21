// 退出按钮：只在服务端启用了访问口令、并且当前浏览器已通过时才出现。

import { LogOutIcon } from 'lucide-react'
import { useTranslation } from '@/i18n/react'
import { Button } from '@/components/ui/button'
import { useErrorToast } from '@/lib/useErrorToast'
import { useAuthStatus, useLogout } from './useAuth'

/** 退出按钮；没有启用口令时不渲染任何东西 */
export function LogoutButton() {
  const { t } = useTranslation()
  const { data } = useAuthStatus()
  const logout = useLogout()
  const toastError = useErrorToast()
  if (!data?.required || !data.authenticated) return null
  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      disabled={logout.isPending}
      onClick={() => logout.mutate(undefined, { onError: toastError })}
    >
      <LogOutIcon />
      {t('auth.logout')}
    </Button>
  )
}
