import { closestCorners, type ClientRect, type Collision, type DroppableContainer } from '@dnd-kit/core'
import { describe, expect, it } from 'vitest'
import { boardCollision } from './collision'

type Args = Parameters<typeof boardCollision>[0]

/** 造一个矩形：左上角 (left, top)，宽 width，高 height */
function rect(left: number, top: number, width: number, height: number): ClientRect {
  return { left, top, width, height, right: left + width, bottom: top + height }
}

interface Entry {
  id: string | number
  data: Record<string, unknown>
  rect: ClientRect
}

/** 整理成碰撞检测的入参：落点的 data 就是 useDroppable 里传的那份 */
function makeArgs(entries: Entry[], collisionRect: ClientRect, pointer: { x: number; y: number } | null): Args {
  const containers = entries.map((e) => ({ id: e.id, data: { current: e.data }, disabled: false }))
  return {
    active: { id: 99 },
    collisionRect,
    droppableRects: new Map(entries.map((e) => [e.id, e.rect])),
    droppableContainers: containers as unknown as DroppableContainer[],
    pointerCoordinates: pointer,
  } as unknown as Args
}

const firstId = (collisions: Collision[]) => collisions[0]?.id

// 场景取自真实看板：「进行中」里一个任务（宽 268、高 75），右边隔一列是又高又窄的收起窄边（宽 48、高 578）。
// 拖动的任务浮层（宽 268）中心压在窄边中心上，但四个角离「进行中」那个任务自己的位置更近，所以只看四个角永远选不中窄边。
const card: Entry = { id: 5, data: {}, rect: rect(322, 196, 268, 75) }
const railData = { type: 'column', columnId: 3, collapsed: true }
const rail: Entry = { id: 'column-3', data: railData, rect: rect(608, 126, 48, 578) }
const floating = rect(497, 378, 268, 75)
const onRail = { x: 632, y: 415 }

describe('boardCollision', () => {
  it('前提：只看四个角时，选中的是任务而不是窄边', () => {
    expect(firstId(closestCorners(makeArgs([card, rail], floating, onRail)))).toBe(5)
  })

  it('指针落在收起的窄边上时，窄边就是落点', () => {
    expect(firstId(boardCollision(makeArgs([card, rail], floating, onRail)))).toBe('column-3')
  })

  it('指针不在窄边上时，按四个角离得最近的来选', () => {
    expect(firstId(boardCollision(makeArgs([card, rail], floating, { x: 500, y: 415 })))).toBe(5)
  })

  it('键盘拖动没有指针坐标，按四个角离得最近的来选', () => {
    expect(firstId(boardCollision(makeArgs([card, rail], floating, null)))).toBe(5)
  })

  it('展开的列不享受指针优先：指针落在它上面时，仍然选四个角最近的任务', () => {
    const open: Entry = { ...rail, data: { ...railData, collapsed: false } }

    expect(firstId(boardCollision(makeArgs([card, open], floating, onRail)))).toBe(5)
  })
})
