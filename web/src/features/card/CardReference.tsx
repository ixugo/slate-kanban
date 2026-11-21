import { useEffect, useState, type CSSProperties, type PointerEvent } from 'react'
import { createPortal } from 'react-dom'
import type { CardView } from '@/lib/api/types'
import { cn } from '@/lib/utils'
import { TILE_CLASS, TileBody } from '@/features/board/CardTile'

interface CardReferenceProps {
  label: string
  /** 被引用的同板任务；找不到时为空，不显示预览 */
  card?: CardView
  onOpen: () => void
}

/** 描述和评论里的 #数字 引用：点击打开任务，鼠标悬浮时显示和看板上一样的任务磁贴（标签、截止时间等都有） */
export function CardReference({ label, card, onOpen }: CardReferenceProps) {
  const title = card?.title
  const [position, setPosition] = useState<CSSProperties | null>(null)

  useEffect(() => {
    if (!position) return
    const hide = () => setPosition(null)
    window.addEventListener('scroll', hide, true)
    window.addEventListener('resize', hide)
    return () => {
      window.removeEventListener('scroll', hide, true)
      window.removeEventListener('resize', hide)
    }
  }, [position])

  const show = (event: PointerEvent<HTMLButtonElement>) => {
    if (position || !card || event.pointerType !== 'mouse') return
    const rect = event.currentTarget.getBoundingClientRect()
    const below = rect.bottom <= window.innerHeight / 2
    setPosition({
      left: `clamp(0.5rem, ${rect.left}px, max(0.5rem, calc(100% - 14.5rem)))`,
      ...(below ? { top: rect.bottom + 6 } : { bottom: window.innerHeight - rect.top + 6 }),
    })
  }

  return (
    <>
      <button
        type="button"
        aria-description={title}
        className="cursor-pointer rounded-sm text-primary underline underline-offset-2 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
        onPointerEnter={show}
        onPointerMove={show}
        onPointerLeave={() => setPosition(null)}
        onClick={() => {
          setPosition(null)
          onOpen()
        }}
      >
        {label}
      </button>
      {/* 跳出详情滚动区的裁剪，预览本身不参与焦点、指针或弹层关闭。 */}
      {position && card && createPortal(
        <div
          role="tooltip"
          className={cn(TILE_CLASS, 'pointer-events-none fixed z-[60] w-56 max-w-[calc(100vw-1rem)] border border-border shadow-float')}
          style={position}
        >
          {card && <TileBody card={card} />}
        </div>,
        document.body,
      )}
    </>
  )
}
