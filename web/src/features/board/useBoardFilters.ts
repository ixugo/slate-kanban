// 看板的点选筛选状态：只属于记下它的那块看板。换到另一块看板时是什么都没选，
// 不把上一块看板的列 id 和标签带过去，因为它们只在原来的看板里有意义。

import { useCallback, useState } from 'react'
import { EMPTY_FILTERS, type BoardFilters } from './filters'

/** 状态属于哪块看板 */
interface Held {
  boardId: number | null
  filters: BoardFilters
}

/** 点选筛选的状态，以及整体替换和清空的方法 */
export function useBoardFilters(boardId: number | null) {
  const [held, setHeld] = useState<Held>({ boardId, filters: EMPTY_FILTERS })
  const filters = held.boardId === boardId ? held.filters : EMPTY_FILTERS
  const setFilters = useCallback((next: BoardFilters) => setHeld({ boardId, filters: next }), [boardId])
  const clear = useCallback(() => setHeld({ boardId, filters: EMPTY_FILTERS }), [boardId])
  return { filters, setFilters, clear }
}
