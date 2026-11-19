/* eslint-disable */
/**
 * Slate 离线 Service Worker：预缓存应用壳，只读缓存看板列表与快照接口。
 * 手写、无 Workbox，便于在 Vitest 里用假环境做路由决策测试。
 */

/** 缓存代数；升级时改此常量，activate 会删掉旧代号的缓存 */
const CACHE_VERSION = 'v1'

/** 所有 Slate 缓存名共用的前缀，便于批量清理 */
const CACHE_PREFIX = 'slate'

/** 应用壳与导航回退用的缓存桶 */
const SHELL_CACHE = `${CACHE_PREFIX}-${CACHE_VERSION}-shell`

/** 带哈希的 /assets/* 静态资源 */
const ASSETS_CACHE = `${CACHE_PREFIX}-${CACHE_VERSION}-assets`

/** 只读看板 API 的 GET 响应 */
const API_CACHE = `${CACHE_PREFIX}-${CACHE_VERSION}-api`

/** 导航请求等网络的超时，超时则回退离线壳 */
const NAV_TIMEOUT_MS = 3000

/** 安装阶段预缓存的 URL，保证离线能打开壳和 manifest/图标 */
const PRECACHE_URLS = [
  '/',
  '/theme-init.js',
  '/favicon.svg',
  '/manifest.webmanifest',
  '/icons/icon-192.png',
  '/icons/icon-512.png',
  '/icons/icon-maskable-512.png',
  '/apple-touch-icon.png',
]

/** 匹配「打开看板必需」的快照 GET */
const SNAPSHOT_PATH_RE = /^\/api\/v1\/boards\/\d+\/snapshot$/

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(SHELL_CACHE).then((cache) => cache.addAll(PRECACHE_URLS)).then(() => self.skipWaiting()),
  )
})

self.addEventListener('activate', (event) => {
  event.waitUntil(
    (async () => {
      await purgeOldCaches()
      await self.clients.claim()
    })(),
  )
})

self.addEventListener('message', (event) => {
  if (event.data && event.data.type === 'clear-caches') {
    event.waitUntil(clearAllSlateCaches())
  }
})

self.addEventListener('fetch', (event) => {
  const { request } = event
  if (request.method !== 'GET') {
    return
  }
  const url = new URL(request.url)
  if (url.origin !== self.location.origin) {
    return
  }
  if (shouldBypassSw(url, request)) {
    return
  }

  if (url.pathname.startsWith('/assets/')) {
    event.respondWith(cacheFirstAssets(request))
    return
  }

  if (isDocumentNavigation(request)) {
    event.respondWith(networkFirstNavigation(request))
    return
  }

  if (isCacheableApiGet(url.pathname)) {
    event.respondWith(networkFirstApi(request))
    return
  }
})

/**
 * 判断是否必须直连网络、不得读写 Slate 缓存（MCP、登录、SSE、非白名单 API，
 * 以及安装说明、skill、健康检查、指标页这些不是网页界面的地址）。
 */
function shouldBypassSw(url, request) {
  if (url.pathname === '/mcp' || url.pathname.startsWith('/mcp/')) {
    return true
  }
  if (url.pathname === '/install.md' || url.pathname === '/health') {
    return true
  }
  if (url.pathname.startsWith('/skills/') || url.pathname.startsWith('/app/')) {
    return true
  }
  if (url.pathname.startsWith('/api/v1/auth/')) {
    return true
  }
  if (url.pathname.endsWith('/stream')) {
    return true
  }
  const accept = request.headers.get('accept') || ''
  if (accept.includes('text/event-stream')) {
    return true
  }
  if (url.pathname.startsWith('/api/') && !isCacheableApiGet(url.pathname)) {
    return true
  }
  return false
}

/** 仅缓存看板列表与单板快照，供离线只读 */
function isCacheableApiGet(pathname) {
  return pathname === '/api/v1/boards' || SNAPSHOT_PATH_RE.test(pathname)
}

/** 判断响应是不是 HTML 页面 */
function isHtmlResponse(response) {
  return (response.headers.get('content-type') || '').includes('text/html')
}

/** 判断是否为页面导航（SPA 路由） */
function isDocumentNavigation(request) {
  return request.mode === 'navigate' || (request.headers.get('accept') || '').includes('text/html')
}

/**
 * /assets/*：缓存优先，构建产物带哈希可长期复用。
 */
async function cacheFirstAssets(request) {
  const cache = await caches.open(ASSETS_CACHE)
  const hit = await cache.match(request)
  if (hit) {
    return hit
  }
  const response = await fetch(request)
  if (response.ok) {
    await cache.put(request, response.clone())
  }
  return response
}

/**
 * 文档导航：先走网络，失败或超时则回退缓存的 `/`（index.html 壳）。
 * 所有网页路由返回的都是同一份壳，所以成功拿到 HTML 时只更新 `/` 这一份，
 * 不按地址（带卡片参数、搜索词的各种 URL）堆积缓存；不是 HTML 的响应绝不覆盖壳。
 */
async function networkFirstNavigation(request) {
  const shellCache = await caches.open(SHELL_CACHE)
  try {
    const response = await fetchWithTimeout(request, NAV_TIMEOUT_MS)
    if (response.ok && isHtmlResponse(response)) {
      await shellCache.put('/', response.clone())
    }
    return response
  } catch {
    const fallback = (await shellCache.match('/')) || (await shellCache.match('/index.html'))
    if (fallback) {
      return fallback
    }
    return new Response('Offline', { status: 503, statusText: 'Offline' })
  }
}

/**
 * 白名单 API：网络优先；2xx 才更新缓存；401 说明服务端不再认这个浏览器（改了口令或会话过期），
 * 此时清掉整个只读缓存，既不回退旧数据，也不让离线时还能看到已失去授权的看板内容。
 */
async function networkFirstApi(request) {
  const cache = await caches.open(API_CACHE)
  try {
    const response = await fetch(request)
    if (response.status === 401) {
      await caches.delete(API_CACHE)
      return response
    }
    if (response.ok) {
      await cache.put(request, response.clone())
    }
    return response
  } catch {
    const cached = await cache.match(request)
    if (cached) {
      return cached
    }
    throw new Error('network error and no cache')
  }
}

/** 带超时的 fetch，供导航策略使用 */
function fetchWithTimeout(request, ms) {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error('timeout')), ms)
    fetch(request)
      .then((res) => {
        clearTimeout(timer)
        resolve(res)
      })
      .catch((err) => {
        clearTimeout(timer)
        reject(err)
      })
  })
}

/** activate 时删除上一版本的 slate-* 缓存桶 */
async function purgeOldCaches() {
  const currentPrefix = `${CACHE_PREFIX}-${CACHE_VERSION}`
  const names = await caches.keys()
  await Promise.all(
    names
      .filter((name) => name.startsWith(`${CACHE_PREFIX}-`) && !name.startsWith(currentPrefix))
      .map((name) => caches.delete(name)),
  )
}

/** 响应登出等场景的 clear-caches 消息 */
async function clearAllSlateCaches() {
  const names = await caches.keys()
  await Promise.all(names.filter((name) => name.startsWith(`${CACHE_PREFIX}-`)).map((name) => caches.delete(name)))
}