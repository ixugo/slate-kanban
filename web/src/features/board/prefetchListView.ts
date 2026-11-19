// 预取列表视图的代码：它是按需加载的，不预取的话，只在看板视图下用过的人离线时切换到列表会加载失败。

/** 打开看板后等多久再预取，让首屏渲染和接口请求先走完（毫秒） */
export const LIST_PREFETCH_DELAY_MS = 1500

/** 下载列表视图的代码；Service Worker 会把它缓存下来，离线时切到列表视图也能正常显示 */
export function prefetchListView(): void {
  void import('./ListView')
}
