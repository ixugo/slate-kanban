// 看板的结构化筛选（仿 Trello 的「筛选」弹层）：点选的条件拼成和搜索框同一套筛选语法，
// 网页、MCP 和自动化规则因此共用服务端的同一个解析器，不会出现两套结果不一致的逻辑。
// 全部是不依赖界面的纯函数，方便单独测试。

import type { CardView } from '@/lib/api/types'

/** 优先级的取值，和筛选语法里 priority: 后面的名字一致 */
export const PRIORITY_NAMES = ['high', 'medium', 'low', 'none'] as const
export type PriorityName = (typeof PRIORITY_NAMES)[number]

/** 状态开关，每一项对应 is:xxx；选了多项表示同时满足 */
export const STATE_FLAGS = ['open', 'done', 'free', 'claimed', 'waiting', 'human'] as const
export type StateFlag = (typeof STATE_FLAGS)[number]

/** 截止日期的单选，any 表示不限 */
export const DUE_CHOICES = ['any', 'overdue', 'today', 'week', 'has', 'none'] as const
export type DueChoice = (typeof DUE_CHOICES)[number]

/** 选了多个标签时的关系：任一，或者全部 */
export type TagMode = 'any' | 'all'

/** 「7 天内到期」往后看几天，从今天算起，含当天 */
const DUE_WEEK_DAYS = 7

/** 互斥的状态：选了其中一个，另一个就取消，因为同时满足两个只会得到空列表 */
const OPPOSITE_STATE: Partial<Record<StateFlag, StateFlag>> = {
  open: 'done',
  done: 'open',
  free: 'claimed',
  claimed: 'free',
}

/**
 * 弹层里点选出来的全部条件。同一组里的多个取值是「任一」（标签可以改成「全部」），
 * 不同组之间、以及状态开关之间都是「同时满足」。
 */
export interface BoardFilters {
  /** 选中的标签，写法取自卡片上的原样 */
  tags: string[]
  tagMode: TagMode
  /** 只看没有标签的卡片；和选标签互斥 */
  noTags: boolean
  /** 选中的列 id */
  columns: number[]
  priorities: PriorityName[]
  /** 选中的领取者（agent 名称） */
  claimers: string[]
  states: StateFlag[]
  due: DueChoice
}

/** 什么都没选 */
export const EMPTY_FILTERS: BoardFilters = {
  tags: [],
  tagMode: 'any',
  noTags: false,
  columns: [],
  priorities: [],
  claimers: [],
  states: [],
  due: 'any',
}

/** 标签选项：名称加上带这个标签的卡片数 */
export interface TagOption {
  name: string
  count: number
}

/** 一个取值能不能写进筛选语法：双引号会被当成引号吃掉，竖线会被当成「多选一」，两者都没法按字面表达 */
export function isFilterable(value: string): boolean {
  return value !== '' && !value.includes('"') && !value.includes('|')
}

/** 取值里有空白时用双引号包起来，筛选语法里引号内的空白不会把词切开 */
function quoteValue(value: string): string {
  return /\s/.test(value) ? `"${value}"` : value
}

