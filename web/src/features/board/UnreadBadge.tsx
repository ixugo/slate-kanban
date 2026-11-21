/** 任务未读角标：大于 0 才显示，超过 99 显示 99+，并带无障碍说明 */

import { useTranslation } from '@/i18n/react'
import { cn } from '@/lib/utils'

interface UnreadBadgeProps {
  count: number
  className?: string
}

/** 把未读数格式化成角标上的文字 */
function formatUnreadCount(count: number): string | null {
  if (count <= 0) return null
  if (count > 99) return '99+'
  return String(count)
}

/** 看板磁贴和列表共用的未读角标 */
export function UnreadBadge({ count, className }: UnreadBadgeProps) {
  const { t } = useTranslation()
  const label = formatUnreadCount(count)
  if (!label) return null
  return (
    <span
      className={cn(
        'inline-flex min-w-[1.125rem] shrink-0 items-center justify-center rounded-full bg-primary px-1 py-0.5 font-mono text-[0.625rem] font-semibold leading-none text-primary-foreground',
        className,
      )}
      aria-label={t('board.unreadBadge', { count: label })}
    >
      {label}
    </span>
  )
}