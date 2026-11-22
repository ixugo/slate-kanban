// 看板页全局快捷键：n 新建、/ 搜索、Esc 清空；输入框有焦点时不触发单键。

import { useEffect, type RefObject } from 'react'

interface BoardKeyboardOptions {
  /** 打开或聚焦新建任务 */
  onNewCard: () => void
  /** 聚焦搜索框 */
  onFocusSearch: () => void
  /** Esc：关闭弹窗或清空搜索等 */
  onEscape: () => void
  searchRef: RefObject<HTMLInputElement | null>
  disabled?: boolean
}

/** 是否当前焦点在可编辑区域，若是则单键快捷键不触发 */
function isEditingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  const tag = target.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true
  return target.isContentEditable
}

/** 注册看板页快捷键，组件卸载时自动移除 */
export function useBoardKeyboard({ onNewCard, onFocusSearch, onEscape, searchRef, disabled }: BoardKeyboardOptions) {
  useEffect(() => {
    if (disabled) return
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.defaultPrevented || e.metaKey || e.ctrlKey || e.altKey) return
      if (isEditingTarget(e.target)) return
      if (e.key === 'n') {
        e.preventDefault()
        onNewCard()
      } else if (e.key === '/') {
        e.preventDefault()
        onFocusSearch()
        searchRef.current?.focus()
      } else if (e.key === 'Escape') {
        onEscape()
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [disabled, onEscape, onFocusSearch, onNewCard, searchRef])
}