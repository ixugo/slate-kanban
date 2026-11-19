// 看板列收起状态：按项目+列读写 localStorage，完成列默认收起。

import { useCallback, useMemo, useState } from 'react'
import type { Column } from '@/lib/api/types'

/** 存储键前缀，完整的键是 slate.col.<项目 id>.<列 id>，值 '1' 表示收起、'0' 表示展开 */
const KEY_PREFIX = 'slate.col'

/** 生成偏好存储键 */
export function columnCollapseKey(boardId: number, columnId: number): string {
  return `${KEY_PREFIX}.${boardId}.${columnId}`
}

/** 没有保存过偏好时，完成列默认收起，其他列默认展开 */
export function defaultCollapsedForColumn(column: Column): boolean {
  return column.is_done
}

/** localStorage 写入失败（隐私模式、配额满）时的内存备份：本次打开期间仍然记得，刷新后才丢 */
const memoryFallback = new Map<string, boolean>()

/** 已经提示过的警告：读写失败在每次渲染都会遇到，同一条只打一次，避免刷屏 */
const warned = new Set<string>()

/** 同一条警告只打印一次；保留错误对象方便排查 */
function warnOnce(message: string, err: unknown): void {
  if (warned.has(message)) return
  warned.add(message)
  console.warn(message, err)
}

/** 读一列的偏好：先看内存备份，再看 localStorage，都没有就用默认值 */
function readCollapsed(boardId: number, column: Column): boolean {
  const key = columnCollapseKey(boardId, column.id)
  const remembered = memoryFallback.get(key)
  if (remembered !== undefined) return remembered
  try {
    const raw = localStorage.getItem(key)
    if (raw === '1') return true
    if (raw === '0') return false
  } catch (err) {
    warnOnce('无法读取列收起偏好，改用默认值', err)
  }
  return defaultCollapsedForColumn(column)
}

/** 写一列的偏好；写失败就放进内存备份，让本次打开期间的行为保持一致 */
function persistCollapsed(boardId: number, columnId: number, collapsed: boolean): void {
  const key = columnCollapseKey(boardId, columnId)
  try {
    localStorage.setItem(key, collapsed ? '1' : '0')
    memoryFallback.delete(key)
  } catch (err) {
    warnOnce('无法保存列收起偏好，刷新后会恢复默认', err)
    memoryFallback.set(key, collapsed)
  }
}

export interface CollapsedColumnsHandle {
  /** 某一列当前是否收起 */
  isCollapsed: (columnId: number) => boolean
  /** 点窄边或按键盘：展开这一列 */
  expandColumn: (columnId: number) => void
  /** 点列头的收起按钮：收起这一列 */
  collapseColumn: (columnId: number) => void
}

/**
 * 管理一个项目里各列的展开/收起。
 * 只记录用户在当前页面手动切换过的列；没切换过的列每次渲染时按存储值和默认值现算，
 * 所以列增删、改成完成列都不需要额外同步。
 */
export function useCollapsedColumns(boardId: number, columns: Column[]): CollapsedColumnsHandle {
  const [overrides, setOverrides] = useState<Record<string, boolean>>({})
  const columnsById = useMemo(() => new Map(columns.map((c) => [c.id, c])), [columns])

  const isCollapsed = useCallback(
    (columnId: number) => {
      const override = overrides[columnCollapseKey(boardId, columnId)]
      if (override !== undefined) return override
      const column = columnsById.get(columnId)
      return column ? readCollapsed(boardId, column) : false
    },
    [boardId, overrides, columnsById],
  )

  const setCollapsed = useCallback(
    (columnId: number, collapsed: boolean) => {
      setOverrides((prev) => ({ ...prev, [columnCollapseKey(boardId, columnId)]: collapsed }))
      persistCollapsed(boardId, columnId, collapsed)
    },
    [boardId],
  )

  const expandColumn = useCallback((columnId: number) => setCollapsed(columnId, false), [setCollapsed])
  const collapseColumn = useCallback((columnId: number) => setCollapsed(columnId, true), [setCollapsed])

  return { isCollapsed, expandColumn, collapseColumn }
}
