import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import vm from 'node:vm'

/** 在内存里模拟 CacheStorage */
function createFakeCaches() {
  const stores = new Map<string, Map<string, Response>>()

  return {
    stores,
    async open(name: string) {
      if (!stores.has(name)) stores.set(name, new Map())
      const bucket = stores.get(name)!
      return {
        async match(req: Request | string) {
          const key = typeof req === 'string' ? req : req.url
          return bucket.get(key) ?? undefined
        },
        async put(req: Request | string, res: Response) {
          const key = typeof req === 'string' ? req : req.url
          bucket.set(key, res)
        },
        async addAll(urls: string[]) {
          for (const u of urls) {
            bucket.set(u, new Response(`body:${u}`, { status: 200 }))
          }
        },
      }
    },
    async keys() {
      return [...stores.keys()]
    },
    async delete(name: string) {
      return stores.delete(name)
    },
    reset() {
      stores.clear()
    },
  }
}

/** 收集 self 上注册的 SW 事件回调 */
function createSwHarness() {
  const listeners = new Map<string, Set<(event: Record<string, unknown>) => void>>()
  const fakeCaches = createFakeCaches()
  let fetchImpl: (req: Request) => Promise<Response> = async () => new Response('net', { status: 200 })

  const self = {
    location: { origin: 'http://localhost:17374' },
    skipWaiting: vi.fn(),
    clients: { claim: vi.fn().mockResolvedValue(undefined) },
    addEventListener(type: string, cb: (event: Record<string, unknown>) => void) {
      if (!listeners.has(type)) listeners.set(type, new Set())
      listeners.get(type)!.add(cb)
    },
    caches: fakeCaches,
  }

  const swSource = readFileSync(join(import.meta.dirname, '../../public/sw.js'), 'utf8')
  const context = { self, caches: fakeCaches, fetch: (req: Request) => fetchImpl(req), Response, Request, Promise, setTimeout, clearTimeout, URL, console }
  vm.createContext(context)
  vm.runInContext(swSource, context)

  /** 触发某一类 SW 事件 */
  async function emit(type: string, event: Record<string, unknown>) {
    const handlers = listeners.get(type)
    if (!handlers) return
    for (const h of handlers) {
      h(event)
    }
    const wait = event.waitUntil as ((p: Promise<unknown>) => void) | undefined
    if (wait && '_promise' in event) {
      await event._promise
    }
  }

  /** 模拟一次 fetch 事件并返回 respondWith 的结果 */
  async function routeFetch(request: Request) {
    let responded: Promise<Response> | undefined
    const event = {
      request,
      respondWith(p: Promise<Response>) {
        responded = p
      },
    }
    await emit('fetch', event)
    if (!responded) {
      return fetchImpl(request)
    }
    return responded
  }

  return {
    fakeCaches,
    setFetchImpl(fn: typeof fetchImpl) {
      fetchImpl = fn
    },
    emit,
    routeFetch,
  }
}

