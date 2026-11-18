import { describe, expect, it } from 'vitest'
import { CARD_TAGS_MAX, CARD_TAG_MAX } from '@/lib/limits'
import { applyTag, normalizeTagInput, popTag, removeTag } from './tagList'

describe('tagList', () => {
  it('添加标签会 trim、去重且不区分大小写', () => {
    const first = applyTag([], '  Bug  ')
    expect(first).toEqual({ ok: true, tags: ['Bug'] })
    const dup = applyTag(['Bug'], 'bug')
    expect(dup).toEqual({ ok: false, reason: 'duplicate' })
  })

  it('达到数量上限时拒绝添加', () => {
    const tags = Array.from({ length: CARD_TAGS_MAX }, (_, i) => `t${i}`)
    expect(applyTag(tags, 'extra')).toEqual({ ok: false, reason: 'limit' })
  })

  it('单标签长度按 CARD_TAG_MAX 截断', () => {
    const long = 'a'.repeat(CARD_TAG_MAX + 5)
    expect(normalizeTagInput(long).length).toBe(CARD_TAG_MAX)
  })

  it('删除与退格删最后一枚', () => {
    expect(removeTag(['a', 'b'], 'a')).toEqual(['b'])
    expect(popTag(['a', 'b'])).toEqual(['a'])
  })
})