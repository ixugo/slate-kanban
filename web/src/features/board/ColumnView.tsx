// 看板中的一列：列头、可排序的卡片列表、添加卡片；支持收起为窄边落点。

import { useDroppable } from '@dnd-kit/core'
import { SortableContext, verticalListSortingStrategy } from '@dnd-kit/sortable'
import type { MouseEvent } from 'react'
import { ChevronLeftIcon, ChevronRightIcon } from 'lucide-react'
import { useTranslation } from '@/i18n/react'
import type { CardView, Column } from '@/lib/api/types'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
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

/** 列的外壳：展开和收起共用，宽度变化时有平滑的过渡；内圆角 14px 加 6px 内边距，外圆角正好是 20px */
const COLUMN_SHELL =
  'flex shrink-0 snap-center flex-col overflow-hidden rounded-[1.25rem] bg-column transition-[width,background-color,box-shadow] duration-[350ms] ease-sheet'

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

  /** 窄边整块当作展开按钮；原生按钮的回车和空格都会触发点击，不需要另外处理按键 */
  const narrowExpand = (e: MouseEvent) => {
    e.stopPropagation()
    onExpand()
  }

  if (collapsed) {
    return (
      <section className={cn(COLUMN_SHELL, 'w-12', isOver && 'bg-ring/15 ring-2 ring-ring/50')}>
        <button
          ref={setNodeRef}
          type="button"
          aria-expanded={false}
          aria-label={t('board.expandColumn', { name: column.name, count: String(cards.length) })}
          className="group/rail flex min-h-[12rem] w-12 flex-1 flex-col items-center gap-3 px-1 py-3.5 text-center outline-none transition-colors duration-200 hover:bg-foreground/[0.04] focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:ring-inset"
          onClick={narrowExpand}
        >
          <span className="flex size-7 items-center justify-center rounded-full bg-secondary text-muted-foreground transition-colors duration-200 group-hover/rail:bg-accent group-hover/rail:text-foreground">
            <ExpandIcon className="size-4" aria-hidden />
          </span>
          <span className="rounded-full bg-secondary px-1.5 text-[0.6875rem] font-medium tabular-nums text-muted-foreground">
            {cards.length}
          </span>
          <span className="max-h-44 truncate text-ui font-semibold tracking-wide [writing-mode:vertical-rl]">{column.name}</span>
        </button>
      </section>
    )
  }

  return (
    <section className={cn(COLUMN_SHELL, 'w-[min(100%,17.5rem)] p-1.5 sm:w-[17.5rem]', isOver && 'bg-ring/10 ring-2 ring-ring/40')}>
      <header className="flex items-start gap-2 px-2.5 pt-2 pb-2.5">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-1.5">
            <h2 className="truncate text-body font-semibold tracking-tight">{column.name}</h2>
            <span className="rounded-full bg-secondary px-1.5 text-[0.6875rem] font-medium tabular-nums text-muted-foreground">
              {cards.length}
            </span>
          </div>
          {column.description && (
            <Tooltip>
              <TooltipTrigger asChild>
                <p className="mt-1 line-clamp-1 cursor-help text-xs text-muted-foreground">{column.description}</p>
              </TooltipTrigger>
              <TooltipContent className="max-w-xs text-xs">{column.description}</TooltipContent>
            </Tooltip>
          )}
        </div>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          className="-mt-0.5 shrink-0 text-muted-foreground"
          aria-label={t('board.collapseColumn', { name: column.name })}
          onClick={(e) => {
            e.stopPropagation()
            onCollapse()
          }}
        >
          <CollapseIcon className="size-4" aria-hidden />
        </Button>
      </header>
      <div ref={setNodeRef} className="-mx-1.5 flex min-h-24 flex-1 flex-col gap-2 overflow-y-auto px-1.5 pt-0.5 pb-1.5">
        <SortableContext items={cardIds} strategy={verticalListSortingStrategy}>
          {cards.map((card) => (
            <CardTile key={card.id} card={card} dragDisabled={dragDisabled} />
          ))}
        </SortableContext>
        {cards.length === 0 && (
          <p className="py-8 text-center text-ui text-muted-foreground/80">{t('board.emptyColumn')}</p>
        )}
      </div>
      <footer className="pt-0.5">
        <AddCard open={addOpen} onOpen={onAddOpen} onClose={onAddClose} onSubmit={onAddCard} busy={addBusy} />
      </footer>
    </section>
  )
}
