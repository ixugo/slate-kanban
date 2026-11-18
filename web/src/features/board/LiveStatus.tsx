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
      ? 'bg-success'
      : state === 'connecting'
        ? 'bg-signal animate-pulse'
        : 'bg-muted-foreground'
  return (
    <div className="flex shrink-0 items-center gap-2 rounded-full bg-secondary/70 px-3 py-1.5 text-xs font-medium text-muted-foreground">
      <span className={cn('size-2 shrink-0 rounded-full', dotClass)} aria-hidden />
      <span className="hidden sm:inline">{label}</span>
      {state === 'offline' && !gone && (
        <Button type="button" variant="ghost" size="xs" className="-mr-1.5" onClick={onRetry}>
          {t('common.retry')}
        </Button>
      )}
    </div>
  )
}