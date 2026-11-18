// 在指定列新建卡片：标题必填，成功后让快照失效以便实时流或查询刷新列表。

import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createCard } from '@/lib/api/endpoints'
import { queryKeys } from '@/lib/queryKeys'
import { useErrorToast } from '@/lib/useErrorToast'
import { CARD_TITLE_MAX } from '@/lib/limits'

/** 返回在某列创建卡片的变更 */
export function useCreateCard(boardId: number) {
  const queryClient = useQueryClient()
  const toastError = useErrorToast()

  return useMutation({
    mutationFn: (input: { columnId: number; title: string }) =>
      createCard(boardId, {
        column_id: input.columnId,
        title: input.title.trim().slice(0, CARD_TITLE_MAX),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.board(boardId) })
    },
    onError: toastError,
  })
}