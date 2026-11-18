// 懒加载的页面还没下载完时显示的占位：一条细的加载条，不闪屏，也不跳动布局。

import { useTranslation } from '@/i18n/react'

/** 页面加载中的占位，读屏软件会读出"加载中" */
export function PageFallback() {
  const { t } = useTranslation()
  return (
    <div role="status" aria-label={t('common.loading')} className="h-0.5 w-full overflow-hidden rounded-full bg-muted">
      <div className="h-full w-1/3 animate-pulse rounded-full bg-primary" />
    </div>
  )
}
