// 所有测试共用的初始化：加载 jest-dom 的断言扩展，并在每个测试后清理页面和本地存储，保证测试互不影响。
import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'

afterEach(() => {
  cleanup()
  localStorage.clear()
})
