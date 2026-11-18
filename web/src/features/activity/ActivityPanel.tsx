// 活动抽屉的内容：筛选条加按时间倒序的事件列表。只在抽屉打开时才挂载，所以关着的时候不会发请求；
// 每次打开都从没有筛选条件开始。

import { useMemo, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useTranslation } from '@/i18n/react'
import type { CardView } from '@/lib/api/types'
import { describeError } from '@/lib/errors'
import { ActivityFilterBar } from './ActivityFilterBar'
import { isHiddenEvent } from './describeEvent'
import { ActivityItem } from './ActivityItem'
import { NO_FILTERS, countFilters, type ActivityFilters } from './filters'
import { useActivity } from './useActivity'

interface ActivityPanelProps {
  boardId: number
  cardsById: ReadonlyMap<number, CardView>
  /** 点了某张卡片的链接后调用 */
  onOpenCard: () => void
}

/** 活动抽屉的内容 */
export function ActivityPanel({ boardId, cardsById, onOpenCard }: ActivityPanelProps) {
  const { t } = useTranslation()
  const [filters, setFilters] = useState<ActivityFilters>(NO_FILTERS)
  const query = useActivity(boardId, filters)
  const events = useMemo(() => query.data?.pages.flatMap((page) => page.events).filter((event) => !isHiddenEvent(event)) ?? [], [query.data])
  const actors = query.data?.pages[0]?.actors ?? []
  const filtered = countFilters(filters) > 0

  return (
    <>
      <ActivityFilterBar filters={filters} onChange={setFilters} actors={actors} />
      <div className="min-h-0 flex-1 overflow-y-auto px-4 pb-4" aria-busy={query.isFetching}>
        {query.error && (
          <div role="alert" className="mt-3 flex flex-col gap-2 rounded-lg border border-destructive/40 p-3 text-sm">
            <p className="text-destructive">{describeError(query.error, t).message}</p>
            <Button type="button" variant="outline" size="sm" className="self-start" onClick={() => void query.refetch()}>
              {t('common.retry')}
            </Button>
          </div>
        )}
        {query.isPending && !query.error && (
          <div className="flex flex-col gap-3 pt-3" aria-label={t('common.loading')}>
            {[0, 1, 2, 3].map((i) => (
              <Skeleton key={i} className="h-12 w-full" />
            ))}
          </div>
        )}
        {!query.isPending && !query.error && events.length === 0 && (
          <div className="flex flex-col items-center gap-3 py-10 text-center text-sm text-muted-foreground">
            <p>{filtered ? t('activity.emptyFiltered') : t('activity.empty')}</p>
            {filtered && (
              <Button type="button" variant="outline" size="sm" onClick={() => setFilters(NO_FILTERS)}>
                {t('activity.clearFilters')}
              </Button>
            )}
          </div>
        )}
        {events.length > 0 && (
          <ol className={query.isPlaceholderData ? 'divide-y opacity-60 transition-opacity' : 'divide-y transition-opacity'}>
            {events.map((event) => (
              <ActivityItem key={event.id} event={event} cardsById={cardsById} onOpenCard={onOpenCard} />
            ))}
          </ol>
        )}
        {query.hasNextPage && (
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="mt-3 w-full"
            disabled={query.isFetchingNextPage}
            onClick={() => void query.fetchNextPage()}
          >
            {query.isFetchingNextPage ? t('common.loading') : t('activity.loadMore')}
          </Button>
        )}
      </div>
    </>
  )
}
