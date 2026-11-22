import { useEffect, useRef } from 'react'
import { useInfiniteQuery } from '@tanstack/react-query'
import { Link, useSearchParams } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { useTranslation } from '@/i18n/react'
import { useLanguage } from '@/i18n/useLanguage'
import { listArchivedCards } from '@/lib/api/endpoints'
import { formatDateTime } from '@/lib/format'
import { queryKeys } from '@/lib/queryKeys'

interface ArchiveDrawerProps {
  boardId: number
  open: boolean
  onOpenChange: (open: boolean) => void
}

export default function ArchiveDrawer({ boardId, open, onOpenChange }: ArchiveDrawerProps) {
  const { t } = useTranslation()
  const { lang } = useLanguage()
  const [params] = useSearchParams()
  const content = useRef<HTMLDivElement>(null)
  const end = useRef<HTMLDivElement>(null)
  const list = useInfiniteQuery({
    queryKey: queryKeys.archivedCards(boardId),
    enabled: open,
    queryFn: ({ pageParam, signal }) => listArchivedCards(boardId, pageParam, signal),
    initialPageParam: undefined as number | undefined,
    getNextPageParam: (last) => last.has_more ? last.cards.at(-1)?.id : undefined,
  })
  const { fetchNextPage, hasNextPage, isFetching, isFetchNextPageError } = list

  useEffect(() => {
    if (!open || !hasNextPage || isFetching || isFetchNextPageError || !end.current) return
    const observer = new IntersectionObserver(([entry]) => {
      if (entry?.isIntersecting) void fetchNextPage({ cancelRefetch: false })
    }, { root: content.current })
    observer.observe(end.current)
    return () => observer.disconnect()
  }, [open, fetchNextPage, hasNextPage, isFetching, isFetchNextPageError])

  const cards = list.data?.pages.flatMap((page) => page.cards) ?? []

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="gap-0 data-[side=right]:w-full">
        <SheetHeader className="border-b pr-14">
          <SheetTitle>{t('card.archive.title')}</SheetTitle>
          <SheetDescription className="sr-only">{t('card.archive.description')}</SheetDescription>
        </SheetHeader>
        <div ref={content} className="min-h-0 flex-1 overflow-y-auto px-4 pb-4">
          <ul className="divide-y">
            {cards.map((card) => {
              const next = new URLSearchParams(params)
              next.set('card', String(card.id))
              return (
                <li key={card.id}>
                  <Link
                    to={{ search: next.toString() }}
                    onClick={() => onOpenChange(false)}
                    className="flex min-w-0 items-start gap-3 rounded-md px-2 py-3.5 hover:bg-muted/55 focus-visible:outline-2 focus-visible:outline-ring"
                  >
                    <span className="shrink-0 font-mono text-xs leading-5 text-muted-foreground">#{card.number}</span>
                    <span className="min-w-0">
                      <span className="block truncate text-sm font-medium">{card.title}</span>
                      {card.archived_at && (
                        <time dateTime={card.archived_at} className="text-xs text-muted-foreground">{formatDateTime(card.archived_at, lang)}</time>
                      )}
                    </span>
                  </Link>
                </li>
              )
            })}
          </ul>
          {list.isPending || list.isFetchingNextPage ? <p role="status" className="py-4 text-sm text-muted-foreground">{t('common.loading')}</p> : null}
          {list.isSuccess && cards.length === 0 && <p className="py-4 text-sm text-muted-foreground">{t('card.archive.empty')}</p>}
          {list.isError && (
            <Button type="button" size="sm" variant="outline" className="mt-4" onClick={() => {
              if (isFetchNextPageError) void fetchNextPage()
              else void list.refetch()
            }}>
              {t('common.retry')}
            </Button>
          )}
          <div ref={end} aria-hidden="true" className="h-px" />
        </div>
      </SheetContent>
    </Sheet>
  )
}
