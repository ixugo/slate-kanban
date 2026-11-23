// 看板页：工具栏、搜索与筛选、列画布、任务弹窗与实时同步。

import { lazy, Suspense, useCallback, useMemo, useRef, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { useTranslation } from '@/i18n/react'
import { ApiError } from '@/lib/api/client'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { describeError } from '@/lib/errors'
import { parseId } from '@/lib/route'
import { useDocumentTitle } from '@/lib/useDocumentTitle'
import { useBoardSnapshot, useBoardView } from './useBoard'
import { useBoardLive } from './useBoardLive'
import { useMoveCard } from './useMoveCard'
import { useBoardSearch } from './useBoardSearch'
import { useBoardFilters } from './useBoardFilters'
import { useCreateCard } from './useCreateCard'
import { useBoardKeyboard } from './useBoardKeyboard'
import { BoardToolbar } from './BoardToolbar'
import { ActiveFilters } from './ActiveFilters'
import { buildFilterTerms, collectClaimers, collectTagOptions, countFilters } from './filters'
import { BoardCanvas } from './BoardCanvas'
import { BoardGone } from './BoardGone'
import { TooltipProvider } from '@/components/ui/tooltip'
import { useBoardViewMode } from './useBoardView'
import { LabelsProvider } from '@/features/labels/LabelsContext'
import type { Tag } from '@/lib/api/types'

/** 快照里没有标签库时用的空库，保持引用不变，避免上下文每次渲染都变 */
const NO_TAGS: Tag[] = []

const CardDialog = lazy(() => import('@/features/card/CardDialog'))
const ActivityDrawer = lazy(() => import('@/features/activity/ActivityDrawer'))
const ArchiveDrawer = lazy(() => import('@/features/card/ArchiveDrawer'))
const ListView = lazy(() => import('./ListView'))

/** 看板主页面 */
export default function BoardPage() {
  const { t } = useTranslation()
  const { boardId: rawId } = useParams()
  const boardId = parseId(rawId)
  const searchRef = useRef<HTMLInputElement>(null)
  const [gone, setGone] = useState(false)
  const [addColumnId, setAddColumnId] = useState<number | null>(null)
  const [listAddOpen, setListAddOpen] = useState(false)
  // 活动抽屉：第一次打开时才加载代码（loaded），之后开关只改 open，关闭动画才播得完
  const [archive, setArchive] = useState({ open: false, loaded: false })
  const [activity, setActivity] = useState({ open: false, loaded: false })
  const { mode: viewMode, setMode: setViewMode } = useBoardViewMode(boardId ?? 0)

  const { data: snapshot, error, isPending, refetch } = useBoardSnapshot(boardId)
  const { columns, cardsById } = useBoardView(snapshot)
  const { filters, setFilters, clear: clearFilters } = useBoardFilters(boardId)
  const filterTerms = useMemo(() => buildFilterTerms(filters), [filters])
  const filterCount = countFilters(filters)
  const tagOptions = useMemo(() => collectTagOptions(snapshot?.cards ?? []), [snapshot])
  const claimers = useMemo(() => collectClaimers(snapshot?.cards ?? []), [snapshot])
  const search = useBoardSearch(boardId, filterTerms)
  const move = useMoveCard(boardId ?? 0)
  const create = useCreateCard(boardId ?? 0)

  const onGone = useCallback(() => setGone(true), [])

  const live = useBoardLive({ boardId, onGone })

  useDocumentTitle(snapshot?.name)

  const searching = search.visibleIds !== null

  const focusFirstAdd = useCallback(() => {
    const first = columns[0]
    if (first) setAddColumnId(first.id)
  }, [columns])

  const clearSearchAndAdd = useCallback(() => {
    search.setQuery('')
    clearFilters()
    setAddColumnId(null)
  }, [search, clearFilters])

  const openNewCard = useCallback(() => {
    if (viewMode === 'list') setListAddOpen(true)
    else focusFirstAdd()
  }, [viewMode, focusFirstAdd])

  useBoardKeyboard({
    searchRef,
    onNewCard: openNewCard,
    onFocusSearch: () => searchRef.current?.focus(),
    onEscape: clearSearchAndAdd,
    disabled: boardId === null || gone,
  })

  if (boardId === null) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center gap-3 p-8 text-center">
        <p>{t('notFound.body')}</p>
        <Button asChild>
          <Link to="/">{t('notFound.home')}</Link>
        </Button>
      </div>
    )
  }

  if (gone) return <BoardGone />

  if (isPending) {
    return (
      <div className="flex flex-1 flex-col gap-2 p-4">
        <Skeleton className="h-10 w-full" />
        <Skeleton className="min-h-64 flex-1" />
      </div>
    )
  }

  if (error) {
    if (error instanceof ApiError && error.status === 404) return <BoardGone />
    const { message } = describeError(error, t)
    return (
      <div role="alert" className="m-4 flex flex-col gap-2 border border-destructive/40 p-4">
        <p className="text-destructive">{message}</p>
        <Button variant="outline" onClick={() => void refetch()}>
          {t('common.retry')}
        </Button>
      </div>
    )
  }

  if (!snapshot) return null

  if (columns.length === 0) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center gap-3 p-8 text-center">
        <h2 className="font-semibold">{t('board.noColumns.title')}</h2>
        <p className="text-muted-foreground">{t('board.noColumns.body')}</p>
        <Button asChild>
          <Link to={`/b/${boardId}/settings`}>{t('board.noColumns.cta')}</Link>
        </Button>
      </div>
    )
  }

  return (
    <TooltipProvider delayDuration={300}>
      <LabelsProvider boardId={boardId} tags={snapshot.tags ?? NO_TAGS}>
      <div className="flex min-h-0 flex-1 flex-col">
        <BoardToolbar
          board={snapshot}
          searchRef={searchRef}
          query={search.query}
          onQueryChange={search.setQuery}
          noResults={(search.visibleIds?.size ?? snapshot.cards.length) === 0}
          truncated={search.truncated}
          syntaxError={search.syntaxError}
          searching={search.searching}
          searchActive={search.query.trim().length > 0 || filterCount > 0}
          filter={{ filters, onChange: setFilters, columns, tags: tagOptions, claimers }}
          liveState={live.state}
          onLiveRetry={live.retry}
          viewMode={viewMode}
          onViewModeChange={setViewMode}
          onActivityOpen={() => setActivity({ open: true, loaded: true })}
          onArchiveOpen={() => setArchive({ open: true, loaded: true })}
        />
        <ActiveFilters filters={filters} columns={columns} onChange={setFilters} />
        {viewMode === 'board' && searching && (
          <p className="border-b bg-muted/40 px-3 py-1 text-center text-[11px] text-muted-foreground">{t('board.dragDisabledSearch')}</p>
        )}
        {viewMode === 'board' ? (
          <BoardCanvas
            boardId={boardId}
            columns={columns}
            cardsById={cardsById}
            visibleIds={search.visibleIds}
            highlights={search.highlights}
            dragDisabled={searching}
            onMove={(cardId, plan) => move.mutate({ cardId, plan })}
            addColumnId={addColumnId}
            onAddOpen={setAddColumnId}
            onAddClose={() => setAddColumnId(null)}
            onAddCard={(columnId, title) => create.mutateAsync({ columnId, title })}
            addBusy={create.isPending}
          />
        ) : (
          <Suspense fallback={null}>
            <ListView
              columns={columns}
              cards={snapshot.cards}
              visibleIds={search.visibleIds}
              highlights={search.highlights}
              listAddOpen={listAddOpen}
              onListAddOpen={() => setListAddOpen(true)}
              onListAddClose={() => setListAddOpen(false)}
              onAddCard={(title) => {
                const first = columns[0]
                if (first) create.mutate({ columnId: first.id, title })
              }}
              addBusy={create.isPending}
            />
          </Suspense>
        )}
        <Suspense fallback={null}>
          <CardDialog boardId={boardId} />
        </Suspense>
        {archive.loaded && (
          <Suspense fallback={null}>
            <ArchiveDrawer boardId={boardId} open={archive.open} onOpenChange={(open) => setArchive({ open, loaded: true })} />
          </Suspense>
        )}
        {activity.loaded && (
          <Suspense fallback={null}>
            <ActivityDrawer
              boardId={boardId}
              open={activity.open}
              onOpenChange={(open) => setActivity((prev) => ({ ...prev, open }))}
              cardsById={cardsById}
            />
          </Suspense>
        )}
      </div>
      </LabelsProvider>
    </TooltipProvider>
  )
}
