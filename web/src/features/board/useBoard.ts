// 看板快照的查询，以及页面要用的派生数据（排好序的列、每列的卡片、按 id 查卡片）。

import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'
import { getSnapshot } from '@/lib/api/endpoints'
import type { CardView, Column, Snapshot } from '@/lib/api/types'
import { queryKeys } from '@/lib/queryKeys'

/** 读取整板快照；boardId 为 null（地址里的编号不合法）时不发请求 */
export function useBoardSnapshot(boardId: number | null) {
  return useQuery({
    queryKey: queryKeys.board(boardId ?? 0),
    queryFn: ({ signal }) => getSnapshot(boardId as number, signal),
    enabled: boardId !== null,
  })
}

/** 页面渲染用的派生数据 */
export interface BoardView {
  /** 按显示顺序排好的列 */
  columns: Column[]
  /** 按卡片 id 查找 */
  cardsById: Map<number, CardView>
}

/** 由快照算出排好序的列和按 id 的卡片表；快照没变时结果不重算 */
export function useBoardView(snapshot: Snapshot | undefined): BoardView {
  return useMemo(() => {
    const columns = [...(snapshot?.columns ?? [])].sort((a, b) => a.sort - b.sort || a.id - b.id)
    const cardsById = new Map((snapshot?.cards ?? []).map((card) => [card.id, card]))
    return { columns, cardsById }
  }, [snapshot])
}
