import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import vm from 'node:vm'
import { expect, it, vi } from 'vitest'

it('接替并注销旧 Service Worker，只删除 Slate 的 PWA 缓存', async () => {
  type Listener = (event: { waitUntil: (promise: Promise<unknown>) => void }) => void
  const listeners = new Map<string, Listener>()
  const remaining = new Set(['slate-v1-shell', 'slate-v1-assets', 'slate-v1-api', 'slate-other', 'another-app'])
  const caches = {
    keys: async () => [...remaining],
    delete: async (name: string) => remaining.delete(name),
    open: async () => ({ addAll: async () => {} }),
  }
  const self = {
    addEventListener: (name: string, listener: Listener) => {
      listeners.set(name, listener)
    },
    skipWaiting: vi.fn().mockResolvedValue(undefined),
    registration: { unregister: vi.fn().mockResolvedValue(true) },
    clients: { claim: vi.fn().mockResolvedValue(undefined) },
  }
  const script = readFileSync(join(import.meta.dirname, '../../public/sw.js'), 'utf8')
  vm.runInNewContext(script, { self, caches })

  async function dispatch(name: string) {
    let completion: Promise<unknown> | undefined
    listeners.get(name)?.({ waitUntil: (promise) => { completion = promise } })
    await completion
  }

  await dispatch('install')
  await dispatch('activate')

  expect(self.skipWaiting).toHaveBeenCalledOnce()
  expect([...remaining]).toEqual(['slate-other', 'another-app'])
  expect(self.registration.unregister).toHaveBeenCalledOnce()
  expect(listeners.has('fetch')).toBe(false)
})
