// 修改卡片字段：带 version；冲突时提示并刷新详情。

import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateCard } from '@/lib/api/endpoints'
import type { UpdateCardInput } from '@/lib/api/types'
import { isVersionConflict } from '@/lib/errors'
import { queryKeys } from '@/lib/queryKeys'
import { useErrorToast } from '@/lib/useErrorToast'
import { useTranslation } from '@/i18n/react'
import { toast } from '@/lib/toast'

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
    onSuccess: (_card, { cardId, boardId }) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.board(boardId) })
      void queryClient.invalidateQueries({ queryKey: queryKeys.card(cardId) })
    },
    onError: (err, { cardId, boardId }) => {
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