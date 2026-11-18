// 多语言初始化：文案打包在页面里，不需要异步加载。
// 语言的选择顺序：用户手动选过的（localStorage）→ 浏览器的语言偏好 → 英文。

import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import { DEFAULT_LANG, LANG_STORAGE_KEY, LANGUAGES, normalizeLanguage, pickLanguage, type Lang } from './languages'
import { en } from './locales/en'
import { ja } from './locales/ja'
import { ko } from './locales/ko'
import { zhCN } from './locales/zh-CN'
import { zhTW } from './locales/zh-TW'

/** 五种语言的全部文案，键是语言代码 */
export const resources = {
  en: { translation: en },
  'zh-CN': { translation: zhCN },
  'zh-TW': { translation: zhTW },
  ja: { translation: ja },
  ko: { translation: ko },
} as const

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
function syncHtmlLang(lng: string): void {
  document.documentElement.lang = normalizeLanguage(lng)
}

/** 切换界面语言并记住这个选择；存储不可用时只在当前页面生效 */
export function setLanguage(lang: Lang): Promise<unknown> {
  try {
    localStorage.setItem(LANG_STORAGE_KEY, lang)
  } catch {
    // 存储不可用：这次切换仍然生效，只是刷新后不会记住
  }
  return i18n.changeLanguage(lang)
}

i18n.on('languageChanged', syncHtmlLang)

void i18n.use(initReactI18next).init({
  resources,
  lng: initialLanguage(),
  fallbackLng: DEFAULT_LANG,
  supportedLngs: LANGUAGES.map((l) => l.code),
  interpolation: { escapeValue: false },
  initAsync: false,
})

syncHtmlLang(i18n.language)

export default i18n
