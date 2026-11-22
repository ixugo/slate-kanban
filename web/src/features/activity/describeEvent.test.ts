import { readFileSync } from 'node:fs'
import path from 'node:path'
import { describe, expect, it } from 'vitest'
import i18n from '@/i18n'
import { makeCard } from '@/test/factories'
import type { BoardEvent } from '@/lib/api/types'
import { actorLabel, describeEvent, eventCard } from './describeEvent'

const tEn = i18n.getFixedT('en')
const tZh = i18n.getFixedT('zh-CN')

// 后端定义的全部事件类型，从 Go 源码里读出来，保证每一种网页都认识
const eventSource = readFileSync(path.resolve(import.meta.dirname, '../../../../internal/core/kanban/event.go'), 'utf8')
const serverTypes = [...eventSource.matchAll(/^\s*Event\w+\s*=\s*"([a-z_]+\.[a-z_]+)"/gm)].map((m) => m[1])

// 造一条事件，只写关心的字段
function ev(type: string, payload: Record<string, unknown> = {}, extra: Partial<BoardEvent> = {}): BoardEvent {
  return {
    id: 1,
    board_id: 1,
    card_id: 0,
    type,
    actor_kind: 'human',
    actor: 'human',
    payload,
    created_at: '2026-11-18T08:00:00Z',
    ...extra,
  }
}

describe('describeEvent', () => {
  const cases: [string, Record<string, unknown>, string][] = [
    ['board.created', {}, 'created the project'],
    ['board.updated', { fields: ['name'] }, 'edited the project settings'],
    ['column.created', { name: '待办' }, 'added column “待办”'],
    ['column.updated', { name: '待办' }, 'edited column “待办”'],
    ['column.deleted', { name: '待办' }, 'deleted column “待办”'],
    ['column.reordered', {}, 'reordered the columns'],
    ['card.created', { column: '待办' }, 'created the task in “待办”'],
    ['card.updated', { fields: ['title'] }, 'changed Title'],
    ['card.updated', { fields: ['priority', 'due_at'] }, 'changed Priority, Due date'],
    ['card.updated', { fields: ['human_only', 'waits_on', 'url', 'started_at', 'finished_at'] }, 'changed Human only, Dependencies, Link, Start time, Finish time'],
    ['card.updated', { fields: ['mystery'] }, 'changed mystery'],
    ['card.updated', {}, 'edited the task'],
    ['card.tags_updated', { fields: ['tags'] }, 'changed the tags'],
    ['board.labels_updated', { fields: ['labels'] }, 'changed the project labels'],
    ['card.moved', { from: '待办', to: '进行中' }, 'moved the task from “待办” to “进行中”'],
    ['card.reordered', {}, 'reordered the task within its column'],
    ['card.deleted', {}, 'deleted the task'],
    ['card.claimed', {}, 'claimed the task'],
    ['card.released', { why: 'released', agent: 'codex' }, 'released the task'],
    ['card.released', { why: 'revoked', agent: 'codex' }, 'took the task back from codex'],
    ['card.released', { why: 'moved', agent: 'codex' }, 'the task moved, so the claim by codex ended'],
    ['comment.added', { snippet: '收到' }, 'commented'],
    ['comment.deleted', {}, 'deleted a comment'],
    ['rule.created', { name: '开始计时' }, 'added rule “开始计时”'],
    ['rule.updated', { name: '开始计时' }, 'edited rule “开始计时”'],
    ['rule.deleted', { name: '开始计时' }, 'deleted rule “开始计时”'],
    ['automation.fired', { actions: [], skipped: [] }, 'ran the rule'],
  ]

  it.each(cases)('%s %j → %s', (type, payload, expected) => {
    expect(describeEvent(ev(type, payload), tEn).text).toBe(expected)
  })

  it('后端的每个事件类型都有专门的句子，不会落到「做了一些操作」', () => {
    for (const type of serverTypes) {
      const { text } = describeEvent(ev(type), tEn)
      expect(text, type).not.toContain('did something')
      expect(text, type).not.toContain('{{')
    }
  })

  it('评论的摘要放在补充说明里', () => {
    expect(describeEvent(ev('comment.added', { snippet: '已经接上了' }), tEn).detail).toBe('已经接上了')
  })

  it('规则触发列出执行的动作（按当前语言翻译），有跳过的再补一句', () => {
    const fired = ev('automation.fired', { actions: ['set_started_at', 'add_comment'], skipped: ['move_to_column：目标列不存在'] })
    expect(describeEvent(fired, tEn).detail).toBe('Set started time, Add comment · 1 skipped')
    expect(describeEvent(fired, tZh).detail).toContain('、')
    expect(describeEvent(ev('automation.fired', { actions: ['set_started_at'], skipped: [] }), tEn).detail).toBe('Set started time')
    expect(describeEvent(ev('automation.fired', { actions: [], skipped: ['x：y'] }), tEn).detail).toBe('1 skipped')
  })

  it('不认识的动作类型原样显示', () => {
    expect(describeEvent(ev('automation.fired', { actions: ['teleport'] }), tEn).detail).toBe('teleport')
  })

  it('按当前语言出句子', () => {
    expect(describeEvent(ev('card.moved', { from: 'A', to: 'B' }), tZh).text).toBe('将任务从「A」移到「B」')
  })

  it('改了描述的事件带变更记录编号，其他事件和不合法的值都没有', () => {
    const changed = ev('card.updated', { fields: ['description'], description_change_id: 7 })
    expect(describeEvent(changed, tEn)).toEqual({ text: 'changed Description', detail: '', descriptionChangeId: 7 })
    expect(describeEvent(ev('card.updated', { fields: ['title'] }), tEn).descriptionChangeId).toBeUndefined()
    for (const bad of [0, -1, 1.5, '7', null, true]) {
      expect(describeEvent(ev('card.updated', { fields: ['description'], description_change_id: bad }), tEn).descriptionChangeId).toBeUndefined()
    }
    expect(describeEvent(ev('card.moved', { from: 'A', to: 'B', description_change_id: 7 }), tEn).descriptionChangeId).toBeUndefined()
  })

  it('localizes changed fields and keeps previous values separate from the action title', () => {
    const data = describeEvent(ev('card.updated', {
      fields: ['branch', 'url', 'human_only', 'engineer_id', 'waits_on', 'priority'],
      previous_values: {
        branch: 'main', url: 'https://example.com/old', human_only: false,
        engineer_id: 'Orion', waits_on: [{ id: 881, number: 81 }, null], priority: 0,
      },
    }), tZh)
    expect(data.text).toBe('修改了分支、链接、仅人工、工程师、依赖、优先级')
    expect(data.previousValues).toEqual([
      { field: 'branch', label: '分支', text: 'main' },
      { field: 'url', label: '链接', text: 'https://example.com/old' },
      { field: 'human_only', label: '仅人工', text: '关闭 → 开启' },
      { field: 'engineer_id', label: '工程师', text: 'Orion' },
      { field: 'waits_on', label: '依赖', references: [{ id: 881, number: 81 }] },
      { field: 'priority', label: '优先级', text: '无' },
    ])
    const historical = describeEvent(ev('card.updated', { fields: ['engineer_id', 'branch'] }), tZh)
    expect(historical.text).toBe('修改了工程师、分支')
    expect(historical.previousValues).toBeUndefined()
  })

  it('不认识的事件类型显示成通用的一句话，不报错', () => {
    expect(describeEvent(ev('card.exploded'), tEn).text).toBe('did something (card.exploded)')
  })

  it('载荷缺字段或根本不是对象时也能显示', () => {
    expect(describeEvent(ev('card.moved', {}), tEn).text).toBe('moved the task from “” to “”')
    const broken = ev('card.moved', {}, { payload: 'not json' as unknown as Record<string, unknown> })
    expect(describeEvent(broken, tEn).text).toBe('moved the task from “” to “”')
    expect(describeEvent(ev('card.moved', { from: 3, to: null }), tEn).text).toBe('moved the task from “” to “”')
  })
})

