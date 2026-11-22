// 任务「日期」弹窗的纯逻辑：把任务上的开始、截止时间变成表单草稿，再把草稿变成要提交的改动。
// 开始时间只到“日”；截止时间到“分”。时间都是浏览器本地时间，提交前由调用方转成 RFC3339。

import { toLocalInput } from '@/lib/format'

/** 弹窗表单的草稿：每个日期一个勾选框，加日期（截止还有时刻） */
export interface DatesDraft {
  start: boolean
  startDate: string
  due: boolean
  dueDate: string
  dueTime: string
}

/** 任务上有没有设开始或截止日期 */
export function hasDates(card: { started_at: string | null; due_at: string | null }): boolean {
  return Boolean(card.started_at || card.due_at)
}

/** 本地今天的日期 YYYY-MM-DD */
export function todayLocal(now: Date = new Date()): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`
}

/** 用任务上现有的时间生成草稿；没有的那一项不勾选，日期留空 */
export function draftOf(card: { started_at: string | null; due_at: string | null }): DatesDraft {
  const [startDate = ''] = toLocalInput(card.started_at).split('T')
  const [dueDate = '', dueTime = ''] = toLocalInput(card.due_at).split('T')
  return {
    start: startDate !== '',
    startDate,
    due: dueDate !== '',
    dueDate,
    dueTime,
  }
}

/** 勾选时，开始日期补今天，截止日期和时间补本地明天的相同时分。 */
export function withDefaults(draft: DatesDraft, now: Date = new Date()): DatesDraft {
  const tomorrow = new Date(now)
  tomorrow.setDate(now.getDate() + 1)
  const time = toLocalInput(now.toISOString()).slice(11)
  return {
    ...draft,
    startDate: draft.start && !draft.startDate ? todayLocal(now) : draft.startDate,
    dueDate: draft.due && !draft.dueDate ? todayLocal(tomorrow) : draft.dueDate,
    dueTime: draft.due && !draft.dueTime ? time : draft.dueTime,
  }
}

/** 表单是否有问题：返回问题的类型，没问题返回 null。勾选了但日期为空，或截止日期早于开始日期 */
export function draftProblem(draft: DatesDraft): 'empty' | 'order' | null {
  if ((draft.start && draft.startDate === '') || (draft.due && (!draft.dueDate || !draft.dueTime))) return 'empty'
  if (draft.start && draft.due && draft.dueDate < draft.startDate) return 'order'
  return null
}

/**
 * 把草稿和任务现有的时间对比，只返回真正变了的字段；空串表示清除。没有任何变化返回空对象。
 * 开始日期没改时不重写：自动化设置的开始时间带具体时刻，用户没动日期就不能被抹成 00:00。
 */
export function datesPatch(card: { started_at: string | null; due_at: string | null }, draft: DatesDraft): Record<string, string> {
  const patch: Record<string, string> = {}
  const currentStart = toLocalInput(card.started_at)
  const nextStart = !draft.start ? '' : draft.startDate === currentStart.slice(0, 10) ? currentStart : `${draft.startDate}T00:00`
  if (nextStart !== currentStart) patch.started_at = nextStart
  const currentDue = toLocalInput(card.due_at)
  const nextDue = !draft.due ? '' : `${draft.dueDate}T${draft.dueTime}`
  if (nextDue !== currentDue) patch.due_at = nextDue
  return patch
}
