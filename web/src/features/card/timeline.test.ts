import { describe, expect, it } from 'vitest'
import type { BoardEvent, Comment } from '@/lib/api/types'
import { buildTimeline } from './timeline'

/** 造一条评论 */
const comment = (id: number, at: string): Comment => ({
  id,
  board_id: 1,
  card_id: 1,
  author_kind: 'human',
  author: 'me',
  body: `c${id}`,
  created_at: at,
  updated_at: at,
})

/** 造一条事件 */
const event = (id: number, at: string): BoardEvent => ({
  id,
  board_id: 1,
  card_id: 1,
  type: 'card.moved',
  actor_kind: 'human',
  actor: 'me',
  payload: {},
  created_at: at,
})

const describeIds = (items: ReturnType<typeof buildTimeline>) => items.map((i) => `${i.kind[0]}${i.id}`)

describe('buildTimeline', () => {
  it('评论和事件按时间从新到旧混排', () => {
    const items = buildTimeline(
      [comment(1, '2026-11-01T10:00:00Z'), comment(2, '2026-11-01T12:00:00Z')],
      [event(7, '2026-11-01T11:00:00Z'), event(8, '2026-11-01T13:00:00Z')],
      true,
    )
    expect(describeIds(items)).toEqual(['e8', 'c2', 'e7', 'c1'])
  })

  it('正文为空的内部标识评论不进时间线', () => {
    const marker = { ...comment(2, '2026-11-01T11:00:00Z'), body: '' }
    const items = buildTimeline([comment(1, '2026-11-01T10:00:00Z'), marker], [], true)
    expect(describeIds(items)).toEqual(['c1'])
  })

  it('不显示详情时只剩评论', () => {
    const items = buildTimeline([comment(1, '2026-11-01T10:00:00Z')], [event(7, '2026-11-01T11:00:00Z')], false)
    expect(describeIds(items)).toEqual(['c1'])
  })

  it('时间相同：评论排在事件前面，同类按编号从大到小', () => {
    const at = '2026-11-01T10:00:00Z'
    const items = buildTimeline([comment(1, at), comment(2, at)], [event(5, at), event(6, at)], true)
    expect(describeIds(items)).toEqual(['c2', 'c1', 'e6', 'e5'])
  })

  it('没有内容返回空数组，不改动传入的数组', () => {
    expect(buildTimeline([], [], true)).toEqual([])
    const comments = [comment(1, '2026-11-01T10:00:00Z'), comment(2, '2026-11-02T10:00:00Z')]
    buildTimeline(comments, [], true)
    expect(comments.map((c) => c.id)).toEqual([1, 2])
  })
})
