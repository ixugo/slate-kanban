// 看板已被删除时的整页提示。

import { Link } from 'react-router-dom'
import { useTranslation } from '@/i18n/react'
import { Button } from '@/components/ui/button'

/** 看板不存在时的占位页 */
export function BoardGone() {
  const { t } = useTranslation()
  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-4 p-8 text-center">
      <div className="space-y-1">
        <h1 className="text-title font-semibold tracking-tight">{t('board.gone.title')}</h1>
        <p className="text-body text-muted-foreground">{t('board.gone.body')}</p>
      </div>
      <Button asChild>
        <Link to="/">{t('board.gone.home')}</Link>
      </Button>
    </div>
  )
}