describe('service worker sw.js', () => {
  let harness: ReturnType<typeof createSwHarness>

  beforeEach(() => {
    harness = createSwHarness()
  })

  it('只缓存白名单 GET API', async () => {
    harness.setFetchImpl(async () => new Response('boards', { status: 200 }))
    const boards = await harness.routeFetch(new Request('http://localhost:17374/api/v1/boards'))
    expect(await boards.text()).toBe('boards')

    harness.setFetchImpl(async () => {
      throw new Error('offline')
    })
    const cached = await harness.routeFetch(new Request('http://localhost:17374/api/v1/boards'))
    expect(await cached.text()).toBe('boards')
  })

  it('非白名单 API 不走缓存回退', async () => {
    let called = 0
    harness.setFetchImpl(async () => {
      called++
      throw new Error('offline')
    })
    await expect(
      harness.routeFetch(new Request('http://localhost:17374/api/v1/boards/1/cards')),
    ).rejects.toThrow()
    expect(called).toBe(1)
  })

  it('/assets 缓存优先', async () => {
    harness.setFetchImpl(async () => new Response('asset-v1', { status: 200 }))
    const first = await harness.routeFetch(new Request('http://localhost:17374/assets/app.js'))
    expect(await first.text()).toBe('asset-v1')

    harness.setFetchImpl(async () => new Response('asset-v2', { status: 200 }))
    const second = await harness.routeFetch(new Request('http://localhost:17374/assets/app.js'))
    expect(await second.text()).toBe('asset-v1')
  })

  it('401 不写缓存且不用旧缓存顶替', async () => {
    harness.setFetchImpl(async () => new Response('ok', { status: 200 }))
    await harness.routeFetch(new Request('http://localhost:17374/api/v1/boards/3/snapshot'))

    harness.setFetchImpl(async () => new Response('unauth', { status: 401 }))
    const res = await harness.routeFetch(new Request('http://localhost:17374/api/v1/boards/3/snapshot'))
    expect(res.status).toBe(401)
    expect(await res.text()).toBe('unauth')
  })

  it('SSE 与 /mcp 不经 SW 策略', async () => {
    let fetchCalls = 0
    harness.setFetchImpl(async () => {
      fetchCalls++
      return new Response('live')
    })
    const streamReq = new Request('http://localhost:17374/api/v1/boards/1/stream', {
      headers: { accept: 'text/event-stream' },
    })
    await harness.routeFetch(streamReq)
    expect(fetchCalls).toBe(1)

    const mcpReq = new Request('http://localhost:17374/mcp')
    await harness.routeFetch(mcpReq)
    expect(fetchCalls).toBe(2)
  })

  it('导航失败回退缓存的 /', async () => {
    await harness.emit('install', {
      waitUntil(p: Promise<unknown>) {
        this._promise = p
      },
    })
    harness.setFetchImpl(async () => {
      throw new Error('offline')
    })
    const nav = new Request('http://localhost:17374/b/1', {
      headers: { accept: 'text/html,application/xhtml+xml' },
    })
    const res = await harness.routeFetch(nav)
    expect(res.status).toBe(200)
    expect(await res.text()).toContain('body:/')
  })

  /** 触发 install 事件，把应用壳预缓存进假缓存 */
  async function installShell() {
    await harness.emit('install', {
      waitUntil(p: Promise<unknown>) {
        this._promise = p
      },
    })
  }

  /** 构造一个 HTML 页面响应 */
  function html(body: string) {
    return new Response(body, { status: 200, headers: { 'content-type': 'text/html; charset=utf-8' } })
  }

  it('安装说明、skill、健康检查和指标页的导航不走离线壳', async () => {
    await installShell()
    harness.setFetchImpl(async () => {
      throw new Error('offline')
    })
    for (const path of ['/install.md', '/skills/slate-kanban/SKILL.md', '/health', '/app/metrics/api']) {
      const nav = new Request(`http://localhost:17374${path}`, { headers: { accept: 'text/html' } })
      await expect(harness.routeFetch(nav), path).rejects.toThrow()
    }
  })

  it('导航成功后只更新 / 这一份壳，不按地址堆积缓存', async () => {
    await installShell()
    harness.setFetchImpl(async () => html('shell-1'))
    await harness.routeFetch(new Request('http://localhost:17374/b/1?card=5', { headers: { accept: 'text/html' } }))
    harness.setFetchImpl(async () => html('shell-2'))
    await harness.routeFetch(new Request('http://localhost:17374/b/2', { headers: { accept: 'text/html' } }))

    const shell = harness.fakeCaches.stores.get('slate-v1-shell')!
    expect([...shell.keys()].filter((k) => k.includes('/b/'))).toEqual([])
    expect(await shell.get('/')!.text()).toBe('shell-2')
  })

  it('不是 HTML 的导航响应不会覆盖离线壳', async () => {
    await installShell()
    harness.setFetchImpl(async () => new Response('# not a page', { status: 200, headers: { 'content-type': 'text/markdown' } }))
    await harness.routeFetch(new Request('http://localhost:17374/whatever', { headers: { accept: 'text/html' } }))
    const shell = harness.fakeCaches.stores.get('slate-v1-shell')!
    expect(await shell.get('/')!.text()).toBe('body:/')
  })

  it('服务端返回 401 时清掉只读 API 缓存，离线也读不到旧数据', async () => {
    harness.setFetchImpl(async () => new Response('private-board', { status: 200 }))
    await harness.routeFetch(new Request('http://localhost:17374/api/v1/boards/3/snapshot'))
    expect(harness.fakeCaches.stores.has('slate-v1-api')).toBe(true)

    harness.setFetchImpl(async () => new Response('unauth', { status: 401 }))
    await harness.routeFetch(new Request('http://localhost:17374/api/v1/boards/3/snapshot'))
    expect(harness.fakeCaches.stores.has('slate-v1-api')).toBe(false)

    harness.setFetchImpl(async () => {
      throw new Error('offline')
    })
    await expect(harness.routeFetch(new Request('http://localhost:17374/api/v1/boards/3/snapshot'))).rejects.toThrow()
  })

  it('clear-caches 清空 slate 缓存', async () => {
    harness.fakeCaches.stores.set('slate-v1-shell', new Map())
    harness.fakeCaches.stores.set('slate-v1-api', new Map())
    await harness.emit('message', {
      data: { type: 'clear-caches' },
      waitUntil(p: Promise<unknown>) {
        this._promise = p
      },
    })
    expect([...harness.fakeCaches.stores.keys()].some((k) => k.startsWith('slate-'))).toBe(false)
  })

  it('activate 删除旧版本缓存', async () => {
    harness.fakeCaches.stores.set('slate-v0-shell', new Map())
    harness.fakeCaches.stores.set('slate-v1-shell', new Map())
    await harness.emit('activate', {
      waitUntil(p: Promise<unknown>) {
        this._promise = p
      },
    })
    expect(harness.fakeCaches.stores.has('slate-v0-shell')).toBe(false)
    expect(harness.fakeCaches.stores.has('slate-v1-shell')).toBe(true)
  })
})