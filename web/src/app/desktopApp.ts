/** 判断页面是否运行在 Slate 桌面客户端内嵌浏览器中。 */

declare global {
  interface Window {
    slateDesktopServer?: { port: number; openPath: string }
  }
}

/** 桌面客户端会在加载时注入 slateDesktopServer，网页版没有该字段。 */
export function isDesktopApp(): boolean {
  return window.slateDesktopServer != null
}