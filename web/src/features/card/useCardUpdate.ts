// 修改卡片字段：带 version。先把预测结果写进详情和看板快照，请求失败再还原；冲突时提示并刷新详情。

import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateCard } from '@/lib/api/endpoints'
import type { CardDetail, Snapshot, UpdateCardInput } from '@/lib/api/types'
import { isVersionConflict } from '@/lib/errors'
import { applyOptimistic, rollbackOptimistic } from '@/lib/optimistic'
import { queryKeys } from '@/lib/queryKeys'
import { useErrorToast } from '@/lib/useErrorToast'
import { useTranslation } from '@/i18n/react'
import { toast } from '@/lib/toast'
import { applyCardPatch } from './cardPatch'

interface PatchArgs {
  cardId: number
  boardId: number
  input: UpdateCardInput
}

/** 返回 PATCH 卡片的变更 */
export function useCardUpdate() {
  const queryClient = useQueryClient()
  const toastError = useErrorToast()
  const { t } = useTranslation()

  return useMutation({
    mutationFn: ({ cardId, input }: PatchArgs) => updateCard(cardId, input),
    // 先改详情和看板磁贴，界面立刻是改完的样子；记下旧值，失败时还原
    onMutate: async ({ cardId, boardId, input }) => {
      const library = queryClient.getQueryData<Snapshot>(queryKeys.board(boardId))?.tags ?? []
      const detail = await applyOptimistic<CardDetail>(queryClient, queryKeys.card(cardId), (old) =>
        applyCardPatch(old, input, library),
      )
      const snapshot = await applyOptimistic<Snapshot>(queryClient, queryKeys.board(boardId), (old) => ({
        ...old,
        cards: old.cards.map((c) => (c.id === cardId ? applyCardPatch(c, input, library) : c)),
      }))
      return { detail, snapshot }
    },
    onSuccess: (updated, { cardId, boardId }) => {
      queryClient.setQueryData<CardDetail>(queryKeys.card(cardId), (prev) => {
        if (!prev) return prev
        return {
          ...prev,
          ...updated,
          dependencies: prev.dependencies,
          comments: prev.comments,
          column_instruction: prev.column_instruction,
        }
      })
      void queryClient.invalidateQueries({ queryKey: queryKeys.board(boardId) })
      void queryClient.invalidateQueries({ queryKey: queryKeys.card(cardId) })
    },
    onError: (err, { cardId, boardId }, context) => {
      rollbackOptimistic(queryClient, queryKeys.card(cardId), context?.detail)
      rollbackOptimistic(queryClient, queryKeys.board(boardId), context?.snapshot)
      if (isVersionConflict(err)) {
        toast.error(t('card.versionConflict'))
        void queryClient.invalidateQueries({ queryKey: queryKeys.card(cardId) })
        void queryClient.invalidateQueries({ queryKey: queryKeys.board(boardId) })
        return
      }
      toastError(err)
    },
  })
}