// 看板页：工具栏、搜索、列画布、卡片弹窗与实时同步。

import { lazy, Suspense, useCallback, useRef, useState } from 'react'
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
import { useCreateCard } from './useCreateCard'
import { useBoardKeyboard } from './useBoardKeyboard'
import { BoardToolbar } from './BoardToolbar'
import { BoardCanvas } from './BoardCanvas'
import { BoardGone } from './BoardGone'
import { TooltipProvider } from '@/components/ui/tooltip'

const CardDialog = lazy(() => import('@/features/card/CardDialog'))

/** 看板主页面 */
export default function BoardPage() {
  const { t } = useTranslation()
  const { boardId: rawId } = useParams()
  const boardId = parseId(rawId)
  const searchRef = useRef<HTMLInputElement>(null)
  const [gone, setGone] = useState(false)
  const [addColumnId, setAddColumnId] = useState<number | null>(null)

  const { data: snapshot, error, isPending, refetch } = useBoardSnapshot(boardId)
  const { columns, cardsById } = useBoardView(snapshot)
  const search = useBoardSearch(boardId)
  const move = useMoveCard(boardId ?? 0)
  const create = useCreateCard(boardId ?? 0)

  const onGone = useCallback(() => setGone(true), [])

  const live = useBoardLive({ boardId, onGone })

  useDocumentTitle(snapshot?.name)

  const dragDisabled = search.visibleIds !== null
  const totalCards = snapshot?.cards.length ?? 0
  const matched = search.visibleIds?.size ?? totalCards

  const focusFirstAdd = useCallback(() => {
    const first = columns[0]
    if (first) setAddColumnId(first.id)
  }, [columns])

  const clearSearchAndAdd = useCallback(() => {
    search.setQuery('')
    setAddColumnId(null)
  }, [search])

  useBoardKeyboard({
    searchRef,
    onNewCard: focusFirstAdd,
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
    <TooltipProvider>
    <div className="flex min-h-0 flex-1 flex-col">
      <BoardToolbar
        board={snapshot}
        searchRef={searchRef}
        query={search.query}
        onQueryChange={search.setQuery}
        matched={matched}
        total={search.total || totalCards}
        truncated={search.truncated}
        syntaxError={search.syntaxError}
        searching={search.searching}
        searchActive={search.query.trim().length > 0}
        liveState={live.state}
        onLiveRetry={live.retry}
      />
      {dragDisabled && (
        <p className="border-b bg-muted/40 px-3 py-1 text-center text-[11px] text-muted-foreground">{t('board.dragDisabledSearch')}</p>
      )}
      <BoardCanvas
        columns={columns}
        cardsById={cardsById}
        visibleIds={search.visibleIds}
        dragDisabled={dragDisabled}
        onMove={(cardId, plan) => move.mutate({ cardId, plan })}
        addColumnId={addColumnId}
        onAddOpen={setAddColumnId}
        onAddClose={() => setAddColumnId(null)}
        onAddCard={(columnId, title) => create.mutate({ columnId, title })}
        addBusy={create.isPending}
      />
      <Suspense fallback={null}>
        <CardDialog boardId={boardId} />
      </Suspense>
    </div>
    </TooltipProvider>
  )
}