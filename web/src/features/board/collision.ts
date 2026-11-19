// 看板的拖拽碰撞检测：决定拖动中的卡片此刻落在哪个落点上。

import { closestCorners, pointerWithin, type CollisionDetection, type DroppableContainer } from '@dnd-kit/core'

/** 这个落点是不是收起成窄边的列：ColumnView 在登记落点时用 data.collapsed 标记 */
function isRail(container: DroppableContainer): boolean {
  return (container.data.current as { collapsed?: boolean } | undefined)?.collapsed === true
}

/**
 * 默认按「拖动卡片的四个角离哪个落点最近」选落点，但收起的窄边又窄又高（48 像素宽、整列那么高），
 * 浮起的卡片比它宽得多，四个角永远离旁边的卡片更近，用四个角几乎选不中它。
 * 所以先看指针：指针落在某个窄边上，就直接选它；否则（包括键盘拖动，没有指针坐标）回到四个角的算法。
 * 复杂度 O(落点数)，一块看板的落点是几十个。
 */
export const boardCollision: CollisionDetection = (args) => {
  const onRail = pointerWithin({ ...args, droppableContainers: args.droppableContainers.filter(isRail) })
  return onRail.length > 0 ? onRail : closestCorners(args)
}
