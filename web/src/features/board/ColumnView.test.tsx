import { DndContext, useDndContext } from '@dnd-kit/core'
import { waitFor } from '@testing-library/react'
import { useEffect } from 'react'
import { describe, expect, it } from 'vitest'
import { makeCard, makeColumn } from '@/test/factories'
import { renderApp } from '@/test/renderApp'
import { screen } from '@testing-library/react'
import { TooltipProvider } from '@/components/ui/tooltip'
import { setLanguage } from '@/i18n'
import { ColumnView } from './ColumnView'

const noop = () => {}
const addCard = async () => makeCard({ id: 1 })

/** 把 dnd-kit 里已登记的全部落点编号交给测试 */
function Registry({ onChange }: { onChange: (ids: string[]) => void }) {
  const { droppableContainers } = useDndContext()
  useEffect(() => {
    onChange(droppableContainers.toArray().map((c) => String(c.id)))
  })
  return null
}

describe('ColumnView 的拖拽落点编号', () => {
  it('任务 id 和列 id 都是数据库自增的数字，经常相同；相同时它们仍各自是独立的落点', async () => {
    let ids: string[] = []
    const common = { addOpen: false, onAddOpen: noop, onAddClose: noop, onAddCard: addCard, columnCount: 2, onExpand: noop, onCollapse: noop }
    renderApp(
      <DndContext>
        {/* 7 号列收成窄边，8 号列展开且里面有一个 7 号任务：任务的 id 同时撞上了窄边的 id */}
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

    // 三个落点：窄边、展开的列、任务；编号两两不同，任务的落点没有被列顶掉
    await waitFor(() => expect(ids).toHaveLength(3))
    expect(new Set(ids).size).toBe(3)
    expect(ids).toContain('7')
  })
})

describe('ColumnView 的起步模板文案', () => {
  const common = { addOpen: false, onAddOpen: noop, onAddClose: noop, onAddCard: addCard, columnCount: 1, onExpand: noop, onCollapse: noop, cards: [], cardIds: [], collapsed: false, columnIndex: 0 }

  it('起步模板的列名和说明跟着界面语言走，自己起的名字原样显示', async () => {
    await setLanguage('en')
    renderApp(
      <DndContext>
        <TooltipProvider>
        <ColumnView {...common} column={makeColumn({ id: 1, name: '待办' })} />
        <ColumnView {...common} column={makeColumn({ id: 2, name: '我的自定义列' })} />
        </TooltipProvider>
      </DndContext>,
    )
    expect(screen.getByRole('heading', { name: /To do/ })).toBeInTheDocument()
    expect(screen.queryByText('Tasks that are confirmed and waiting in line.')).not.toBeInTheDocument()
    expect(screen.getByRole('heading', { name: /我的自定义列/ })).toBeInTheDocument()
  })
})
