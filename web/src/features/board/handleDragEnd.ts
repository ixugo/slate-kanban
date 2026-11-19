// 把 dnd-kit 的松手事件换算成 planMove 并交给 useMoveCard；单独文件方便在 jsdom 里单测。

import { finalizeDrop, planMove, type Items, type MovePlan } from './dnd'

/** 一次拖拽结束的处理结果 */
export interface DragEndResult {
  /** 拖完之后的列内顺序 */
  items: Items
  /** 需要发给服务端的移动；null 表示原地放下或无效落点 */
  plan: MovePlan | null
}

/**
 * 根据活动卡片、落点以及拖动前后的 items 算出是否要发移动请求。
 * overId 为 null 表示没有有效落点。
 */
export function handleDragEnd(
  items: Items,
  original: Items,
  activeId: number,
  overId: number | null,
  overIsColumn: boolean,
): DragEndResult {
  if (overId === null) return { items, plan: null }
  const next = finalizeDrop(items, activeId, overId, overIsColumn)
  const plan = planMove(next, original, activeId)
  return { plan, items: next }
}