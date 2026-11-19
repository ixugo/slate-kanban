/**
 * 生产环境注册 Service Worker；失败只打日志，不影响页面渲染。
 */

/** 仅在生产、支持 SW 且处于安全上下文（localhost 或 https）时注册 /sw.js */
export function registerServiceWorker(): void {
  if (!import.meta.env.PROD) {
    return
  }
  if (!('serviceWorker' in navigator)) {
    return
  }
  if (!window.isSecureContext) {
    return
  }

  void navigator.serviceWorker.register('/sw.js').catch((err: unknown) => {
    console.warn('Service Worker 注册失败，离线功能不可用', err)
  })
}