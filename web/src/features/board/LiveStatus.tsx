// 工具栏上的实时同步状态：小圆点加文字，离线时提供重试。

import { useTranslation } from '@/i18n/react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import type { LiveState } from './useBoardLive'

interface LiveStatusProps {
  state: LiveState
  onRetry: () => void
  gone?: boolean
}

/** 根据 useBoardLive 的状态显示连接指示 */
export function LiveStatus({ state, onRetry, gone }: LiveStatusProps) {
  const { t } = useTranslation()
  const label = gone ? t('stream.notFound') : t(`stream.${state}`)
  const dotClass =
    state === 'live'
      ? 'bg-emerald-500'
      : state === 'connecting'
        ? 'bg-amber-500 animate-pulse'
        : 'bg-muted-foreground'
  return (
    <div className="flex items-center gap-2 text-xs text-muted-foreground">
      <span className={cn('size-2 shrink-0 rounded-full', dotClass)} aria-hidden />
      <span className="hidden sm:inline">{label}</span>
      {state === 'offline' && !gone && (
        <Button type="button" variant="ghost" size="sm" className="h-7 px-2 text-xs" onClick={onRetry}>
          {t('common.retry')}
        </Button>
      )}
    </div>
  )
}