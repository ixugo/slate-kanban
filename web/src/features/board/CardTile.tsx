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

/** 优先级圆点的颜色：低、中、高；0 表示没设优先级，不显示 */
const PRIORITY_DOT = ['', 'bg-p-low', 'bg-p-mid', 'bg-p-high'] as const

interface CardTileProps {
  card: CardView
  dragDisabled?: boolean
  /** 作为拖拽时跟手的那一份：抬起、略放大、阴影更深 */
  overlay?: boolean
}

/** 单张可拖拽卡片；磁贴本身可聚焦，不再嵌套链接 */
export function CardTile({ card, dragDisabled, overlay }: CardTileProps) {
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
        'group relative flex cursor-pointer flex-col gap-2 rounded-[0.875rem] bg-card p-3 text-left shadow-card outline-none transition-[box-shadow,opacity,translate] duration-300 ease-spring hover:shadow-card-hover focus-visible:ring-[3px] focus-visible:ring-ring/50',
        card.done && 'opacity-65',
        isDragging && 'z-10 opacity-35',
        overlay && 'scale-[1.03] rotate-[1.2deg] cursor-grabbing opacity-100 shadow-float',
      )}
      {...attributes}
      {...restListeners}
      onClick={onClick}
      onKeyDown={onKeyDown}
      aria-label={label}
    >
      <div className="space-y-1.5">
        <div className="flex items-center gap-1.5">
          {card.priority > 0 && (
            <span className={cn('size-1.5 shrink-0 rounded-full', PRIORITY_DOT[card.priority])} aria-hidden />
          )}
          <span className="shrink-0 font-mono text-[0.6875rem] text-muted-foreground">{card.key}</span>
        </div>
        <p className="line-clamp-3 text-[0.9375rem] leading-snug font-medium tracking-[-0.005em]">{card.title}</p>
        {(card.tags.length > 0 || card.due_at || card.human_only || card.waiting || card.claim_active) && (
          <div className="flex flex-wrap items-center gap-1.5 pt-0.5">
            {card.tags.slice(0, 3).map((tag) => (
              <Badge key={tag} variant="secondary" className="max-w-[8rem] truncate">
                {tag}
              </Badge>
            ))}
            {card.tags.length > 3 && <span className="text-[0.6875rem] text-muted-foreground">+{card.tags.length - 3}</span>}
            {card.due_at && (
              <span className={cn('text-[0.6875rem]', overdue ? 'font-medium text-destructive' : 'text-muted-foreground')}>
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
              <span className="flex items-center gap-1.5 text-[0.6875rem] font-medium text-agent">
                <span className="signal-dot size-1.5 rounded-full bg-agent" aria-hidden />
                {t('board.claimed', { name: card.claimed_by })}
              </span>
            )}
          </div>
        )}
      </div>
    </div>
  )
}