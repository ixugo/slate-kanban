// 任务松手后的落下动画：浮起的任务飞向它的新位置，放大、倾斜和阴影同时收回，
// 结束时和真实任务完全重合，换成真实任务的那一帧看不出跳变。

import type { DropAnimation, DropAnimationKeyframeResolver, DropAnimationSideEffects } from '@dnd-kit/core'
import { CSS } from '@dnd-kit/utilities'

/** 落下动画的时长（毫秒） */
const DROP_DURATION_MS = 300

/** 落下动画的曲线：快速靠近、轻柔停住，没有回弹 */
const DROP_EASING = 'cubic-bezier(0.32, 0.72, 0, 1)'

/**
 * 算出落下动画的位移：让外层容器的左上角正好落在真实任务的左上角。
 * dnd-kit 默认量的是容器里的任务本身，而浮起的任务被放大、倾斜过，外框比真实任务大几个像素，
 * 终点就会偏几个像素；外层容器没有缩放和倾斜，用它对齐才不会偏。
 */
export const alignToSlot: DropAnimationKeyframeResolver = ({ transform, active, dragOverlay }) => {
  const box = dragOverlay.node.getBoundingClientRect()
  const final = {
    ...transform.initial,
    x: transform.initial.x + (active.rect.left - box.left),
    y: transform.initial.y + (active.rect.top - box.top),
  }
  return [{ transform: CSS.Transform.toString(transform.initial) }, { transform: CSS.Transform.toString(final) }]
}

/**
 * 落下动画开始时：把真实任务先藏起来，让浮起的任务把放大、倾斜和阴影收回到平放的样子；动画结束后恢复真实任务。
 * 藏任务用 visibility 而不是 opacity：任务的 opacity 有 CSS 过渡，恢复时会从全透明慢慢淡入，看起来像闪了一下。
 */
export const settleOverlay: DropAnimationSideEffects = ({ active, dragOverlay }) => {
  const original = active.node.style.visibility
  active.node.style.visibility = 'hidden'
  const card = dragOverlay.node.firstElementChild
  if (card instanceof HTMLElement && typeof card.animate === 'function') {
    const lifted = getComputedStyle(card)
    card.animate(
      [
        { scale: lifted.scale, rotate: lifted.rotate, boxShadow: lifted.boxShadow },
        { scale: '1', rotate: '0deg', boxShadow: 'var(--shadow-card)' },
      ],
      { duration: DROP_DURATION_MS, easing: DROP_EASING, fill: 'forwards' },
    )
  }
  return () => {
    active.node.style.visibility = original
  }
}

/** 生成落下动画的配置；系统开了「减少动态效果」时时长为 0，任务直接换位，不飞行 */
export function createDropAnimation(reduceMotion: boolean): DropAnimation {
  return {
    duration: reduceMotion ? 0 : DROP_DURATION_MS,
    easing: DROP_EASING,
    keyframes: alignToSlot,
    sideEffects: settleOverlay,
  }
}

/** 系统是否开了「减少动态效果」；浏览器没有 matchMedia 时按没开处理 */
export function prefersReducedMotion(): boolean {
  return typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
}
