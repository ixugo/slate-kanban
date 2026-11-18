// 当前语言、订阅通知和 t 函数：替代 i18next 的运行时核心。

import { getBundle, loadLanguage } from './bundles'
import { DEFAULT_LANG, LANG_STORAGE_KEY, LANGUAGES, normalizeLanguage, pickLanguage, type Lang } from './languages'
import { resolveMessage } from './lookup'
import type { TFunction } from './types'
import { en } from './locales/en'

const listeners = new Set<() => void>()
let currentLang: Lang = DEFAULT_LANG

/** 通知 React 等订阅方：语言或文案树已变，需要重新取 t */
function emit(): void {
  for (const fn of listeners) fn()
}

/** 注册语言变化监听；返回取消函数 */
export function subscribe(listener: () => void): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

/** 给 useSyncExternalStore 用的快照：只关心语言代码是否变化 */
export function getLanguageSnapshot(): Lang {
  return currentLang
}

// 读取用户手动选过的语言；没选过、值不合法或存储不可用都返回 null。
function readStoredLanguage(): Lang | null {
  try {
    const saved = localStorage.getItem(LANG_STORAGE_KEY)
    return LANGUAGES.find((l) => l.code === saved)?.code ?? null
  } catch {
    return null
  }
}

/** 决定启动时用哪种语言 */
export function initialLanguage(): Lang {
  return readStoredLanguage() ?? pickLanguage(navigator.languages)
}

// 让 <html lang> 跟着界面语言走：中日韩共用的汉字要靠它才会选对字形，读屏软件也靠它选发音。
function syncHtmlLang(lng: Lang): void {
  document.documentElement.lang = normalizeLanguage(lng)
}

/** 为指定语言构造 t；缺键先回退英文，再缺则返回 key 本身 */
function makeT(lang: Lang): TFunction {
  const translate = (key: string, params?: Record<string, string | number>): string => {
    const primary = getBundle(lang)
    const hit = resolveMessage(primary, key, params)
    if (hit !== undefined) return hit
    const fallback = resolveMessage(en, key, params)
    if (fallback !== undefined) return fallback
    return key
  }
  return translate as TFunction
}

/** 启动时设定语言并同步 html；主入口在 render 前还会 await loadLanguage */
export function initRuntimeLanguage(lang: Lang): void {
  currentLang = lang
  syncHtmlLang(lang)
}

/** 切换界面语言：先加载包、再改状态、写 localStorage、同步 html */
export async function setLanguage(lang: Lang): Promise<void> {
  await loadLanguage(lang)
  try {
    localStorage.setItem(LANG_STORAGE_KEY, lang)
  } catch {
    // 存储不可用：这次切换仍然生效，只是刷新后不会记住
  }
  currentLang = lang
  syncHtmlLang(lang)
  emit()
}

/** 固定某一语言的 t，测试里 describeError 不依赖当前界面语言 */
export function getFixedT(lang: Lang): TFunction {
  return makeT(lang)
}

/** 当前界面语言的 t */
export function getT(): TFunction {
  return makeT(currentLang)
}

/** 当前界面语言代码 */
export function getCurrentLanguage(): Lang {
  return currentLang
}