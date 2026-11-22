// 任务详情弹窗：由 ?card= 打开根任务，描述和评论的引用可在同一个弹窗里逐层打开。

import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import { useTranslation } from '@/i18n/react'
import { ApiError } from '@/lib/api/client'
import { archiveCard, deleteCard, forkCard, releaseCard } from '@/lib/api/endpoints'
import { describeError } from '@/lib/errors'
import { Dialog, DialogContent } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import type { Card, CardDetail, Snapshot, UpdateCardInput } from '@/lib/api/types'
import { applyOptimistic, rollbackOptimistic } from '@/lib/optimistic'
import { queryKeys } from '@/lib/queryKeys'
import { useErrorToast } from '@/lib/useErrorToast'
import { toast } from '@/lib/toast'
import { parseId } from '@/lib/route'
import { fromLocalInput } from '@/lib/format'
import { useMoveCard } from '@/features/board/useMoveCard'
import { useBoardSnapshot, useBoardView } from '@/features/board/useBoard'
import { useCardDetail } from './useCardDetail'
import { useMarkCardRead } from './useMarkCardRead'
import { useCardUpdate } from './useCardUpdate'
import { clearClaim } from './cardPatch'
import { useLanguage } from '@/i18n/useLanguage'
import { CardDialogBody } from './CardDialogBody'
import { CardHeaderBar } from './CardHeaderBar'
import { CardNumberReference } from './CardNumberReference'

interface CardDialogProps {
  boardId: number
}

/**
 * 描述编辑区和更多操作各有自己的 Escape 行为。
 * Radix 先收到键盘事件，在这里保留详情，让内层继续处理。
 */
function keepDialogWhenInnerEscapes(e: KeyboardEvent) {
  if ((e.target as HTMLElement | null)?.closest?.('[data-escape-cancel], [data-card-actions-expanded="true"]')) e.preventDefault()
}

/** URL 只记录用户打开的根任务，切换根任务时清空引用层。 */
export default function CardDialog({ boardId }: CardDialogProps) {
  const [params, setParams] = useSearchParams()
  const rootId = parseId(params.get('card'))
  const closeAll = useCallback(() => {
    setParams((prev) => {
      const next = new URLSearchParams(prev)
      next.delete('card')
      return next
    })
  }, [setParams])

  const openFork = (id: number) => {
    setParams((prev) => {
      const next = new URLSearchParams(prev)
      next.set('card', String(id))
      return next
    })
  }

  if (rootId === null) return null
  return <CardDialogStack key={`${boardId}:${rootId}`} boardId={boardId} rootId={rootId} closeAll={closeAll} openFork={openFork} />
}

