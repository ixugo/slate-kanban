// 看板列里的卡片磁贴：整张磁贴是一个指向卡片的真实链接，只有一个 Tab 停靠点。
// 左键点击在当前页弹出详情；中键或 Cmd/Ctrl+点击由浏览器在新标签页打开同一张卡片；空格交给 dnd-kit 拖拽。

import { useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { LockIcon, Link2Icon } from 'lucide-react'
import { useTranslation } from '@/i18n/react'
import { Link, useSearchParams } from 'react-router-dom'
import type { CardView } from '@/lib/api/types'
import { formatDate, isPast } from '@/lib/format'
import { useLanguage } from '@/i18n/useLanguage'
import { cn } from '@/lib/utils'
import { TagBadge } from '@/features/labels/TagBadge'
import { displayKey } from '@/lib/cardKey'

/** 优先级圆点的颜色：低、中、高；0 表示没设优先级，不显示 */
const PRIORITY_DOT = ['', 'bg-p-low', 'bg-p-mid', 'bg-p-high'] as const

/** 磁贴的外观：链接版和拖拽时跟手的那一份共用；iOS 的长按链接预览会和长按拖拽冲突，所以关掉 */
const TILE_CLASS =
  'group relative flex shrink-0 flex-col gap-2 overflow-hidden rounded-[0.875rem] bg-card p-3 text-left shadow-card outline-none transition-[box-shadow,opacity,translate] duration-300 ease-spring [-webkit-touch-callout:none]'

interface CardTileProps {
  card: CardView
  /** 搜索或筛选时不能拖：位置在结果里没有意义；点击打开卡片不受影响 */
  dragDisabled?: boolean
}

/** 磁贴里的内容：编号和领取者（同一行）、标题、标签、截止日期、状态图标 */
function TileBody({ card }: { card: CardView }) {
  const { t } = useTranslation()
  const { lang } = useLanguage()
  const overdue = card.due_at && !card.done && isPast(card.due_at)
  const tagIds = card.tag_ids ?? []

  return (
    <div className="space-y-1.5">
      <div className="flex items-center gap-1.5">
        {card.priority > 0 && (
          <span className={cn('size-1.5 shrink-0 rounded-full', PRIORITY_DOT[card.priority])} aria-hidden />
        )}
        <span className="shrink-0 font-mono text-[0.6875rem] text-muted-foreground">{displayKey(card.key)}</span>
        {card.claim_active && card.claimed_by && (
          <span className="flex min-w-0 items-center gap-1.5 text-[0.6875rem] font-medium text-agent">
            <span className="signal-dot size-1.5 shrink-0 rounded-full bg-agent" aria-hidden />
            <span className="truncate">{t('board.claimed', { name: card.claimed_by })}</span>
          </span>
        )}
      </div>
      <p className="line-clamp-3 text-[0.9375rem] leading-snug font-medium tracking-[-0.005em]">{card.title}</p>
      {(tagIds.length > 0 || card.due_at || card.human_only || card.waiting) && (
        <div className="flex flex-wrap items-center gap-1.5 pt-0.5">
          {tagIds.slice(0, 3).map((id) => (
            <TagBadge key={id} id={id} />
          ))}
          {tagIds.length > 3 && <span className="text-[0.6875rem] text-muted-foreground">+{tagIds.length - 3}</span>}
          {card.due_at && (
            <span className={cn('font-mono text-[0.6875rem]', overdue ? 'font-medium text-destructive' : 'text-muted-foreground')}>
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
        </div>
      )}
    </div>
  )
}

/** 单张可拖拽卡片：一个指向 `?card=` 的链接，点击不刷新页面，修饰键点击交给浏览器新开标签页 */
export function CardTile({ card, dragDisabled }: CardTileProps) {
  const [params] = useSearchParams()
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: card.id,
    disabled: dragDisabled,
    // 实际元素是 <a>，别被 dnd-kit 默认的 button 角色盖掉
    attributes: { role: 'link' },
  })
  const style = { transform: CSS.Transform.toString(transform), transition }
  // 沿用地址里已有的其他参数，只替换 card
  const next = new URLSearchParams(params)
  next.set('card', String(card.id))

  return (
    <Link
      ref={setNodeRef}
      to={{ search: next.toString() }}
      // 链接默认能被浏览器原生拖走，会打断 dnd-kit 的指针拖拽
      draggable={false}
      style={style}
      className={cn(
        TILE_CLASS,
        'cursor-pointer hover:shadow-card-hover focus-visible:ring-[3px] focus-visible:ring-ring/50',
        card.done && 'opacity-65',
        isDragging && 'z-10 opacity-35',
      )}
      {...attributes}
      {...listeners}
      // 搜索时 dnd-kit 会标记 disabled，但链接仍然能点，不能让读屏说它不可用
      aria-disabled={undefined}
      aria-label={`${displayKey(card.key)} ${card.title}`}
      // 键盘拖拽中按 Enter 会触发链接点击，这时不能打开卡片
      onClick={(e) => {
        if (isDragging) e.preventDefault()
      }}
    >
      <TileBody card={card} />
    </Link>
  )
}

/** 拖拽时跟手的那一份：抬起、略放大、阴影更深；只是视觉副本，不是链接，也不进读屏 */
export function CardTileOverlay({ card }: { card: CardView }) {
  return (
    <div className={cn(TILE_CLASS, 'scale-[1.03] rotate-[1.2deg] cursor-grabbing shadow-float')} aria-hidden>
      <TileBody card={card} />
    </div>
  )
}
