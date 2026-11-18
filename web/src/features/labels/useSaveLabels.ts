// 整体保存看板的标签库；改名和删除服务端会同步到所有卡片，所以保存后要刷新快照和卡片详情。

import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateBoard } from '@/lib/api/endpoints'
import type { LabelInput } from '@/lib/api/types'
import { queryKeys } from '@/lib/queryKeys'
import { useErrorToast } from '@/lib/useErrorToast'

/** 保存标签库；失败时弹出服务端给的原因 */
export function useSaveLabels(boardId: number) {
  const queryClient = useQueryClient()
  const toastError = useErrorToast()
  return useMutation({
    mutationFn: (labels: LabelInput[]) => updateBoard(boardId, { labels }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.board(boardId) })
      void queryClient.invalidateQueries({ queryKey: queryKeys.card() })
    },
    onError: toastError,
  })
}
