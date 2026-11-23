// 让旧版 Slate 的浏览器更新到这个脚本，随后注销离线 Service Worker。
self.addEventListener('install', (event) => {
  event.waitUntil(self.skipWaiting())
})

self.addEventListener('activate', (event) => {
  event.waitUntil((async () => {
    await Promise.all([
      caches.delete('slate-v1-shell'),
      caches.delete('slate-v1-assets'),
      caches.delete('slate-v1-api'),
    ])
    await self.registration.unregister()
  })())
})
