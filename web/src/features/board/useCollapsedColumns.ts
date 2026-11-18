// 看板列收起状态：按看板+列读写 localStorage，完成列默认收起。

import { useCallback, useEffect, useMemo, useState } from 'react'
import type { Column } from '@/lib/api/types'

/** 生成偏好存储键 */
export function columnCollapseKey(boardId: number, columnId: number): string {
  return `slate.col.${boardId}.${columnId}`
}

/** 没有保存过偏好时，完成列默认收起 */
export function defaultCollapsedForColumn(column: Column): boolean {
  return column.is_done
}

/** localStorage 不可用时的内存备份（模块级，测试间用 clear 或 mock） */
const memoryFallback = new Map<string, boolean>()

function readCollapsed(boardId: number, column: Column): boolean {
  const key = columnCollapseKey(boardId, column.id)
  try {
    const raw = localStorage.getItem(key)
    if (raw === '1') return true
    if (raw === '0') return false
    return defaultCollapsedForColumn(column)
  } catch (err) {
    console.warn('无法读取列收起偏好', err)
    return memoryFallback.get(key) ?? defaultCollapsedForColumn(column)
  }
}

function persistCollapsed(boardId: number, columnId: number, collapsed: boolean): void {
  const key = columnCollapseKey(boardId, columnId)
  try {
    localStorage.setItem(key, collapsed ? '1' : '0')
    memoryFallback.delete(key)
  } catch (err) {
    console.warn('无法保存列收起偏好', err)
    memoryFallback.set(key, collapsed)
  }
}

export interface CollapsedColumnsHandle {
  /** 某一列当前是否收起 */
  isCollapsed: (columnId: number) => boolean
  /** 列头收起按钮：在展开与收起之间切换 */
  toggleCollapsed: (columnId: number) => void
  /** 窄边点击或键盘操作：强制展开 */
  expandColumn: (columnId: number) => void
  /** 列头收起：强制收起 */
  collapseColumn: (columnId: number) => void
}

/**
 * 管理各列展开/收起偏好。
 * 列集合变化时重新合并本地已有键，避免新列没有状态。
 */
export function useCollapsedColumns(boardId: number, columns: Column[]): CollapsedColumnsHandle {
  const columnSig = useMemo(
    () => columns.map((c) => `${c.id}:${c.is_done ? 1 : 0}`).join('|'),
    [columns],
  )
  const [collapsedById, setCollapsedById] = useState<Record<number, boolean>>(() => {
    const next: Record<number, boolean> = {}
    for (const col of columns) {
      next[col.id] = readCollapsed(boardId, col)
    }
    return next
  })

  useEffect(() => {
    const next: Record<number, boolean> = {}
    for (const col of columns) {
      next[col.id] = readCollapsed(boardId, col)
    }
    setCollapsedById(next)
  }, [boardId, columnSig, columns])

  const setCollapsed = useCallback(
    (columnId: number, collapsed: boolean) => {
      setCollapsedById((prev) => ({ ...prev, [columnId]: collapsed }))
      persistCollapsed(boardId, columnId, collapsed)
    },
    [boardId],
  )

  const isCollapsed = useCallback(
    (columnId: number) => collapsedById[columnId] ?? false,
    [collapsedById],
  )

  const toggleCollapsed = useCallback(
    (columnId: number) => setCollapsed(columnId, !isCollapsed(columnId)),
    [isCollapsed, setCollapsed],
  )

  const expandColumn = useCallback((columnId: number) => setCollapsed(columnId, false), [setCollapsed])
  const collapseColumn = useCallback((columnId: number) => setCollapsed(columnId, true), [setCollapsed])

  return { isCollapsed, toggleCollapsed, expandColumn, collapseColumn }
}