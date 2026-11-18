import { describe, expect, it } from 'vitest'
import { makeCard } from '@/test/factories'
import {
  EMPTY_FILTERS,
  buildFilterTerms,
  collectClaimers,
  collectTagOptions,
  countFilters,
  isFilterable,
  joinQuery,
  toggleClaimer,
  toggleColumn,
  toggleNoTags,
  togglePriority,
  toggleState,
  toggleTag,
  type BoardFilters,
} from './filters'

/** 2026-11-18 早上 9 点（本地时间） */
const NOW = new Date(2026, 10, 18, 9, 0, 0)

const filters = (patch: Partial<BoardFilters>): BoardFilters => ({ ...EMPTY_FILTERS, ...patch })

describe('buildFilterTerms', () => {
  it('没选任何条件时没有筛选词', () => {
    expect(buildFilterTerms(EMPTY_FILTERS, NOW)).toEqual([])
  })

  it('标签：「任一」用竖线连成一个词，「全部」每个标签一个词，只选一个时两种写法一样', () => {
    expect(buildFilterTerms(filters({ tags: ['bug', 'ui'] }), NOW)).toEqual(['tag:bug|ui'])
    expect(buildFilterTerms(filters({ tags: ['bug', 'ui'], tagMode: 'all' }), NOW)).toEqual(['tag:bug', 'tag:ui'])
    expect(buildFilterTerms(filters({ tags: ['bug'] }), NOW)).toEqual(['tag:bug'])
    expect(buildFilterTerms(filters({ tags: ['bug'], tagMode: 'all' }), NOW)).toEqual(['tag:bug'])
  })

  it('标签带空格时用引号包起来，每个取值各自包', () => {
    expect(buildFilterTerms(filters({ tags: ['my tag', 'ui'] }), NOW)).toEqual(['tag:"my tag"|ui'])
    expect(buildFilterTerms(filters({ tags: ['my tag', 'x y'], tagMode: 'all' }), NOW)).toEqual([
      'tag:"my tag"',
      'tag:"x y"',
    ])
  })

  it('「没有标签」是 no:tags', () => {
    expect(buildFilterTerms(filters({ noTags: true }), NOW)).toEqual(['no:tags'])
  })

  it('列用列 id（改名、重名都不影响），优先级用名字，领取者用名字，多选之间用竖线', () => {
    expect(buildFilterTerms(filters({ columns: [3, 12] }), NOW)).toEqual(['col:3|12'])
    expect(buildFilterTerms(filters({ priorities: ['high', 'none'] }), NOW)).toEqual(['priority:high|none'])
    expect(buildFilterTerms(filters({ claimers: ['codex', 'claude code'] }), NOW)).toEqual(['claimed:codex|"claude code"'])
  })

  it('状态开关每项一个 is: 词，同时满足', () => {
    expect(buildFilterTerms(filters({ states: ['open', 'free', 'human'] }), NOW)).toEqual([
      'is:open',
      'is:free',
      'is:human',
    ])
  })

  it('截止日期：逾期、今天、7 天内、有、没有；不限不产生词', () => {
    expect(buildFilterTerms(filters({ due: 'any' }), NOW)).toEqual([])
    expect(buildFilterTerms(filters({ due: 'overdue' }), NOW)).toEqual(['is:overdue'])
    expect(buildFilterTerms(filters({ due: 'today' }), NOW)).toEqual(['due:today'])
    expect(buildFilterTerms(filters({ due: 'week' }), NOW)).toEqual(['due:>=today', 'due:<=2026-11-25'])
    expect(buildFilterTerms(filters({ due: 'has' }), NOW)).toEqual(['has:due'])
    expect(buildFilterTerms(filters({ due: 'none' }), NOW)).toEqual(['no:due'])
  })

  it('7 天内跨月、跨年时日期正确', () => {
    expect(buildFilterTerms(filters({ due: 'week' }), new Date(2026, 11, 28, 9))).toContain('due:<=2027-01-04')
    expect(buildFilterTerms(filters({ due: 'week' }), new Date(2028, 1, 25, 9))).toContain('due:<=2028-03-03')
  })

  it('写不进语法的取值（含双引号或竖线）直接跳过，不会拼出错误的条件', () => {
    expect(buildFilterTerms(filters({ tags: ['a|b', 'c"d', 'ok'] }), NOW)).toEqual(['tag:ok'])
    expect(buildFilterTerms(filters({ tags: ['a|b'] }), NOW)).toEqual([])
    expect(buildFilterTerms(filters({ claimers: ['x|y'] }), NOW)).toEqual([])
  })

  it('各组一起选：每组一个或几个词，组与组之间都满足', () => {
    const all = filters({
      tags: ['bug', 'ui'],
      columns: [1, 2],
      priorities: ['high'],
      claimers: ['codex'],
      states: ['free'],
      due: 'overdue',
    })
    expect(buildFilterTerms(all, NOW)).toEqual([
      'tag:bug|ui',
      'col:1|2',
      'priority:high',
      'claimed:codex',
      'is:free',
      'is:overdue',
    ])
  })
})

