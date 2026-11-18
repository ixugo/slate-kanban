// 横向滚动的列画布，接上 dnd-kit 多容器拖拽。

import {
  DndContext,
  DragOverlay,
  KeyboardCode,
  KeyboardSensor,
  PointerSensor,
  TouchSensor,
  closestCorners,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragOverEvent,
  type DragStartEvent,
} from '@dnd-kit/core'
import { sortableKeyboardCoordinates } from '@dnd-kit/sortable'
import { useMemo, useRef, useState } from 'react'
import type { CardView, Column } from '@/lib/api/types'
import { buildItems, findColumnOf, moveToColumn, pickShownItems, type Items, type MovePlan, type Settling } from './dnd'
import { handleDragEnd } from './handleDragEnd'
import { createDropAnimation, prefersReducedMotion } from './dropAnimation'
import { ColumnView } from './ColumnView'
import { CardTile } from './CardTile'
import { useCollapsedColumns } from './useCollapsedColumns'

interface BoardCanvasProps {
  boardId: number
  columns: Column[]
  cardsById: Map<number, CardView>
  visibleIds: ReadonlySet<number> | null
  dragDisabled: boolean
  onMove: (cardId: number, plan: MovePlan) => void
  addColumnId: number | null
  onAddOpen: (columnId: number) => void
  onAddClose: () => void
  onAddCard: (columnId: number, title: string) => void
  addBusy?: boolean
}

/** 看板列与拖拽上下文 */
export function BoardCanvas({
  boardId,
  columns,
  cardsById,
  visibleIds,
  dragDisabled,
  onMove,
  addColumnId,
  onAddOpen,
  onAddClose,
  onAddCard,
  addBusy,
}: BoardCanvasProps) {
  const collapse = useCollapsedColumns(boardId, columns)
  const cards = useMemo(() => [...cardsById.values()], [cardsById])
  const baseItems = useMemo(() => buildItems(columns, cards, visibleIds), [columns, cards, visibleIds])
  const [dragItems, setDragItems] = useState<Items | null>(null)
  const [settling, setSettling] = useState<Settling | null>(null)
  const [activeId, setActiveId] = useState<number | null>(null)
  const originalRef = useRef<Items>(baseItems)
  const shownItems = pickShownItems(dragItems, settling, baseItems)
  const dropAnimation = useMemo(() => createDropAnimation(prefersReducedMotion()), [])

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
    useSensor(TouchSensor, { activationConstraint: { delay: 200, tolerance: 5 } }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
      keyboardCodes: {
        start: [KeyboardCode.Space],
        cancel: [KeyboardCode.Esc],
        end: [KeyboardCode.Space],
      },
    }),
  )

  const onDragStart = (e: DragStartEvent) => {
    const id = Number(e.active.id)
    originalRef.current = shownItems
    setSettling(null)
    setDragItems(shownItems)
    setActiveId(id)
  }

  const onDragOver = (e: DragOverEvent) => {
    const active = Number(e.active.id)
    const over = e.over
    if (!over || dragDisabled || dragItems === null) return
    const overId = Number(over.id)
    const overData = over.data.current as { type?: string; columnId?: number } | undefined
    const targetColumnId = overData?.columnId
    if (overData?.type === 'column' && targetColumnId !== undefined) {
      setDragItems((prev) => (prev ? moveToColumn(prev, active, targetColumnId, prev[targetColumnId]?.length ?? 0) : prev))
      return
    }
    const overColumn = findColumnOf(dragItems, overId)
    if (overColumn === null) return
    const index = dragItems[overColumn].indexOf(overId)
    setDragItems((prev) =>
      prev ? moveToColumn(prev, active, overColumn, index < 0 ? prev[overColumn].length : index) : prev,
    )
  }

  const endDrag = (restore: boolean) => {
    setActiveId(null)
    if (restore) setDragItems(null)
  }

  const onDragEnd = (e: DragEndEvent) => {
    const active = Number(e.active.id)
    const over = e.over
    const workItems = dragItems ?? baseItems
    if (!over || dragDisabled) {
      endDrag(true)
      return
    }
    const overId = Number(over.id)
    const overData = over.data.current as { type?: string } | undefined
    const { items: next, plan } = handleDragEnd(workItems, originalRef.current, active, overId, overData?.type === 'column')
    // 有移动时先保持松手时的排列，等乐观更新落地再换成服务端数据；
    // 否则这一瞬间卡片会退回旧位置，落下动画就飞向旧位置，再跳到新位置
    setSettling(plan ? { items: next, base: baseItems } : null)
    setDragItems(null)
    setActiveId(null)
    if (plan) onMove(active, plan)
  }

  const activeCard = activeId !== null ? cardsById.get(activeId) : undefined

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCorners}
      onDragStart={onDragStart}
      onDragOver={onDragOver}
      onDragEnd={onDragEnd}
      onDragCancel={() => endDrag(true)}
    >
      <div className="flex min-h-0 flex-1 snap-x snap-mandatory gap-3 overflow-x-auto px-4 pt-1 pb-4 sm:px-6">
        {columns.map((column, columnIndex) => {
          const ids = shownItems[column.id] ?? []
          const columnCards = ids.map((id) => cardsById.get(id)).filter((c): c is CardView => c !== undefined)
          return (
            <ColumnView
              key={column.id}
              column={column}
              cards={columnCards}
              cardIds={ids}
              addOpen={addColumnId === column.id}
              onAddOpen={() => onAddOpen(column.id)}
              onAddClose={onAddClose}
              onAddCard={(title) => onAddCard(column.id, title)}
              addBusy={addBusy}
              dragDisabled={dragDisabled}
              collapsed={collapse.isCollapsed(column.id)}
              columnIndex={columnIndex}
              columnCount={columns.length}
              onExpand={() => collapse.expandColumn(column.id)}
              onCollapse={() => collapse.collapseColumn(column.id)}
            />
          )
        })}
      </div>
      <DragOverlay dropAnimation={dropAnimation}>
        {activeCard ? <CardTile card={activeCard} dragDisabled overlay /> : null}
      </DragOverlay>
    </DndContext>
  )
}