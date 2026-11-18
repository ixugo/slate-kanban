// 卡片详情弹窗：由 ?card= 控制开关，宽屏两栏布局。

import { useCallback } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import { useTranslation } from '@/i18n/react'
import { deleteCard, releaseCard } from '@/lib/api/endpoints'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import type { CardDetail, Snapshot, UpdateCardInput } from '@/lib/api/types'
import { queryKeys } from '@/lib/queryKeys'
import { useErrorToast } from '@/lib/useErrorToast'
import { parseId } from '@/lib/route'
import { fromLocalInput } from '@/lib/format'
import { useMoveCard } from '@/features/board/useMoveCard'
import { useBoardSnapshot, useBoardView } from '@/features/board/useBoard'
import { useCardDetail } from './useCardDetail'
import { useCardUpdate } from './useCardUpdate'
import { useLanguage } from '@/i18n/useLanguage'
import { CardDialogBody } from './CardDialogBody'

interface CardDialogProps {
  boardId: number
}

/** 根据 URL 参数打开卡片弹窗 */
export default function CardDialog({ boardId }: CardDialogProps) {
  const { t } = useTranslation()
  const { lang } = useLanguage()
  const [params, setParams] = useSearchParams()
  const cardParam = params.get('card')
  const cardId = parseId(cardParam)
  const open = cardId !== null
  const { data: card, error, isPending } = useCardDetail(cardParam)
  const { data: snapshot } = useBoardSnapshot(boardId)
  const { columns } = useBoardView(snapshot)
  const patch = useCardUpdate()
  const move = useMoveCard(boardId)
  const queryClient = useQueryClient()
  const toastError = useErrorToast()

  const close = useCallback(() => {
    setParams((prev) => {
      const next = new URLSearchParams(prev)
      next.delete('card')
      return next
    })
  }, [setParams])

  const depOptions = useDepOptions(snapshot, card)

  const onPatch = (input: UpdateCardInput) => {
    if (!cardId || !card) return
    const body = { ...input, version: input.version ?? card.version }
    if (typeof body.due_at === 'string' && body.due_at !== '') {
      const iso = fromLocalInput(body.due_at)
      if (iso === null) return
      body.due_at = iso
    }
    patch.mutate({ cardId, boardId, input: body })
  }

  const onColumn = (columnId: number) => {
    if (!card || columnId === card.column_id) return
    move.mutate({ cardId: card.id, plan: { columnId } })
  }

  const release = useMutation({
    mutationFn: () => releaseCard(card!.id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.card(card!.id) })
      void queryClient.invalidateQueries({ queryKey: queryKeys.board(boardId) })
    },
    onError: toastError,
  })

  const remove = useMutation({
    mutationFn: () => deleteCard(card!.id),
    onSuccess: () => {
      close()
      void queryClient.invalidateQueries({ queryKey: queryKeys.board(boardId) })
    },
    onError: toastError,
  })

  const showNotFound = open && !isPending && (error !== null || !card)

  return (
    <Dialog open={open} onOpenChange={(v) => !v && close()}>
      <DialogContent className="flex max-h-[90dvh] w-[calc(100%-1rem)] flex-col gap-0 overflow-hidden p-0 sm:w-full sm:max-w-4xl">
        <DialogHeader className="shrink-0 border-b px-4 py-3">
          <DialogTitle className="font-mono text-sm">{card?.key ?? t('card.title')}</DialogTitle>
          <DialogDescription className="sr-only">{t('card.title')}</DialogDescription>
        </DialogHeader>
        <div className="min-h-0 flex-1 overflow-y-auto px-4 py-4">
          {isPending && <Skeleton className="h-40 w-full" />}
          {showNotFound && <p role="alert">{t('card.notFound')}</p>}
          {card && (
            <CardDialogBody
              card={card}
              columns={columns}
              depOptions={depOptions}
              lang={lang}
              boardId={boardId}
              onPatch={onPatch}
              onColumn={onColumn}
              onDesc={(description) => onPatch({ description, version: card.version })}
              patchBusy={patch.isPending}
              onRelease={() => release.mutate()}
              releaseBusy={release.isPending}
              onDelete={() => remove.mutate()}
              deleteBusy={remove.isPending}
            />
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}

/** 合并依赖候选：详情里的 dependencies 加上快照里其余卡片 */
function useDepOptions(snapshot: Snapshot | undefined, card: CardDetail | undefined): CardDetail['dependencies'] {
  if (!snapshot || !card) return []
  const map = new Map((card.dependencies ?? []).map((d) => [d.id, d]))
  for (const c of snapshot.cards) {
    if (c.id === card.id) continue
    if (!map.has(c.id)) map.set(c.id, { id: c.id, key: c.key, title: c.title, column: c.column, done: c.done })
  }
  return [...map.values()].sort((a, b) => a.key.localeCompare(b.key))
}