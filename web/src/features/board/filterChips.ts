// 把点选的条件整理成「当前筛选」那一行小标签：每个选中的取值一个，点标签上的叉就取消这个取值。

import type { TFunction } from '@/i18n/react'
import type { Column } from '@/lib/api/types'
import { tagText } from '@/features/labels/tagName'
import {
  toggleClaimer,
  toggleColumn,
  toggleNoTags,
  togglePriority,
  toggleState,
  toggleTag,
  type BoardFilters,
} from './filters'

/** 一个「当前筛选」小标签：label 是显示的文字，next 是取消它之后的全部条件 */
export interface FilterChip {
  key: string
  label: string
  next: BoardFilters
}

/** 按弹层里分组的顺序，列出当前选中的每个取值；columns 用来把列 id 换成列名 */
export function describeFilters(filters: BoardFilters, columns: readonly Column[], t: TFunction): FilterChip[] {
  const chip = (group: string, value: string) => t('filter.chip', { group, value })
  const tagGroup =
    filters.tagMode === 'all' && filters.tags.length > 1 ? t('filter.groups.tagsAll') : t('filter.groups.tags')
  const chips: FilterChip[] = []
  for (const tag of filters.tags) {
    chips.push({ key: `tag:${tag}`, label: chip(tagGroup, tagText(tag, t)), next: toggleTag(filters, tag) })
  }
  if (filters.noTags) {
    chips.push({ key: 'noTags', label: chip(tagGroup, t('filter.noTags')), next: toggleNoTags(filters) })
  }
  for (const id of filters.columns) {
    const name = columns.find((c) => c.id === id)?.name ?? `#${id}`
    chips.push({ key: `col:${id}`, label: chip(t('filter.groups.columns'), name), next: toggleColumn(filters, id) })
  }
  for (const state of filters.states) {
    chips.push({
      key: `is:${state}`,
      label: chip(t('filter.groups.status'), t(`filter.states.${state}`)),
      next: toggleState(filters, state),
    })
  }
  for (const priority of filters.priorities) {
    chips.push({
      key: `priority:${priority}`,
      label: chip(t('filter.groups.priority'), t(`filter.priority.${priority}`)),
      next: togglePriority(filters, priority),
    })
  }
  for (const name of filters.claimers) {
    chips.push({ key: `claimed:${name}`, label: chip(t('filter.groups.claimedBy'), name), next: toggleClaimer(filters, name) })
  }
  if (filters.due !== 'any') {
    chips.push({
      key: 'due',
      label: chip(t('filter.groups.due'), t(`filter.due.${filters.due}`)),
      next: { ...filters, due: 'any' },
    })
  }
  return chips
}
