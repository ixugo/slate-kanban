// 任务详情「评论和活动」的数据整理：把评论和这个任务的事件合成一条按时间从新到旧的时间线。

import type { BoardEvent, Comment } from '@/lib/api/types'

/** 时间线上要显示的任务事件类型：建卡、修改、换列、认领、释放、归档、自动化。
 * 不含评论事件（评论本身已经列出，重复了）和任务内顺序调整（太碎）。 */
export const CARD_EVENT_TYPES = ['card.created', 'card.updated', 'card.moved', 'card.claimed', 'card.released', 'card.archived', 'automation.fired'] as const

/** 时间线上的一项：一条评论，或一条事件 */
export type TimelineItem = { kind: 'comment'; at: string; id: number; comment: Comment } | { kind: 'event'; at: string; id: number; event: BoardEvent }

/** 合并评论和事件，按时间从新到旧；时间相同时事件在评论之后，保证排序稳定。showEvents 为假时只留评论。
 * 正文为空的评论只是服务内部的标识（只带扩展字段），没有内容可看，不列出 */
export function buildTimeline(comments: readonly Comment[], events: readonly BoardEvent[], showEvents: boolean): TimelineItem[] {
  const items: TimelineItem[] = comments.filter((comment) => comment.body.trim() !== '').map((comment) => ({ kind: 'comment', at: comment.created_at, id: comment.id, comment }))
  if (showEvents) for (const event of events) items.push({ kind: 'event', at: event.created_at, id: event.id, event })
  const rank = (i: TimelineItem) => (i.kind === 'comment' ? 0 : 1)
  return items.sort((a, b) => {
    const diff = Date.parse(b.at) - Date.parse(a.at)
    if (diff !== 0 && !Number.isNaN(diff)) return diff
    return rank(a) - rank(b) || b.id - a.id
  })
}
