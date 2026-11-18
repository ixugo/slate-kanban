// 所有测试共用的初始化：加载 jest-dom 的断言扩展，补上 jsdom 缺少的浏览器接口（Radix 组件会用到），
// 预加载五种语言包，并在每个测试后清理页面、本地存储和界面语言，保证测试互不影响。
import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach, beforeAll, beforeEach } from 'vitest'
import { preloadAllLanguages, setLanguage } from '@/i18n'
import { installFakeEventSource } from './fakeEventSource'

// jsdom 没有实现下面这些接口，Radix 的下拉菜单、选择框、弹窗在交互时会调用它们。
Element.prototype.hasPointerCapture ??= () => false
Element.prototype.setPointerCapture ??= () => {}
Element.prototype.releasePointerCapture ??= () => {}
Element.prototype.scrollIntoView ??= () => {}
// 没有 matchMedia 时按"不匹配"处理；需要特定结果的测试自己用 vi.stubGlobal 替换。
if (typeof window.matchMedia !== 'function') {
  window.matchMedia = (query: string) =>
    ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener() {},
      removeEventListener() {},
      addListener() {},
      removeListener() {},
      dispatchEvent: () => false,
    }) as MediaQueryList
}
globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
}

beforeAll(async () => {
  await preloadAllLanguages()
})

// jsdom 没有 EventSource：每个测试前装上假的，进入看板页的流程不会因为缺它而崩溃；需要手动推事件的测试自己再装一次拿到实例。
beforeEach(() => {
  installFakeEventSource()
})

afterEach(async () => {
  cleanup()
  localStorage.clear()
  document.documentElement.removeAttribute('class')
  await setLanguage('en')
})