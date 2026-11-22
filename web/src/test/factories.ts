// 测试用的数据工厂：生成字段齐全的任务、列、快照，各测试只写自己关心的字段。

import type { Board, CardView, Column, Snapshot } from '@/lib/api/types'

const T0 = '2026-11-18T08:00:00Z'

/** 生成一个任务，id 和编号随 id 变化，其余字段是"普通的空任务" */
export function makeCard(overrides: Partial<CardView> & { id: number }): CardView {
  const id = overrides.id
  return {
    board_id: 1,
    column_id: 1,
    number: id,
    key: `SLATE-${id}`,
    title: `任务 ${id}`,
    description: '',
    position: id * 1024,
    priority: 0,
    tag_ids: [],
    tags: [],
    human_only: false,
    waits_on: [],
    due_at: null,
    started_at: null,
    finished_at: null,
    claimed_by: '',
    engineer_id: 0,
    claim_engineer_id: 0,
    claim_expires_at: null,
    branch: '',
    url: '',
    version: 1,
    unread_count: 0,
    created_by: 'admin',
    updated_by: 'admin',
    created_at: T0,
    updated_at: T0,
    column: '待办',
    done: false,
    waiting: false,
    claim_active: false,
    ...overrides,
  }
}

/** 生成一列 */
export function makeColumn(overrides: Partial<Column> & { id: number }): Column {
  return {
    board_id: 1,
    name: `列 ${overrides.id}`,
    instruction: '',
    is_done: false,
    next_column_id: 0,
    sort: overrides.id,
    created_at: T0,
    updated_at: T0,
    ...overrides,
  }
}

/** 生成看板信息 */
export function makeBoard(overrides: Partial<Board> = {}): Board {
  return {
    id: 1,
    name: '示例看板',
    slug: 'SLATE',
    description: '',
    tags: [],
    engineers: [],
    project_path: '',
    created_at: T0,
    updated_at: T0,
    last_activity_at: T0,
    ...overrides,
  }
}

/** 生成快照；不传列时给三列：待办、进行中、完成（最后一列是完成列） */
export function makeSnapshot(overrides: Partial<Snapshot> = {}): Snapshot {
  return {
    ...makeBoard(),
    columns: [
      makeColumn({ id: 1, name: '待办' }),
      makeColumn({ id: 2, name: '进行中' }),
      makeColumn({ id: 3, name: '完成', is_done: true }),
    ],
    rules: [],
    cards: [],
    seq: 0,
    ...overrides,
  }
}
