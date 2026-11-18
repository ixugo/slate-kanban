// 卡片详情弹窗：由 ?card= 控制开关，字段带 version 乐观并发。

import { useCallback } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import { useTranslation } from '@/i18n/react'
import { deleteCard, releaseCard } from '@/lib/api/endpoints'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import type { CardDetail, Column, Snapshot, UpdateCardInput } from '@/lib/api/types'
import { formatDateTime } from '@/lib/format'
import { useLanguage } from '@/i18n/useLanguage'
import { queryKeys } from '@/lib/queryKeys'
import { useErrorToast } from '@/lib/useErrorToast'
import { parseId } from '@/lib/route'
import { fromLocalInput } from '@/lib/format'
import { useMoveCard } from '@/features/board/useMoveCard'
import { useBoardSnapshot, useBoardView } from '@/features/board/useBoard'
import { useCardDetail } from './useCardDetail'
import { useCardUpdate } from './useCardUpdate'
import { CardFields } from './CardFields'
import { DescriptionEditor } from './DescriptionEditor'
import { Comments } from './Comments'

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
    const body = { ...input }
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
      <DialogContent className="flex max-h-[90dvh] w-[calc(100%-1rem)] max-w-2xl flex-col gap-4 overflow-y-auto sm:w-full">
        <DialogHeader>
          <DialogTitle className="font-mono text-sm">{card?.key ?? t('card.title')}</DialogTitle>
          <DialogDescription className="sr-only">{t('card.title')}</DialogDescription>
        </DialogHeader>
        {isPending && <Skeleton className="h-40 w-full" />}
        {showNotFound && <p role="alert">{t('card.notFound')}</p>}
        {card && (
          <FunctionCardBody
            card={card}
            columns={columns}
            depOptions={depOptions}
            lang={lang}
            t={t}
            onPatch={onPatch}
            onColumn={onColumn}
            onDesc={(description) => onPatch({ description, version: card.version })}
            patchBusy={patch.isPending}
            claimActive={card.claim_active}
            onRelease={() => release.mutate()}
            releaseBusy={release.isPending}
            onDelete={() => remove.mutate()}
            deleteBusy={remove.isPending}
            boardId={boardId}
          />
        )}
      </DialogContent>
    </Dialog>
  )
}

/** 合并依赖候选：详情里的 dependencies 加上快照里其余卡片 */
function useDepOptions(snapshot: Snapshot | undefined, card: CardDetail | undefined): CardDetail['dependencies'] {
  if (!snapshot || !card) return []
  const map = new Map(card.dependencies.map((d) => [d.id, d]))
  for (const c of snapshot.cards) {
    if (c.id === card.id) continue
    if (!map.has(c.id)) map.set(c.id, { id: c.id, key: c.key, title: c.title, column: c.column, done: c.done })
  }
  return [...map.values()].sort((a, b) => a.key.localeCompare(b.key))
}

interface BodyProps {
  card: CardDetail
  columns: Column[]
  depOptions: CardDetail['dependencies']
  lang: string
  t: ReturnType<typeof useTranslation>['t']
  onPatch: (input: UpdateCardInput) => void
  onColumn: (columnId: number) => void
  onDesc: (description: string) => void
  patchBusy: boolean
  claimActive: boolean
  onRelease: () => void
  releaseBusy: boolean
  onDelete: () => void
  deleteBusy: boolean
  boardId: number
}

function FunctionCardBody({
  card,
  columns,
  depOptions,
  lang,
  t,
  onPatch,
  onColumn,
  onDesc,
  patchBusy,
  claimActive,
  onRelease,
  releaseBusy,
  onDelete,
  deleteBusy,
  boardId,
}: BodyProps) {
  return (
    <>
      {claimActive && card.claimed_by && (
        <div className="flex flex-wrap items-center gap-2 rounded-md border border-signal/30 bg-signal/10 px-3 py-2 text-sm">
          <span>{t('card.claim.banner', { name: card.claimed_by, time: formatDateTime(card.claim_expires_at, lang) })}</span>
          <Button type="button" size="sm" variant="secondary" disabled={releaseBusy} onClick={onRelease}>
            {t('card.claim.release')}
          </Button>
        </div>
      )}
      <CardFields card={card} columns={columns} depOptions={depOptions} version={card.version} onPatch={(p) => onPatch(p as UpdateCardInput)} onColumn={onColumn} />
      {card.column_instruction && (
        <details className="rounded-md border p-2 text-xs">
          <summary className="cursor-pointer font-medium">{t('card.columnInstruction')}</summary>
          <p className="mt-2 text-muted-foreground">{card.column_instruction}</p>
        </details>
      )}
      <DescriptionEditor key={card.version} value={card.description} onSave={onDesc} busy={patchBusy} />
      <Comments cardId={card.id} boardId={boardId} comments={card.comments} />
      <p className="text-xs text-muted-foreground">
        {t('card.meta', {
          created: formatDateTime(card.created_at, lang),
          createdBy: card.created_by,
          updated: formatDateTime(card.updated_at, lang),
          updatedBy: card.updated_by,
        })}
      </p>
      <AlertDialog>
        <AlertDialogTrigger asChild>
          <Button type="button" variant="destructive" size="sm">
            {t('common.delete')}
          </Button>
        </AlertDialogTrigger>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('card.deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>{t('card.deleteBody')}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction disabled={deleteBusy} onClick={onDelete}>
              {t('common.delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}