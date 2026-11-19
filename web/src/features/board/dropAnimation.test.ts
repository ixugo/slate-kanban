import type { DropAnimationKeyframeResolver, DropAnimationSideEffects } from '@dnd-kit/core'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { alignToSlot, createDropAnimation, prefersReducedMotion, settleOverlay } from './dropAnimation'

type KeyframeArgs = Parameters<DropAnimationKeyframeResolver>[0]
type EffectArgs = Parameters<DropAnimationSideEffects>[0]

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('alignToSlot', () => {
  it('终点按外层容器对齐：容器的左上角落到真实卡片的左上角', () => {
    const args = {
      transform: { initial: { x: 10, y: 20, scaleX: 1, scaleY: 1 }, final: { x: 0, y: 0, scaleX: 1, scaleY: 1 } },
      // 真实卡片在 (300, 80)
      active: { rect: { left: 300, top: 80 } },
      // 外层容器现在在 (100, 50)；容器里放大倾斜后的卡片外框更大，但不影响容器自己的位置
      dragOverlay: { node: { getBoundingClientRect: () => ({ left: 100, top: 50 }) } },
    } as unknown as KeyframeArgs

    const [from, to] = alignToSlot(args)

    expect(from.transform).toBe('translate3d(10px, 20px, 0) scaleX(1) scaleY(1)')
    // x：10 + (300 - 100)，y：20 + (80 - 50)
    expect(to.transform).toBe('translate3d(210px, 50px, 0) scaleX(1) scaleY(1)')
  })
})

describe('settleOverlay', () => {
  /** 一个假的浮起卡片，记录它被调用了什么动画 */
  function makeOverlay() {
    const node = document.createElement('div')
    const card = document.createElement('div')
    const animate = vi.fn()
    Object.assign(card, { animate })
    node.append(card)
    return { node, animate }
  }

  it('动画期间藏起真实卡片，结束后恢复它原来的样式', () => {
    const active = document.createElement('div')
    active.style.visibility = 'visible'
    const { node } = makeOverlay()

    const cleanup = settleOverlay({ active: { node: active }, dragOverlay: { node } } as unknown as EffectArgs)
    expect(active.style.visibility).toBe('hidden')

    ;(cleanup as () => void)()
    expect(active.style.visibility).toBe('visible')
  })

  it('浮起的卡片把放大、倾斜和阴影收回平放的样子，并停在终点', () => {
    const { node, animate } = makeOverlay()

    settleOverlay({ active: { node: document.createElement('div') }, dragOverlay: { node } } as unknown as EffectArgs)

    expect(animate).toHaveBeenCalledTimes(1)
    const [keyframes, options] = animate.mock.calls[0]
    expect(keyframes[1]).toEqual({ scale: '1', rotate: '0deg', boxShadow: 'var(--shadow-card)' })
    expect(options).toMatchObject({ fill: 'forwards' })
  })

  it('浏览器不支持 Element.animate 时只藏卡片，不报错', () => {
    const node = document.createElement('div')
    node.append(document.createElement('div'))
    const active = document.createElement('div')

    expect(() => settleOverlay({ active: { node: active }, dragOverlay: { node } } as unknown as EffectArgs)).not.toThrow()
    expect(active.style.visibility).toBe('hidden')
  })
})

describe('createDropAnimation / prefersReducedMotion', () => {
  it('默认有时长；开了减少动态效果时时长为 0（dnd-kit 遇到 0 就不播放动画）', () => {
    expect(createDropAnimation(false)).toMatchObject({ duration: 300, keyframes: alignToSlot, sideEffects: settleOverlay })
    expect(createDropAnimation(true)).toMatchObject({ duration: 0 })
  })

  it('读系统的减少动态效果设置；没有 matchMedia 时按没开处理', () => {
    vi.stubGlobal('matchMedia', (query: string) => ({ matches: query.includes('reduce') }))
    expect(prefersReducedMotion()).toBe(true)

    vi.stubGlobal('matchMedia', undefined)
    expect(prefersReducedMotion()).toBe(false)
  })
})
