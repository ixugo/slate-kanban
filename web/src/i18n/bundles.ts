// 各语言包的加载：英文静态打进主包，其余四种走独立异步 chunk。

import type { Lang } from './languages'
import type { MessageTree } from './types'
import { en } from './locales/en'

/** 非英文语言的动态加载器；构建后每种语言单独一个 chunk */
const loaders: Record<Exclude<Lang, 'en'>, () => Promise<MessageTree>> = {
  'zh-CN': async () => (await import('./locales/zh-CN')).zhCN,
  'zh-TW': async () => (await import('./locales/zh-TW')).zhTW,
  ja: async () => (await import('./locales/ja')).ja,
  ko: async () => (await import('./locales/ko')).ko,
}

const cache = new Map<Lang, MessageTree>([['en', en]])

/** 取已加载的文案树；未加载的非英文语言会抛错，调用方应先 await loadLanguage */
export function getBundle(lang: Lang): MessageTree {
  const bundle = cache.get(lang)
  if (!bundle) throw new Error(`语言包尚未加载: ${lang}`)
  return bundle
}

/** 加载并缓存语言包；英文立即返回，已缓存的也不再请求 */
export async function loadLanguage(lang: Lang): Promise<void> {
  if (lang === 'en' || cache.has(lang)) return
  const messages = await loaders[lang]()
  cache.set(lang, messages)
}

/** 测试用：并行预加载全部五种语言，保证 t 在跑用例时同步可用 */
export async function preloadAllLanguages(): Promise<void> {
  await Promise.all((Object.keys(loaders) as Exclude<Lang, 'en'>[]).map((code) => loadLanguage(code)))
}

/** 测试用：导出已缓存的全部语言，用来核对键是否一致 */
export function getCachedBundles(): ReadonlyMap<Lang, MessageTree> {
  return cache
}