// 界面文案在 React 里的统一入口：用 useSyncExternalStore 订阅语言变化，换掉 react-i18next。

import { useMemo, useSyncExternalStore } from 'react'
import i18n from './index'
import { getT, subscribe, getLanguageSnapshot } from './runtime'
import type { TFunction } from './types'

export type { TFunction } from './types'

/** 返回当前语言的 t 和 i18n 句柄；切换语言时本 hook 会使组件重新渲染 */
export function useTranslation(): { t: TFunction; i18n: typeof i18n } {
  const lang = useSyncExternalStore(subscribe, getLanguageSnapshot, getLanguageSnapshot)
  // lang 变时换新闭包，让依赖 t 的 useCallback/useMemo 跟着刷新
  const t = useMemo((): TFunction => {
    void lang
    const translate = getT()
    return ((key, params) => translate(key, params)) as TFunction
  }, [lang])
  return { t, i18n }
}