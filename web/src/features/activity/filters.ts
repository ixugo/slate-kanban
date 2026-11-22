// 活动流的筛选条件：事件类型按人看得懂的分组点选（可多选），操作者单选；条件最后换算成服务端的查询参数。

import type { ActivityActor, ActivityQuery } from '@/lib/api/types'

/** 事件分组：每一组对应服务端的若干个事件类型，完整的类型清单见 .cursor/scopes/kanban.event.go.md */
export const EVENT_GROUPS = {
  move: ['card.moved', 'card.reordered'],
  card: ['card.created', 'card.updated', 'card.tags_updated', 'card.deleted', 'card.archived'],
  claim: ['card.claimed', 'card.released'],
  comment: ['comment.added', 'comment.deleted'],
  automation: ['automation.fired'],
  setup: [
    'board.created',
    'board.updated',
    'board.labels_updated',
    'column.created',
    'column.updated',
    'column.deleted',
    'column.reordered',
    'rule.created',
    'rule.updated',
    'rule.deleted',
  ],
} as const satisfies Record<string, readonly string[]>

export type EventGroup = keyof typeof EVENT_GROUPS

/** 分组在界面上的显示顺序，也是筛选条件里分组的规范顺序 */
export const EVENT_GROUP_ORDER = Object.keys(EVENT_GROUPS) as EventGroup[]

/** 活动流默认不要的事件：设置标签很琐碎，服务端照常记录和广播，只是网页读活动流时用 exclude_types 把它们挡掉 */
export const HIDDEN_EVENT_TYPES = ['card.tags_updated', 'board.labels_updated'] as const

/** 活动流的筛选条件 */
export interface ActivityFilters {
  /** 选中的事件分组，空表示不限 */
  groups: readonly EventGroup[]
  /** 选中的操作者，null 表示不限 */
  actor: ActivityActor | null
}

/** 什么都不筛 */
export const NO_FILTERS: ActivityFilters = { groups: [], actor: null }

/** 点一下某个事件分组：选中的取消，没选的加上。结果按规范顺序排列，同一组条件始终对应同一个缓存键 */
export function toggleGroup(filters: ActivityFilters, group: EventGroup): ActivityFilters {
  const selected = filters.groups.includes(group)
    ? filters.groups.filter((g) => g !== group)
    : [...filters.groups, group]
  return { ...filters, groups: EVENT_GROUP_ORDER.filter((g) => selected.includes(g)) }
}

/** 两个操作者是不是同一个：类型和名称都一样才算 */
export function isSameActor(a: ActivityActor | null, b: ActivityActor | null): boolean {
  return a === b || (a !== null && b !== null && a.kind === b.kind && a.name === b.name)
}

/** 已启用的筛选项个数（事件类型算一项，操作者算一项） */
export function countFilters(filters: ActivityFilters): number {
  return (filters.groups.length > 0 ? 1 : 0) + (filters.actor === null ? 0 : 1)
}

/** 换算成服务端的查询参数：选中分组里的类型合在一起，没选就不传；设置标签的事件永远排除 */
export function toQuery(filters: ActivityFilters): Pick<ActivityQuery, 'types' | 'exclude_types' | 'actor_kind' | 'actor'> {
  const types: string[] = filters.groups.flatMap((g) => EVENT_GROUPS[g])
  return {
    types: types.length > 0 ? types : undefined,
    exclude_types: HIDDEN_EVENT_TYPES,
    actor_kind: filters.actor?.kind,
    actor: filters.actor?.name,
  }
}
