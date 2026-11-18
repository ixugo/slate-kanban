// 登录令牌的存取。令牌放在 localStorage：页面同源、没有第三方脚本。
// 本地存储不可用时（比如浏览器的隐私模式禁用了它）退回内存，只在当前页面有效，刷新后需要重新登录。

/** localStorage 里存令牌的键 */
export const TOKEN_KEY = 'slate.token'

const listeners = new Set<() => void>()
let memoryToken = ''
let storageBroken = false

// 读取本地存储里的令牌；存储不可用时记下来，以后都用内存里的值。
function readStored(): string {
  if (storageBroken) return memoryToken
  try {
    return localStorage.getItem(TOKEN_KEY) ?? ''
  } catch {
    storageBroken = true
    return memoryToken
  }
}

// 写入或删除本地存储里的令牌；存储不可用时改用内存。
function writeStored(token: string): void {
  memoryToken = token
  if (storageBroken) return
  try {
    if (token) localStorage.setItem(TOKEN_KEY, token)
    else localStorage.removeItem(TOKEN_KEY)
  } catch {
    storageBroken = true
  }
}

// 通知所有订阅者令牌变了，页面据此重新渲染或跳转。
function notify(): void {
  listeners.forEach((fn) => fn())
}

/** 返回当前令牌，没有登录时是空串 */
export function getToken(): string {
  return readStored()
}

/** 保存令牌并通知订阅者 */
export function setToken(token: string): void {
  writeStored(token)
  notify()
}

/** 清除令牌并通知订阅者，页面会回到登录页 */
export function clearToken(): void {
  writeStored('')
  notify()
}

/**
 * 订阅令牌变化，返回取消订阅的函数。
 * 同时监听 storage 事件：别的标签页退出登录或换了令牌，这个标签页也要跟着变。
 */
export function subscribeToken(listener: () => void): () => void {
  listeners.add(listener)
  const onStorage = (e: StorageEvent) => {
    if (e.key === TOKEN_KEY || e.key === null) listener()
  }
  window.addEventListener('storage', onStorage)
  return () => {
    listeners.delete(listener)
    window.removeEventListener('storage', onStorage)
  }
}
