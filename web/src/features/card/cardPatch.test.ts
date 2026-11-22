import { describe, expect, it } from 'vitest'
import type { Tag } from '@/lib/api/types'
import { makeCard } from '@/test/factories'
import { applyCardPatch, clearClaim } from './cardPatch'

const library: Tag[] = [
  { id: 1, text: 'bug', color: 'red-2' },
  { id: 2, text: '', color: 'green-1' },
  { id: 3, text: 'ui', color: '' },
]

describe('applyCardPatch', () => {
  it('只改出现的字段，其余原样', () => {
    const card = makeCard({ id: 1, title: '旧', priority: 1, branch: 'a' })
    const next = applyCardPatch(card, { title: '新', priority: 3 }, library)
    expect(next).toMatchObject({ title: '新', priority: 3, branch: 'a' })
    expect(card.title).toBe('旧')
  })

  it('时间传空串表示清除', () => {
    const card = makeCard({ id: 1, due_at: '2030-01-01T00:00:00Z', started_at: null })
    const next = applyCardPatch(card, { due_at: '', started_at: '2030-02-01T00:00:00Z' }, library)
    expect(next.due_at).toBeNull()
    expect(next.started_at).toBe('2030-02-01T00:00:00Z')
  })

  it('改标签时 tags 文字按标签库推出，无文字的标签不出现', () => {
    const card = makeCard({ id: 1, tag_ids: [], tags: [] })
    const next = applyCardPatch(card, { tag_ids: [1, 2, 3] }, library)
    expect(next.tag_ids).toEqual([1, 2, 3])
    expect(next.tags).toEqual(['bug', 'ui'])
  })

  it('不带 tags 字段的对象不会凭空多出 tags', () => {
    const { tags: _tags, ...plain } = makeCard({ id: 1 })
    const next = applyCardPatch(plain, { tag_ids: [1] }, library)
    expect('tags' in next).toBe(false)
  })
})

describe('clearClaim', () => {
  it('清掉领取者、租约和工程师，并标记不再被领取', () => {
    const card = makeCard({ id: 1, claimed_by: 'bot', claim_expires_at: '2030-01-01T00:00:00Z', claim_engineer_id: 2, claim_active: true })
    expect(clearClaim(card)).toMatchObject({ claimed_by: '', claim_expires_at: null, claim_engineer_id: 0, claim_active: false })
  })
})
