// 多语言入口：英文静态打包，其余语言按需加载；对外形状接近原 i18next，方便测试和渐进迁移。

import { getCachedBundles, loadLanguage, preloadAllLanguages } from './bundles'
import { initialLanguage, initRuntimeLanguage, getCurrentLanguage, getFixedT, getT, setLanguage, subscribe } from './runtime'

export { initialLanguage, setLanguage, loadLanguage, preloadAllLanguages, getCachedBundles }
export type { Lang } from './languages'
export { LANGUAGES, LANG_STORAGE_KEY, DEFAULT_LANG, normalizeLanguage, pickLanguage } from './languages'

// 模块加载时先定下语言代码；非英文文案由 main 或测试 setup 在首屏渲染前加载。
initRuntimeLanguage(initialLanguage())

/** 与旧 i18n 默认导出兼容的对象：language、t、changeLanguage、getFixedT */
const i18n = {
  get language() {
    return getCurrentLanguage()
  },
  t: getT(),
  changeLanguage: setLanguage,
  getFixedT,
}

// 语言切换后 t 的引用要变，否则 useCallback 依赖 t 的组件不会刷新。
subscribe(() => {
  i18n.t = getT()
})

export default i18n