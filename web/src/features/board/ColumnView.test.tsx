import { DndContext, useDndContext } from '@dnd-kit/core'
import { waitFor } from '@testing-library/react'
import { useEffect } from 'react'
import { describe, expect, it } from 'vitest'
import { makeCard, makeColumn } from '@/test/factories'
import { renderApp } from '@/test/renderApp'
import { ColumnView } from './ColumnView'

const noop = () => {}

/** 把 dnd-kit 里已登记的全部落点编号交给测试 */
function Registry({ onChange }: { onChange: (ids: string[]) => void }) {
  const { droppableContainers } = useDndContext()
  useEffect(() => {
    onChange(droppableContainers.toArray().map((c) => String(c.id)))
  })
  return null
}

describe('ColumnView 的拖拽落点编号', () => {
  it('卡片 id 和列 id 都是数据库自增的数字，经常相同；相同时它们仍各自是独立的落点', async () => {
    let ids: string[] = []
    const common = { addOpen: false, onAddOpen: noop, onAddClose: noop, onAddCard: noop, columnCount: 2, onExpand: noop, onCollapse: noop }
    renderApp(
      <DndContext>
        {/* 7 号列收成窄边，8 号列展开且里面有一张 7 号卡片：卡片的 id 同时撞上了窄边的 id */}
        <ColumnView {...common} column={makeColumn({ id: 7, name: 'Rail' })} cards={[]} cardIds={[]} collapsed columnIndex={0} />
        <ColumnView
          {...common}
          column={makeColumn({ id: 8, name: 'Body' })}
          cards={[makeCard({ id: 7, column_id: 8, title: 'Same id as the rail' })]}
          cardIds={[7]}
          collapsed={false}
          columnIndex={1}
        />
        <Registry onChange={(next) => (ids = next)} />
      </DndContext>,
    )

    // 三个落点：窄边、展开的列、卡片；编号两两不同，卡片的落点没有被列顶掉
    await waitFor(() => expect(ids).toHaveLength(3))
    expect(new Set(ids).size).toBe(3)
    expect(ids).toContain('7')
  })
})
