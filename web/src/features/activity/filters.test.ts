import { readFileSync } from 'node:fs'
import path from 'node:path'
import { describe, expect, it } from 'vitest'
import { EVENT_GROUPS, HIDDEN_EVENT_TYPES, EVENT_GROUP_ORDER, NO_FILTERS, countFilters, isSameActor, toQuery, toggleGroup } from './filters'

// 后端定义的全部事件类型，直接从 Go 源码里读，后端加了新类型这里会测试失败，提醒给它安排一个分组
const eventSource = readFileSync(path.resolve(import.meta.dirname, '../../../../internal/core/kanban/event.go'), 'utf8')
const serverTypes = [...eventSource.matchAll(/^\s*Event\w+\s*=\s*"([a-z_]+\.[a-z_]+)"/gm)].map((m) => m[1])

describe('事件分组', () => {
  it('后端的每个事件类型恰好属于一个分组，没有多出来的类型', () => {
    expect(serverTypes.length).toBeGreaterThan(15)
    const grouped = EVENT_GROUP_ORDER.flatMap((g) => [...EVENT_GROUPS[g]])
    expect(grouped.sort()).toEqual([...serverTypes].sort())
  })
})

describe('toggleGroup', () => {
  it('没选的加上，选了的去掉，不改原对象', () => {
    const on = toggleGroup(NO_FILTERS, 'move')
    expect(on.groups).toEqual(['move'])
    expect(NO_FILTERS.groups).toEqual([])
    expect(toggleGroup(on, 'move').groups).toEqual([])
  })

  it('不管点击顺序，结果都按规范顺序排，同一组条件只对应一个缓存键', () => {
    const a = toggleGroup(toggleGroup(NO_FILTERS, 'comment'), 'move')
    const b = toggleGroup(toggleGroup(NO_FILTERS, 'move'), 'comment')
    expect(a.groups).toEqual(['move', 'comment'])
    expect(b.groups).toEqual(a.groups)
  })

  it('不影响已选的操作者', () => {
    const actor = { kind: 'agent' as const, name: 'codex' }
    expect(toggleGroup({ groups: [], actor }, 'card').actor).toBe(actor)
  })
})

describe('toQuery', () => {
  it('什么都没选就什么参数都不传', () => {
    expect(toQuery(NO_FILTERS)).toEqual({ types: undefined, exclude_types: HIDDEN_EVENT_TYPES, actor_kind: undefined, actor: undefined })
  })

  it('选中分组里的类型合在一起，操作者拆成类型和名称', () => {
    const filters = { groups: ['move', 'comment'] as const, actor: { kind: 'agent' as const, name: 'codex' } }
    expect(toQuery(filters)).toEqual({
      types: ['card.moved', 'card.reordered', 'comment.added', 'comment.deleted'],
      exclude_types: HIDDEN_EVENT_TYPES,
      actor_kind: 'agent',
      actor: 'codex',
    })
  })
})

describe('countFilters 和 isSameActor', () => {
  it('事件类型不管选几个分组都只算一项，操作者算一项', () => {
    expect(countFilters(NO_FILTERS)).toBe(0)
    expect(countFilters({ groups: ['move', 'card'], actor: null })).toBe(1)
    expect(countFilters({ groups: ['move'], actor: { kind: 'human', name: 'human' } })).toBe(2)
  })

  it('操作者的类型和名称都一样才算同一个', () => {
    const agent = { kind: 'agent' as const, name: 'codex' }
    expect(isSameActor(null, null)).toBe(true)
    expect(isSameActor(agent, { ...agent })).toBe(true)
    expect(isSameActor(agent, null)).toBe(false)
    expect(isSameActor(agent, { kind: 'human', name: 'codex' })).toBe(false)
    expect(isSameActor(agent, { kind: 'agent', name: 'claude' })).toBe(false)
  })
})
