// 退出登录时通知 Service Worker 清掉离线缓存，免得下一个人用这台设备还能离线看到上一个人的看板。

/** 让 Service Worker 清空它缓存的页面、静态资源和看板数据；没有 Service Worker 时什么也不做 */
export function clearOfflineCaches(): void {
  if (!('serviceWorker' in navigator)) return
  navigator.serviceWorker.controller?.postMessage({ type: 'clear-caches' })
}
