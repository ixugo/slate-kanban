import type { MouseEvent } from 'react'

declare global {
  interface Window {
    _wails?: { invoke?: (message: string) => void }
  }
}

/** 从 macOS 桌面顶栏自身的空白按住左键后移动，才发起原生窗口拖动。 */
export function startWindowDrag(event: MouseEvent<HTMLElement>) {
  if (event.button !== 0 || event.target !== event.currentTarget || !document.documentElement.hasAttribute('data-slate-mac')) return
  // 桌面端加载独立 HTTP 服务的页面，Wails 注入了消息桥，但没有加载完整的前端拖动脚本。
  const bridge = window._wails
  if (!bridge?.invoke) return
  event.preventDefault()

  // 复用原生拖动：只有左键仍按着的首次移动才发送消息；普通点击、失焦和下一次按键取消候选手势。
  // 监听窗口而不是顶栏，鼠标从空白区移入正文或控件时也能继续这次拖动。
  const gesture = new AbortController()
  const cancel = () => gesture.abort()
  const options = { capture: true, once: true, signal: gesture.signal }
  window.addEventListener('mousemove', (move) => {
    cancel()
    if (move.buttons & 1) bridge.invoke?.('wails:drag')
  }, options)
  window.addEventListener('mouseup', cancel, options)
  window.addEventListener('mousedown', cancel, options)
  window.addEventListener('blur', cancel, options)
}