describe('isFilterable', () => {
  it.each(['bug', 'my tag', '前端', 'a:b', '-x'])('%s 可以写进筛选语法', (v) => {
    expect(isFilterable(v)).toBe(true)
  })

  it.each(['a|b', 'say "hi"', '"', '|', ''])('%s 不能写进筛选语法', (v) => {
    expect(isFilterable(v)).toBe(false)
  })
})

describe('joinQuery', () => {
  it('关键字在前，筛选词在后，空白和空项被忽略', () => {
    expect(joinQuery('  登录 ', ['tag:bug', 'is:open'])).toBe('登录 tag:bug is:open')
    expect(joinQuery('', ['tag:bug'])).toBe('tag:bug')
    expect(joinQuery('登录', [])).toBe('登录')
    expect(joinQuery('   ', [])).toBe('')
  })
})

describe('countFilters', () => {
  it('统计选中的条件个数：每个选中的取值算一个，截止不限不算', () => {
    expect(countFilters(EMPTY_FILTERS)).toBe(0)
    expect(countFilters(filters({ tags: ['a', 'b'], columns: [1], states: ['free'], due: 'today' }))).toBe(5)
    expect(countFilters(filters({ noTags: true, priorities: ['high'], claimers: ['codex'] }))).toBe(3)
  })
})

describe('切换条件', () => {
  it('标签：再点一次取消，不区分大小写；选标签会取消「没有标签」', () => {
    const one = toggleTag(EMPTY_FILTERS, 'Bug')
    expect(one.tags).toEqual(['Bug'])
    expect(toggleTag(one, 'bug').tags).toEqual([])
    expect(toggleTag(filters({ noTags: true }), 'bug')).toMatchObject({ tags: ['bug'], noTags: false })
  })

  it('「没有标签」：选上会取消已选的标签，再点一次取消', () => {
    const next = toggleNoTags(filters({ tags: ['bug'] }))
    expect(next).toMatchObject({ tags: [], noTags: true })
    expect(toggleNoTags(next).noTags).toBe(false)
  })

  it('列、优先级、领取者：点一次选上，再点一次取消', () => {
    expect(toggleColumn(EMPTY_FILTERS, 2).columns).toEqual([2])
    expect(toggleColumn(filters({ columns: [2] }), 2).columns).toEqual([])
    expect(togglePriority(EMPTY_FILTERS, 'high').priorities).toEqual(['high'])
    expect(toggleClaimer(filters({ claimers: ['codex'] }), 'codex').claimers).toEqual([])
  })

  it('状态：未完成和已完成互斥，空闲和已领取互斥，其余可以叠加', () => {
    expect(toggleState(filters({ states: ['done'] }), 'open').states).toEqual(['open'])
    expect(toggleState(filters({ states: ['claimed', 'human'] }), 'free').states).toEqual(['human', 'free'])
    expect(toggleState(filters({ states: ['waiting'] }), 'human').states).toEqual(['waiting', 'human'])
    expect(toggleState(filters({ states: ['open'] }), 'open').states).toEqual([])
  })
})

describe('collectTagOptions', () => {
  it('按出现的卡片数从多到少排，同样多按名称排；大小写不同的算同一个标签，写法取第一次出现的', () => {
    const cards = [
      makeCard({ id: 1, tags: ['bug', 'ui'] }),
      makeCard({ id: 2, tags: ['Bug'] }),
      makeCard({ id: 3, tags: ['后端', 'api'] }),
    ]
    expect(collectTagOptions(cards)).toEqual([
      { name: 'bug', count: 2 },
      { name: 'api', count: 1 },
      { name: 'ui', count: 1 },
      { name: '后端', count: 1 },
    ])
  })

  it('写不进筛选语法的标签不出现在选项里；没有标签返回空数组', () => {
    expect(collectTagOptions([makeCard({ id: 1, tags: ['a|b', 'c"d', 'ok'] })])).toEqual([{ name: 'ok', count: 1 }])
    expect(collectTagOptions([makeCard({ id: 1 })])).toEqual([])
  })
})

describe('collectClaimers', () => {
  it('只列出正在领取（租约有效）的 agent，去重并按名称排序', () => {
    const cards = [
      makeCard({ id: 1, claimed_by: 'codex', claim_active: true }),
      makeCard({ id: 2, claimed_by: 'claude', claim_active: true }),
      makeCard({ id: 3, claimed_by: 'codex', claim_active: true }),
      makeCard({ id: 4, claimed_by: 'old-agent', claim_active: false }),
      makeCard({ id: 5, claimed_by: 'x|y', claim_active: true }),
    ]
    expect(collectClaimers(cards)).toEqual(['claude', 'codex'])
  })
})
