// 地址不存在时的页面。

import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { useDocumentTitle } from '@/lib/useDocumentTitle'

/** 404 页面：一句说明加回到看板列表的按钮 */
export function NotFoundPage() {
  const { t } = useTranslation()
  useDocumentTitle(t('notFound.title'))
  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-3 p-8 text-center">
      <p className="font-mono text-5xl font-medium text-muted-foreground/60">404</p>
      <h1 className="text-lg font-semibold">{t('notFound.title')}</h1>
      <p className="max-w-sm text-muted-foreground">{t('notFound.body')}</p>
      <Button asChild className="mt-2">
        <Link to="/">{t('notFound.home')}</Link>
      </Button>
    </div>
  )
}
