// 提示条的外部 store：模块内订阅者集合，供 Toaster 与 toast API 共享同一份列表。

/** 提示类型，对应不同默认时长和样式 */
export type ToastVariant = 'default' | 'success' | 'error' | 'info' | 'warning'

/** 一条提示在 store 里的形状 */
export interface ToastRecord {
  id: string | number
  title: string
  description?: string
  variant: ToastVariant
  duration: number
  createdAt: number
}

const DEFAULT_MS = 4000
const ERROR_MS = 8000

let seq = 1
let items: ToastRecord[] = []
/** useSyncExternalStore 要求 getSnapshot 在未变更时返回同一引用 */
let snapshot: readonly ToastRecord[] = items
const listeners = new Set<() => void>()

function notify(): void {
  for (const l of listeners) l()
}

/** useSyncExternalStore 订阅 */
export function subscribeToasts(onStoreChange: () => void): () => void {
  listeners.add(onStoreChange)
  return () => listeners.delete(onStoreChange)
}

/** 当前列表快照 */
export function getToastsSnapshot(): readonly ToastRecord[] {
  return snapshot
}

/** 各变体默认自动消失时长 */
export function defaultDuration(variant: ToastVariant): number {
  return variant === 'error' ? ERROR_MS : DEFAULT_MS
}

export interface ToastInput {
  title: string
  description?: string
  variant?: ToastVariant
  duration?: number
  id?: string | number
}

/** 新增或按 id 更新一条提示；返回这条的 id */
export function pushToast(input: ToastInput): string | number {
  const variant = input.variant ?? 'default'
  const id = input.id ?? seq++
  const duration = input.duration ?? defaultDuration(variant)
  const next: ToastRecord = {
    id,
    title: input.title,
    description: input.description,
    variant,
    duration,
    createdAt: Date.now(),
  }
  const idx = items.findIndex((t) => t.id === id)
  items = idx >= 0 ? items.map((t, i) => (i === idx ? { ...t, ...next } : t)) : [...items, next]
  snapshot = items
  notify()
  return id
}

/** 按 id 移除；不传 id 时清空全部 */
export function dismissToast(id?: string | number): void {
  items = id === undefined ? [] : items.filter((t) => t.id !== id)
  snapshot = items
  notify()
}