describe('actorLabel', () => {
  it('人叫「我」', () => {
    expect(actorLabel({ kind: 'human', name: 'human' }, tEn)).toBe('Me')
    expect(actorLabel({ kind: 'human', name: 'human' }, tZh)).toBe('我')
  })

  it('agent 用它自己报的名字', () => {
    expect(actorLabel({ kind: 'agent', name: 'codex' }, tEn)).toBe('codex')
  })

  it('规则触发去掉固定前缀，显示规则名', () => {
    expect(actorLabel({ kind: 'system', name: 'automation:开始计时' }, tEn)).toBe('Automation · 开始计时')
    expect(actorLabel({ kind: 'system', name: 'cron' }, tEn)).toBe('Automation · cron')
  })
})

describe('eventCard', () => {
  const cards = new Map([[7, makeCard({ id: 7, key: 'SLATE-7', title: '现在的标题' })]])

  it('任务还在时用它现在的编号和标题，事件里记的旧标题不算', () => {
    const e = ev('card.created', { key: 'SLATE-7', title: '旧标题' }, { card_id: 7 })
    expect(eventCard(e, cards)).toEqual({ id: 7, key: 'SLATE-7', title: '现在的标题' })
  })

  it('任务已经没了，用事件里记的编号和标题，并且不能点开', () => {
    const e = ev('card.deleted', { key: 'SLATE-9', title: '已删除' }, { card_id: 9 })
    expect(eventCard(e, cards)).toEqual({ id: null, key: 'SLATE-9', title: '已删除' })
  })

  it('和任务无关的事件没有任务', () => {
    expect(eventCard(ev('column.created', { name: '待办' }), cards)).toBeNull()
  })
})
