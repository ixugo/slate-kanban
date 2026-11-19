// 卡片详情弹窗：由 ?card= 控制开关，宽屏两栏布局。

import { useCallback } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import { useTranslation } from '@/i18n/react'
import { ApiError } from '@/lib/api/client'
import { deleteCard, releaseCard } from '@/lib/api/endpoints'
import { describeError } from '@/lib/errors'
import { Dialog, DialogContent } from '@/components/ui/dialog'
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
import { CardHeaderBar } from './CardHeaderBar'

interface CardDialogProps {
  boardId: number
}

/**
 * 描述编辑区里按 Esc 是“放弃这次修改”，不该顺带关掉整个弹窗；
 * Radix 在文档层先收到按键，所以要在这里拦住，编辑器自己的 Esc 处理照常执行。
 */
function keepDialogWhenEditorCancels(e: KeyboardEvent) {
  if ((e.target as HTMLElement | null)?.closest?.('[data-escape-cancel]')) e.preventDefault()
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

  // 只有服务端明确说卡片不存在（404）才提示"已经没有这张卡片"；
  // 断网、服务器出错时如实说明原因，不能让人误以为卡片被删了。
  const gone = error instanceof ApiError && error.status === 404
  const loaded = open && !isPending
  const showNotFound = loaded && (gone || (error === null && !card))
  const loadError = loaded && !card && error !== null && !gone ? describeError(error, t).message : ''

  return (
    <Dialog open={open} onOpenChange={(v) => !v && close()}>
      <DialogContent
        onEscapeKeyDown={keepDialogWhenEditorCancels}
        className="flex max-h-[90dvh] w-[calc(100%-1rem)] flex-col gap-0 overflow-hidden bg-popover! p-0 backdrop-blur-none! sm:w-full sm:max-w-4xl"
      >
        <CardHeaderBar card={card} columns={columns} onColumn={onColumn} onDelete={() => remove.mutate()} deleteBusy={remove.isPending} />
        <div className="flex min-h-0 flex-1 flex-col overflow-y-auto md:overflow-hidden">
          {isPending && <Skeleton className="m-6 h-40 rounded-2xl" />}
          {showNotFound && <p role="alert" className="p-6">{t('card.notFound')}</p>}
          {loadError !== '' && <p role="alert" className="p-6">{loadError}</p>}
          {card && (
            <CardDialogBody
              card={card}
              depOptions={depOptions}
              lang={lang}
              boardId={boardId}
              onPatch={onPatch}
              onDesc={(description) => onPatch({ description, version: card.version })}
              patchBusy={patch.isPending}
              onRelease={() => release.mutate()}
              releaseBusy={release.isPending}
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