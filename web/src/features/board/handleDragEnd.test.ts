import { describe, expect, it } from 'vitest'
import { moveToColumn } from './dnd'
import { handleDragEnd } from './handleDragEnd'

describe('handleDragEnd', () => {
  it('跨列移动时产生 columnId', () => {
    const original = { 1: [1, 2], 2: [3] }
    const during = moveToColumn(original, 1, 2, 0)
    const { plan } = handleDragEnd(during, original, 1, 3, false)
    expect(plan?.columnId).toBe(2)
  })

  it('没有落点时不发 plan', () => {
    const original = { 1: [1, 2], 2: [3] }
    const { plan } = handleDragEnd(original, original, 1, null, false)
    expect(plan).toBeNull()
  })
})