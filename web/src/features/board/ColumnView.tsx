// 看板中的一列：列头、可排序的卡片列表、添加卡片；支持收起为窄边落点。

import { useDroppable } from '@dnd-kit/core'
import { SortableContext, verticalListSortingStrategy } from '@dnd-kit/sortable'
import type { KeyboardEvent, MouseEvent } from 'react'
import { ChevronLeftIcon, ChevronRightIcon } from 'lucide-react'
import { useTranslation } from '@/i18n/react'
import type { CardView, Column } from '@/lib/api/types'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { CardTile } from './CardTile'
import { AddCard } from './AddCard'
import { cn } from '@/lib/utils'

interface ColumnViewProps {
  column: Column
  cards: CardView[]
  cardIds: number[]
  addOpen: boolean
  onAddOpen: () => void
  onAddClose: () => void
  onAddCard: (title: string) => void
  addBusy?: boolean
  dragDisabled?: boolean
  /** 当前是否收成 48px 窄边 */
  collapsed: boolean
  /** 在整板中的下标，用来决定收起箭头朝左还是朝右 */
  columnIndex: number
  columnCount: number
  onExpand: () => void
  onCollapse: () => void
}

/** 箭头朝哪一侧收起：靠左的列向左收，靠右的列向右收 */
function collapseTowardStart(columnIndex: number, columnCount: number): boolean {
  return columnIndex < columnCount / 2
}

/** 单列容器，展开时卡片区为拖拽落点，收起时窄边为落点 */
export function ColumnView({
  column,
  cards,
  cardIds,
  addOpen,
  onAddOpen,
  onAddClose,
  onAddCard,
  addBusy,
  dragDisabled,
  collapsed,
  columnIndex,
  columnCount,
  onExpand,
  onCollapse,
}: ColumnViewProps) {
  const { t } = useTranslation()
  const { setNodeRef, isOver } = useDroppable({ id: column.id, data: { type: 'column', columnId: column.id } })
  const towardStart = collapseTowardStart(columnIndex, columnCount)
  const CollapseIcon = towardStart ? ChevronLeftIcon : ChevronRightIcon
  const ExpandIcon = towardStart ? ChevronRightIcon : ChevronLeftIcon

  /** 窄边整块当作展开按钮，阻止冒泡以免误触拖拽层 */
  const narrowExpand = (e: MouseEvent | KeyboardEvent) => {
    e.stopPropagation()
    onExpand()
  }

  if (collapsed) {
    return (
      <section
        className={cn(
          'flex w-12 shrink-0 snap-center flex-col overflow-hidden rounded-lg border bg-muted/30 transition-[width] duration-[250ms] motion-reduce:transition-none',
          isOver && 'ring-2 ring-signal/40',
        )}
      >
        <button
          ref={setNodeRef}
          type="button"
          aria-expanded={false}
          aria-label={t('board.expandColumn', { name: column.name, count: String(cards.length) })}
          className="flex min-h-[12rem] w-12 flex-1 flex-col items-center gap-2 px-1 py-3 text-center hover:bg-muted/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          onClick={narrowExpand}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault()
              narrowExpand(e)
            }
          }}
        >
          <ExpandIcon className="size-4 shrink-0 text-muted-foreground" aria-hidden />
          <span className="text-xs tabular-nums text-muted-foreground">{cards.length}</span>
          <span className="max-h-40 truncate text-xs font-semibold [writing-mode:vertical-rl]">{column.name}</span>
        </button>
      </section>
    )
  }

  return (
    <section
      className={cn(
        'flex w-[min(100%,20rem)] shrink-0 snap-center flex-col rounded-lg border bg-muted/30 transition-[width] duration-[250ms] motion-reduce:transition-none sm:w-72',
      )}
    >
      <header className="flex items-start gap-2 border-b px-3 py-2">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-1.5">
            <h2 className="truncate text-sm font-semibold">{column.name}</h2>
            <span className="text-xs text-muted-foreground">{cards.length}</span>
            {column.agent_pickup && (
              <Badge variant="outline" className="text-[10px]">
                {t('board.columnAi')}
              </Badge>
            )}
            {column.is_done && (
              <Badge variant="secondary" className="text-[10px]">
                {t('board.columnDone')}
              </Badge>
            )}
          </div>
          {column.description && (
            <Tooltip>
              <TooltipTrigger asChild>
                <p className="mt-0.5 line-clamp-1 cursor-help text-[11px] text-muted-foreground">{column.description}</p>
              </TooltipTrigger>
              <TooltipContent className="max-w-xs text-xs">{column.description}</TooltipContent>
            </Tooltip>
          )}
        </div>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          className="shrink-0"
          aria-label={t('board.collapseColumn', { name: column.name })}
          onClick={(e) => {
            e.stopPropagation()
            onCollapse()
          }}
        >
          <CollapseIcon className="size-4" aria-hidden />
        </Button>
      </header>
      <div ref={setNodeRef} className={cn('flex min-h-24 flex-1 flex-col gap-2 overflow-y-auto p-2', isOver && 'ring-2 ring-signal/40 ring-inset')}>
        <SortableContext items={cardIds} strategy={verticalListSortingStrategy}>
          {cards.map((card) => (
            <CardTile key={card.id} card={card} dragDisabled={dragDisabled} />
          ))}
        </SortableContext>
        {cards.length === 0 && <p className="py-6 text-center text-xs text-muted-foreground">{t('board.emptyColumn')}</p>}
      </div>
      <footer className="border-t p-2">
        <AddCard open={addOpen} onOpen={onAddOpen} onClose={onAddClose} onSubmit={onAddCard} busy={addBusy} />
      </footer>
    </section>
  )
}