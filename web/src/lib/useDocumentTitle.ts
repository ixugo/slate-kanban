// 浏览器标签页的标题：页面名加应用名，离开页面时恢复成应用名。

import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'

/** 设置标签页标题；title 为空时只显示应用名 */
export function useDocumentTitle(title: string | undefined): void {
  const { t } = useTranslation()
  useEffect(() => {
    const base = t('app.name')
    document.title = title ? `${title} · ${base}` : base
    return () => {
      document.title = base
    }
  }, [title, t])
}
