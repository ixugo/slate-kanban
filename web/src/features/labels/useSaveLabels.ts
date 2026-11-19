// 整体保存看板的标签库；删除标签时服务端会从所有卡片上摘掉它，所以保存后要刷新快照和卡片详情。

import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateBoard } from '@/lib/api/endpoints'
import type { TagInput } from '@/lib/api/types'
import { queryKeys } from '@/lib/queryKeys'
import { useErrorToast } from '@/lib/useErrorToast'

/** 保存标签库；成功时返回带新主键的看板，失败时弹出服务端给的原因 */
export function useSaveLabels(boardId: number) {
  const queryClient = useQueryClient()
  const toastError = useErrorToast()
  return useMutation({
    mutationFn: (tags: TagInput[]) => updateBoard(boardId, { tags }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.board(boardId) })
      void queryClient.invalidateQueries({ queryKey: queryKeys.card() })
    },
    onError: toastError,
  })
}
