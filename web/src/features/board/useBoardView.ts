// 看板 vs 列表视图偏好：按项目 id 读写 localStorage，默认看板视图。

import { useCallback, useState } from 'react'

/** 视图模式 */
export type BoardViewMode = 'board' | 'list'

/** localStorage 键前缀，完整键 slate.view.<boardId> */
const KEY_PREFIX = 'slate.view'

/** 读写出错时的内存备份 */
const memoryFallback = new Map<string, BoardViewMode>()

/** 同一条警告只打一次 */
const warned = new Set<string>()

/** 读写 localStorage 失败只警告一次 */
function warnOnce(message: string, err: unknown): void {
  if (warned.has(message)) return
  warned.add(message)
  console.warn(message, err)
}

/** 生成偏好键 */
export function boardViewStorageKey(boardId: number): string {
  return `${KEY_PREFIX}.${boardId}`
}

/** 从存储读视图；非法值或未设置时返回 board */
export function readBoardViewMode(boardId: number): BoardViewMode {
  const key = boardViewStorageKey(boardId)
  const remembered = memoryFallback.get(key)
  if (remembered) return remembered
  try {
    const raw = localStorage.getItem(key)
    if (raw === 'list') return 'list'
    if (raw === 'board') return 'board'
  } catch (err) {
    warnOnce('无法读取视图偏好，改用看板视图', err)
  }
  return 'board'
}

/** 写入视图偏好 */
export function persistBoardViewMode(boardId: number, mode: BoardViewMode): void {
  const key = boardViewStorageKey(boardId)
  try {
    localStorage.setItem(key, mode)
    memoryFallback.delete(key)
  } catch (err) {
    warnOnce('无法保存视图偏好，刷新后会恢复默认', err)
    memoryFallback.set(key, mode)
  }
}

export interface BoardViewModeHandle {
  mode: BoardViewMode
  setMode: (mode: BoardViewMode) => void
}

/**
 * 管理当前项目的看板/列表切换；初始值从 localStorage 读，切换时写回。
 * 看板 id 变了（在同一个页面组件里切换项目）就重新读那个项目的偏好，
 * 用「渲染时调整 state」的写法，避免先闪一下旧项目的视图。
 */
export function useBoardViewMode(boardId: number): BoardViewModeHandle {
  const [state, setState] = useState(() => ({ boardId, mode: readBoardViewMode(boardId) }))
  if (state.boardId !== boardId) {
    setState({ boardId, mode: readBoardViewMode(boardId) })
  }

  const setMode = useCallback(
    (next: BoardViewMode) => {
      setState({ boardId, mode: next })
      persistBoardViewMode(boardId, next)
    },
    [boardId],
  )

  return { mode: state.boardId === boardId ? state.mode : readBoardViewMode(boardId), setMode }
}
