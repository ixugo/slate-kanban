// 看板列里的卡片磁贴：单一 Tab 停靠点，Enter 开弹窗，空格交给 dnd-kit 拖拽。

import { useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { LockIcon, Link2Icon } from 'lucide-react'
import { useTranslation } from '@/i18n/react'
import { useSearchParams } from 'react-router-dom'
import type { CardView } from '@/lib/api/types'
import { formatDate, isPast } from '@/lib/format'
import { useLanguage } from '@/i18n/useLanguage'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import type { KeyboardEvent, MouseEvent } from 'react'

const PRIORITY_BAR = ['', 'bg-muted-foreground/50', 'bg-signal/80', 'bg-destructive/80'] as const

interface CardTileProps {
  card: CardView
  dragDisabled?: boolean
}

/** 单张可拖拽卡片；磁贴本身可聚焦，不再嵌套链接 */
export function CardTile({ card, dragDisabled }: CardTileProps) {
  const { t } = useTranslation()
  const { lang } = useLanguage()
  const [, setParams] = useSearchParams()
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: card.id,
    disabled: dragDisabled,
  })
  const { onKeyDown: sortableKeyDown, ...restListeners } = listeners ?? {}
  const style = { transform: CSS.Transform.toString(transform), transition }
  const overdue = card.due_at && !card.done && isPast(card.due_at)

  const openCard = () => {
    setParams((prev) => {
      const next = new URLSearchParams(prev)
      next.set('card', String(card.id))
      return next
    })
  }

  const onClick = (e: MouseEvent) => {
    if (dragDisabled || isDragging) return
    if (e.defaultPrevented) return
    openCard()
  }

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    sortableKeyDown?.(e)
    if (e.key === 'Enter' && !isDragging) {
      e.preventDefault()
      openCard()
    }
  }

  const label = `${card.key} ${card.title}`

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={cn(
        'group relative flex cursor-pointer flex-col gap-1.5 rounded-md border bg-card p-2.5 text-left shadow-sm ring-1 ring-foreground/5 outline-none focus-visible:ring-2 focus-visible:ring-ring',
        card.done && 'opacity-60',
        isDragging && 'z-10 opacity-90 shadow-md',
      )}
      {...attributes}
      {...restListeners}
      onClick={onClick}
      onKeyDown={onKeyDown}
      aria-label={label}
    >
      {card.priority > 0 && (
        <span className={cn('absolute inset-y-2 left-0 w-0.5 rounded-full', PRIORITY_BAR[card.priority])} aria-hidden />
      )}
      <div className="space-y-1">
        <div className="flex items-start gap-2">
          <span className="shrink-0 font-mono text-[10px] text-muted-foreground">{card.key}</span>
          <span className="min-w-0 flex-1 text-sm font-medium leading-snug line-clamp-3">{card.title}</span>
        </div>
        {(card.tags.length > 0 || card.due_at || card.human_only || card.waiting || card.claim_active) && (
          <div className="flex flex-wrap items-center gap-1">
            {card.tags.slice(0, 3).map((tag) => (
              <Badge key={tag} variant="secondary" className="max-w-[8rem] truncate px-1.5 text-[10px] font-normal">
                {tag}
              </Badge>
            ))}
            {card.tags.length > 3 && <span className="text-[10px] text-muted-foreground">+{card.tags.length - 3}</span>}
            {card.due_at && (
              <span className={cn('text-[10px]', overdue ? 'font-medium text-destructive' : 'text-muted-foreground')}>
                {formatDate(card.due_at, lang)}
              </span>
            )}
            {card.human_only && (
              <span title={t('board.humanOnly')} className="text-muted-foreground">
                <LockIcon className="size-3" aria-hidden />
              </span>
            )}
            {card.waiting && (
              <span title={t('board.waiting')} className="text-muted-foreground">
                <Link2Icon className="size-3" aria-hidden />
              </span>
            )}
            {card.claim_active && card.claimed_by && (
              <span className="flex items-center gap-1 text-[10px] text-signal">
                <span className="signal-dot size-1.5 rounded-full bg-signal" aria-hidden />
                {t('board.claimed', { name: card.claimed_by })}
              </span>
            )}
          </div>
        )}
      </div>
    </div>
  )
}