/** 本地时间的日期，写成筛选语法认的 YYYY-MM-DD */
export function formatLocalDate(date: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

/** 截止日期单选对应的筛选词 */
function dueTerms(choice: DueChoice, now: Date): string[] {
  switch (choice) {
    case 'overdue':
      return ['is:overdue']
    case 'today':
      return ['due:today']
    case 'week': {
      const last = new Date(now.getFullYear(), now.getMonth(), now.getDate() + DUE_WEEK_DAYS)
      return ['due:>=today', `due:<=${formatLocalDate(last)}`]
    }
    case 'has':
      return ['has:due']
    case 'none':
      return ['no:due']
    default:
      return []
  }
}

/**
 * 把点选的条件换成筛选词（语法见服务端 card_filter.go）。
 * 写不进语法的取值（含双引号或竖线）直接跳过，免得拼出和用户意图不同的条件。now 只用来算「7 天内」。
 */
export function buildFilterTerms(filters: BoardFilters, now: Date = new Date()): string[] {
  const terms: string[] = []
  const tags = filters.tags.filter(isFilterable).map(quoteValue)
  if (tags.length > 0) {
    if (filters.tagMode === 'all') terms.push(...tags.map((tag) => `tag:${tag}`))
    else terms.push(`tag:${tags.join('|')}`)
  }
  if (filters.noTags) terms.push('no:tags')
  if (filters.columns.length > 0) terms.push(`col:${filters.columns.join('|')}`)
  if (filters.priorities.length > 0) terms.push(`priority:${filters.priorities.join('|')}`)
  const claimers = filters.claimers.filter(isFilterable).map(quoteValue)
  if (claimers.length > 0) terms.push(`claimed:${claimers.join('|')}`)
  terms.push(...filters.states.map((state) => `is:${state}`))
  terms.push(...dueTerms(filters.due, now))
  return terms
}

/** 把搜索框里的关键字和点选出来的筛选词合成一条查询，空白和空项都去掉 */
export function joinQuery(keyword: string, terms: readonly string[]): string {
  return [keyword.trim(), ...terms].filter((part) => part !== '').join(' ')
}

/** 选中了多少个条件：每个选中的取值算一个，截止不限不算；弹层按钮上的角标用它 */
export function countFilters(filters: BoardFilters): number {
  return (
    filters.tags.length +
    (filters.noTags ? 1 : 0) +
    filters.columns.length +
    filters.priorities.length +
    filters.claimers.length +
    filters.states.length +
    (filters.due === 'any' ? 0 : 1)
  )
}

/** 列表里有这个值就去掉，没有就加到末尾 */
function toggle<T>(list: readonly T[], value: T, same: (a: T, b: T) => boolean = (a, b) => a === b): T[] {
  return list.some((item) => same(item, value)) ? list.filter((item) => !same(item, value)) : [...list, value]
}

/** 点一个标签：选上或取消（不区分大小写），选上时取消「没有标签」 */
export function toggleTag(filters: BoardFilters, tag: string): BoardFilters {
  const same = (a: string, b: string) => a.toLowerCase() === b.toLowerCase()
  return { ...filters, tags: toggle(filters.tags, tag, same), noTags: false }
}

/** 点「没有标签」：选上时清掉已选的标签 */
export function toggleNoTags(filters: BoardFilters): BoardFilters {
  const noTags = !filters.noTags
  return { ...filters, noTags, tags: noTags ? [] : filters.tags }
}

/** 点一列：选上或取消 */
export function toggleColumn(filters: BoardFilters, columnId: number): BoardFilters {
  return { ...filters, columns: toggle(filters.columns, columnId) }
}

/** 点一个优先级：选上或取消 */
export function togglePriority(filters: BoardFilters, priority: PriorityName): BoardFilters {
  return { ...filters, priorities: toggle(filters.priorities, priority) }
}

/** 点一个领取者：选上或取消 */
export function toggleClaimer(filters: BoardFilters, name: string): BoardFilters {
  return { ...filters, claimers: toggle(filters.claimers, name) }
}

/** 点一个状态：选上或取消；选上时去掉和它互斥的状态 */
export function toggleState(filters: BoardFilters, state: StateFlag): BoardFilters {
  if (filters.states.includes(state)) return { ...filters, states: filters.states.filter((s) => s !== state) }
  const opposite = OPPOSITE_STATE[state]
  return { ...filters, states: [...filters.states.filter((s) => s !== opposite), state] }
}

/**
 * 整理出弹层里的标签选项：按带这个标签的卡片数从多到少排，同样多按名称排。
 * 大小写不同的算同一个标签（服务端匹配标签不区分大小写），写法取第一次出现的；写不进语法的标签不列出。
 * 复杂度 O(n·t + k log k)，n 是卡片数，t 是每张卡片的标签数，k 是标签种数。
 */
export function collectTagOptions(cards: readonly CardView[]): TagOption[] {
  const byKey = new Map<string, TagOption>()
  for (const card of cards) {
    for (const tag of card.tags) {
      if (!isFilterable(tag)) continue
      const key = tag.toLowerCase()
      const option = byKey.get(key)
      if (option) option.count += 1
      else byKey.set(key, { name: tag, count: 1 })
    }
  }
  return [...byKey.values()].sort((a, b) => b.count - a.count || (a.name < b.name ? -1 : a.name > b.name ? 1 : 0))
}

/** 当前正在领取（租约有效）卡片的 agent 名称，去重后按名称排序；写不进语法的名称不列出 */
export function collectClaimers(cards: readonly CardView[]): string[] {
  const names = new Set<string>()
  for (const card of cards) {
    if (card.claim_active && isFilterable(card.claimed_by)) names.add(card.claimed_by)
  }
  return [...names].sort()
}
