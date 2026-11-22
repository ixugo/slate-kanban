import { useId, useRef, useState, type ComponentProps, type PointerEvent } from 'react'
import { ArchiveIcon, EllipsisIcon, GitForkIcon, Trash2Icon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { useTranslation } from '@/i18n/react'
import { cn } from '@/lib/utils'

interface CardActionsProps {
  archived: boolean
  busy: boolean
  onFork: () => void
  onArchive: () => void
  onDelete: () => void
}

const ACTION_STYLE = 'shrink-0 text-muted-foreground hover:text-foreground'

type DeleteConfirmationProps = {
  open: boolean
  busy: boolean
  onOpenChange: (open: boolean) => void
  onCloseAutoFocus: ComponentProps<typeof PopoverContent>['onCloseAutoFocus']
  onConfirm: () => void
}

/** A confirmation anchored to its trigger, with no backdrop or modal focus trap. */
function DeleteConfirmation({ open, busy, onOpenChange, onCloseAutoFocus, onConfirm }: DeleteConfirmationProps) {
  const { t } = useTranslation()
  const label = t('common.delete')
  const titleId = useId()
  const descriptionId = useId()
  const cancelRef = useRef<HTMLButtonElement>(null)

  return (
    <Popover open={open} onOpenChange={onOpenChange} modal={false}>
      <PopoverTrigger asChild>
        <Button type="button" variant="destructive" size="icon-sm" disabled={busy} aria-label={label} title={label}>
          <Trash2Icon />
        </Button>
      </PopoverTrigger>
      <PopoverContent
        align="end"
        className="z-[60] w-64 max-w-[calc(100vw-1rem)] motion-reduce:animate-none"
        aria-labelledby={titleId}
        aria-describedby={descriptionId}
        onOpenAutoFocus={(event) => {
          event.preventDefault()
          cancelRef.current?.focus()
        }}
        onCloseAutoFocus={onCloseAutoFocus}
      >
        <p id={titleId} className="font-semibold">{t('card.deleteTitle')}</p>
        <p id={descriptionId} className="text-xs text-muted-foreground">{t('card.deleteBody')}</p>
        <div className="flex justify-end gap-1.5">
          <Button ref={cancelRef} type="button" size="sm" variant="outline" onClick={() => onOpenChange(false)}>
            {t('common.cancel')}
          </Button>
          <Button type="button" size="sm" variant="destructive" disabled={busy} onClick={() => {
            onOpenChange(false)
            onConfirm()
          }}>
            {label}
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  )
}

/** Hover opens the action strip. The disclosure button also works without a pointer. */
export function CardActions({ archived, busy, onFork, onArchive, onDelete }: CardActionsProps) {
  const { t } = useTranslation()
  const archiveLabel = t('card.archive.action')
  const stripId = useId()
  const more = useRef<HTMLButtonElement>(null)
  const strip = useRef<HTMLDivElement>(null)
  const [hovered, setHovered] = useState(false)
  const [toggled, setToggled] = useState(false)
  const [confirmingDelete, setConfirmingDelete] = useState(false)
  const expanded = hovered || toggled || confirmingDelete
  const revealOnMouse = (event: PointerEvent<HTMLDivElement>) => {
    if (!hovered && event.pointerType === 'mouse' && event.target instanceof Node && event.currentTarget.contains(event.target)) {
      setHovered(true)
    }
  }
  const restoreFocus = (event: Event) => {
    if (!hovered && !toggled) {
      event.preventDefault()
      more.current?.focus()
    }
  }

  return (
    <div
      className="relative flex shrink-0 items-center"
      data-card-actions-expanded={expanded}
      onPointerEnter={revealOnMouse}
      onPointerMove={revealOnMouse}
      onPointerLeave={(event) => {
        if (event.pointerType !== 'mouse') return
        setHovered(false)
        if (!toggled && !confirmingDelete && strip.current?.contains(document.activeElement)) more.current?.focus()
      }}
      onBlur={(event) => {
        if (!confirmingDelete && !event.currentTarget.contains(event.relatedTarget)) setToggled(false)
      }}
      onKeyDown={(event) => {
        if (event.key !== 'Escape' || confirmingDelete || !expanded) return
        setHovered(false)
        setToggled(false)
        more.current?.focus()
      }}
    >
      <Button
        ref={more}
        type="button"
        variant="ghost"
        size="icon-sm"
        aria-label={t('common.more')}
        title={t('common.more')}
        aria-expanded={expanded}
        aria-controls={stripId}
        className={ACTION_STYLE}
        onClick={(event) => {
          if (event.detail === 0 || !hovered) setToggled((open) => !open)
        }}
      >
        <EllipsisIcon className={cn('transition-[scale] duration-300 ease-spring-bounce motion-reduce:transition-none', expanded && 'scale-115')} />
      </Button>
      <div
        id={stripId}
        ref={strip}
        inert={!expanded}
        aria-hidden={!expanded}
        className={cn(
          'absolute right-full z-10 flex w-max origin-right items-center gap-2 rounded-lg bg-popover py-0.5 pr-2 pl-0.5 shadow-sm transition-[clip-path,scale,translate,opacity] duration-300 ease-spring-bounce motion-reduce:transition-none',
          expanded
            ? 'scale-100 opacity-100 [clip-path:inset(-0.25rem)]'
            : 'pointer-events-none translate-x-2 scale-90 opacity-0 [clip-path:inset(0_0_0_100%)]',
        )}
      >
        <DeleteConfirmation
          open={confirmingDelete}
          busy={busy}
          onOpenChange={(open) => {
            if (open) setHovered(false)
            setConfirmingDelete(open)
          }}
          onCloseAutoFocus={restoreFocus}
          onConfirm={onDelete}
        />
        <Button type="button" variant="ghost" size="icon-sm" className={ACTION_STYLE} aria-label="Fork" title="Fork" disabled={busy} onClick={onFork}>
          <GitForkIcon />
        </Button>
        {!archived && (
          <Button type="button" variant="ghost" size="icon-sm" className={ACTION_STYLE} disabled={busy} aria-label={archiveLabel} title={archiveLabel} onClick={() => {
            more.current?.focus()
            onArchive()
          }}>
            <ArchiveIcon />
          </Button>
        )}
      </div>
    </div>
  )
}
