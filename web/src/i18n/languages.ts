// 支持的界面语言，以及把浏览器给的各种写法归并到这五种之一的规则。
// 语言代码和服务端 internal/core/kanban/board_template.go 里的 normalizeLocale 保持一致：
// 创建看板时把当前语言原样传给 locale，起步的列和规则就是这种语言。

/** 五种界面语言的代码 */
export type Lang = 'zh-CN' | 'zh-TW' | 'en' | 'ja' | 'ko'

/** 语言选择菜单里的条目。名称用各自的语言写，这样不管当前界面是什么语言都认得出来 */
export const LANGUAGES: readonly { code: Lang; label: string }[] = [
  { code: 'zh-CN', label: '简体中文' },
  { code: 'zh-TW', label: '繁體中文' },
  { code: 'en', label: 'English' },
  { code: 'ja', label: '日本語' },
  { code: 'ko', label: '한국어' },
]

/** 不认识的语言一律用英文 */
export const DEFAULT_LANG: Lang = 'en'

/** 用户手动选的语言存在 localStorage 的这个键里 */
export const LANG_STORAGE_KEY = 'slate.lang'

/**
 * 把各种写法的语言标识归并到支持的五种之一：大小写和下划线不敏感；
 * zh-TW、zh-HK、zh-MO、zh-Hant 开头的归繁体，其余 zh 归简体，ja、ko 各自归并，不认识的归英文。
 */
export function normalizeLanguage(raw: string): Lang {
  const l = raw.trim().replaceAll('_', '-').toLowerCase()
  if (l === 'zh-tw' || l === 'zh-hk' || l === 'zh-mo' || l.startsWith('zh-hant')) return 'zh-TW'
  if (l === 'zh' || l.startsWith('zh-')) return 'zh-CN'
  if (l === 'ja' || l.startsWith('ja-')) return 'ja'
  if (l === 'ko' || l.startsWith('ko-')) return 'ko'
  return DEFAULT_LANG
}

/** 判断一个语言标识是不是五种之一的"家族"：中文、日文、韩文、英文 */
function isSupportedFamily(raw: string): boolean {
  const l = raw.trim().replaceAll('_', '-').toLowerCase()
  return ['zh', 'ja', 'ko', 'en'].some((family) => l === family || l.startsWith(`${family}-`))
}

/**
 * 按优先顺序从一组候选语言（比如 navigator.languages）里挑第一个支持的，归并成五种之一。
 * 不能直接对第一个候选做归并：用户的首选是法语、第二是日语时，应该给他日语而不是英文。
 */
export function pickLanguage(candidates: readonly string[]): Lang {
  const first = candidates.find(isSupportedFamily)
  return first === undefined ? DEFAULT_LANG : normalizeLanguage(first)
}