function CardDialogStack({ boardId, rootId, closeAll, openFork }: CardDialogProps & { rootId: number; closeAll: () => void; openFork: (id: number) => void }) {
  const { t } = useTranslation()
  const { lang } = useLanguage()
  const [references, setReferences] = useState<number[]>([])
  const cardId = references.at(-1) ?? rootId
  const { data: card, error, isPending } = useCardDetail(cardId)
  const { data: snapshot } = useBoardSnapshot(boardId)
  const cardsByNumber = useMemo(
    () => new Map(snapshot?.cards.map((candidate) => [candidate.number, candidate])),
    [snapshot?.cards],
  )
  const { columns } = useBoardView(snapshot)
  const patch = useCardUpdate()
  const move = useMoveCard(boardId)
  const queryClient = useQueryClient()
  const toastError = useErrorToast()
  const { tryMark } = useMarkCardRead(boardId)

  useEffect(() => {
    tryMark(cardId)
  }, [cardId, tryMark])

  const content = useRef<HTMLDivElement>(null)
  const layer = useRef({ cardId, depth: references.length })
  useLayoutEffect(() => {
    layer.current = { cardId, depth: references.length }
    content.current?.focus({ preventScroll: true })
  }, [cardId, references.length])

  const closeCard = (id: number) => {
    if (layer.current.cardId !== id) return
    if (layer.current.depth === 0) closeAll()
    else setReferences((previous) => previous.at(-1) === id ? previous.slice(0, -1) : previous)
  }

  const openReference = (target: Card | undefined) => {
    if (!target) {
      toast.error(t('card.notFound'))
      return
    }
    setReferences((previous) => (previous.at(-1) ?? rootId) === target.id ? previous : [...previous, target.id])
  }

  const renderCardReference = (number: number, reference: string) => {
    const target = cardsByNumber.get(number)
    const isShortReference = reference.startsWith('#')
    if (!isShortReference && target?.key !== reference) return reference
    const label = isShortReference ? reference : '#' + number
    return <CardNumberReference boardId={boardId} number={number} label={label} card={target} onOpen={openReference} />
  }

  const depOptions = useDepOptions(snapshot, card)

  const onPatch = (input: UpdateCardInput) => {
    if (!card || card.archived_at) return
    const body = { ...input, version: input.version ?? card.version }
    // 日期弹窗给出的是本地时间，转成 RFC3339 再提交；空串表示清除，原样提交
    for (const key of ['due_at', 'started_at'] as const) {
      const value = body[key]
      if (typeof value !== 'string' || value === '') continue
      const iso = fromLocalInput(value)
      if (iso === null) return
      body[key] = iso
    }
    patch.mutate({ cardId, boardId, input: body })
  }

  const onColumn = (columnId: number) => {
    if (!card || card.archived_at || columnId === card.column_id) return
    move.mutate({ cardId: card.id, plan: { columnId } })
  }

  // 放弃领取：详情和看板磁贴先显示成未被领取，失败再还原
  const release = useMutation({
    mutationFn: releaseCard,
    onMutate: async (id) => ({
      detail: await applyOptimistic<CardDetail>(queryClient, queryKeys.card(id), (old) => clearClaim(old)),
      snapshot: await applyOptimistic<Snapshot>(queryClient, queryKeys.board(boardId), (old) => ({
        ...old,
        cards: old.cards.map((c) => (c.id === id ? clearClaim(c) : c)),
      })),
    }),
    onError: (err, id, context) => {
      rollbackOptimistic(queryClient, queryKeys.card(id), context?.detail)
      rollbackOptimistic(queryClient, queryKeys.board(boardId), context?.snapshot)
      toastError(err)
    },
    onSettled: (_data, _err, id) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.card(id) })
      void queryClient.invalidateQueries({ queryKey: queryKeys.board(boardId) })
    },
  })

  const archive = useMutation({
    mutationFn: archiveCard,
    onSuccess: (updated) => {
      queryClient.setQueryData<CardDetail>(queryKeys.card(updated.id), (old) => old && {
        ...old,
        ...updated,
        claim_active: false,
      })
      void queryClient.invalidateQueries({ queryKey: queryKeys.card(updated.id) })
      void queryClient.invalidateQueries({ queryKey: queryKeys.board(boardId) })
    },
    onError: toastError,
  })

  const fork = useMutation({
    mutationFn: forkCard,
    onSuccess: (created) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.board(boardId) })
      openFork(created.id)
    },
    onError: toastError,
  })

  // 删除：弹窗先关、磁贴先消失，失败再把磁贴放回来（弹窗不再弹出，只提示原因）
  const remove = useMutation({
    mutationFn: deleteCard,
    onMutate: async (id) => {
      closeCard(id)
      return {
        snapshot: await applyOptimistic<Snapshot>(queryClient, queryKeys.board(boardId), (old) => ({
          ...old,
          cards: old.cards.filter((c) => c.id !== id),
        })),
      }
    },
    onError: (err, _id, context) => {
      rollbackOptimistic(queryClient, queryKeys.board(boardId), context?.snapshot)
      toastError(err)
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.board(boardId) })
    },
  })

  // 只有服务端明确说任务不存在（404）才提示"已经没有这个任务"；
  // 断网、服务器出错时如实说明原因，不能让人误以为任务被删了。
  const gone = error instanceof ApiError && error.status === 404
  const loaded = !isPending
  const showNotFound = loaded && (gone || (error === null && !card))
  const loadError = loaded && !card && error !== null && !gone ? describeError(error, t).message : ''

  return (
    <Dialog open onOpenChange={(open) => !open && closeCard(cardId)}>
      <DialogContent
        ref={content}
        tabIndex={-1}
        showCloseButton={false}
        onEscapeKeyDown={keepDialogWhenInnerEscapes}
        onPointerDownOutside={(event) => {
          const { target, button, ctrlKey } = event.detail.originalEvent
          if (!(target instanceof Element) || !target.matches('[data-slot="dialog-overlay"]')) return
          event.preventDefault()
          if (button === 0 && !ctrlKey) closeAll()
        }}
        className="flex h-[90dvh] w-[calc(100%-1rem)] flex-col gap-0 overflow-hidden bg-popover! p-0 backdrop-blur-none! sm:w-full sm:max-w-4xl"
      >
        <CardHeaderBar
          card={card}
          columns={columns}
          onColumn={onColumn}
          onDelete={() => remove.mutate(cardId)}
          onArchive={() => archive.mutate(cardId)}
          onFork={() => fork.mutate(cardId)}
          actionBusy={archive.isPending || fork.isPending || remove.isPending}
        />
        <div className="flex min-h-0 flex-1 flex-col overflow-y-auto md:overflow-hidden">
          {isPending && <Skeleton className="m-6 h-40 rounded-2xl" />}
          {showNotFound && <p role="alert" className="p-6">{t('card.notFound')}</p>}
          {loadError !== '' && <p role="alert" className="p-6">{loadError}</p>}
          {card && (
            <CardDialogBody
              key={cardId}
              card={card}
              depOptions={depOptions}
              lang={lang}
              boardId={boardId}
              onPatch={onPatch}
              onDesc={(description) => onPatch({ description, version: card.version })}
              renderCardReference={renderCardReference}
              patchBusy={patch.isPending && patch.variables?.cardId === cardId}
              onRelease={() => release.mutate(cardId)}
              releaseBusy={release.isPending && release.variables === cardId}
            />
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}

/** 合并依赖候选：详情里的 dependencies 加上快照里其余任务 */
function useDepOptions(snapshot: Snapshot | undefined, card: CardDetail | undefined): CardDetail['dependencies'] {
  if (!card) return []
  const map = new Map((card.dependencies ?? []).map((d) => [d.id, d]))
  for (const c of snapshot?.cards ?? []) {
    if (c.id === card.id) continue
    if (!map.has(c.id)) map.set(c.id, { id: c.id, key: c.key, title: c.title, column: c.column, done: c.done })
  }
  return [...map.values()]
}
