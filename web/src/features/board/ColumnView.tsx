// 看板中的一列：列头、可排序的卡片列表、添加卡片。

import { useDroppable } from '@dnd-kit/core'
import { SortableContext, verticalListSortingStrategy } from '@dnd-kit/sortable'
import { useTranslation } from '@/i18n/react'
import type { CardView, Column } from '@/lib/api/types'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Badge } from '@/components/ui/badge'
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
}

/** 单列容器，同时作为拖拽落点 */
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
}: ColumnViewProps) {
  const { t } = useTranslation()
  const { setNodeRef, isOver } = useDroppable({ id: column.id, data: { type: 'column', columnId: column.id } })

  return (
    <section
      className={cn(
        'flex w-[min(100%,20rem)] shrink-0 snap-center flex-col rounded-lg border bg-muted/30 sm:w-72',
        isOver && 'ring-2 ring-signal/40',
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
      </header>
      <div ref={setNodeRef} className="flex min-h-24 flex-1 flex-col gap-2 overflow-y-auto p-2">
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