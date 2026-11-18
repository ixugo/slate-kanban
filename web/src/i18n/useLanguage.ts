// 读取和切换当前界面语言的钩子。

import { useTranslation } from './react'
import { normalizeLanguage, type Lang } from './languages'
import { setLanguage } from './index'

/** 返回当前语言和切换函数；语言变化时用到它的组件会重新渲染 */
export function useLanguage(): { lang: Lang; setLang: (lang: Lang) => void } {
  const { i18n } = useTranslation()
  return {
    lang: normalizeLanguage(i18n.language),
    setLang: (lang) => {
      void setLanguage(lang)
    },
  }
}
