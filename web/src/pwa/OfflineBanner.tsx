import { useTranslation } from '@/i18n/react'
import { useOnlineStatus } from './useOnlineStatus'

/**
 * 全局离线提示条：只在离线时显示，告知用户当前只能查看缓存内容。
 */
export function OfflineBanner() {
  const online = useOnlineStatus()
  const { t } = useTranslation()

  if (online) {
    return null
  }

  return (
    <div
      role="status"
      className="bg-signal text-signal-foreground border-signal/30 w-full border-b px-3 py-1.5 text-center text-ui"
    >
      {t('pwa.offlineBanner')}
    </div>
  )
}