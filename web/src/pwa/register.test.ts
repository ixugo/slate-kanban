import { afterEach, describe, expect, it, vi } from 'vitest'

/** 每次测试前重新加载 register 模块，便于改 import.meta.env */
async function loadRegister() {
  vi.resetModules()
  return import('./register')
}

describe('registerServiceWorker', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it('开发环境不注册', async () => {
    vi.stubEnv('PROD', false)
    const register = vi.fn()
    vi.stubGlobal('navigator', { serviceWorker: { register } })
    const { registerServiceWorker } = await loadRegister()
    registerServiceWorker()
    expect(register).not.toHaveBeenCalled()
  })

  it('不支持 serviceWorker 时不注册', async () => {
    vi.stubEnv('PROD', true)
    vi.stubGlobal('navigator', {})
    const { registerServiceWorker } = await loadRegister()
    registerServiceWorker()
    expect(navigator.serviceWorker).toBeUndefined()
  })

  it('非安全上下文不注册', async () => {
    vi.stubEnv('PROD', true)
    const register = vi.fn()
    vi.stubGlobal('navigator', { serviceWorker: { register } })
    vi.stubGlobal('isSecureContext', false)
    const { registerServiceWorker } = await loadRegister()
    registerServiceWorker()
    expect(register).not.toHaveBeenCalled()
  })

  it('注册失败只 warn 不抛错', async () => {
    vi.stubEnv('PROD', true)
    vi.stubGlobal('isSecureContext', true)
    const register = vi.fn().mockRejectedValue(new Error('fail'))
    vi.stubGlobal('navigator', { serviceWorker: { register } })
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const { registerServiceWorker } = await loadRegister()
    expect(() => registerServiceWorker()).not.toThrow()
    await Promise.resolve()
    expect(register).toHaveBeenCalledWith('/sw.js')
    expect(warn).toHaveBeenCalled()
  })
})